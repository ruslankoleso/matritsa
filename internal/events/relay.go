package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// channel is the PostgreSQL notification channel shared by all instances of the API.
const channel = "matritsa_events"

type envelope struct {
	Key   string `json:"k"`
	Event Event  `json:"e"`
}

// Relay connects the hub to PostgreSQL LISTEN/NOTIFY: an event published on any instance of the API
// reaches the streams open on every instance, so they can run behind a load balancer.
// It returns once the first LISTEN is in place; reconnection after that runs in the background.
func Relay(ctx context.Context, pool *pgxpool.Pool, h *Hub) {
	ready := make(chan struct{})
	go listen(ctx, pool, h, ready)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		slog.Warn("event relay: LISTEN is not ready yet, events stay local until it is")
	case <-ctx.Done():
	}
}

func listen(ctx context.Context, pool *pgxpool.Pool, h *Hub, ready chan struct{}) {
	signalled := false
	for ctx.Err() == nil {
		err := listenOnce(ctx, pool, h, func() {
			h.SetSender(func(key string, e Event) error { return notify(pool, key, e) })
			if !signalled {
				signalled = true
				close(ready)
			}
		})
		if ctx.Err() != nil {
			return
		}
		// Without a listener this instance would miss its own events: deliver locally until it is back.
		h.SetSender(nil)
		slog.Warn("event relay: connection lost, retrying", "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func listenOnce(ctx context.Context, pool *pgxpool.Pool, h *Hub, onListening func()) error {
	pc, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// The connection keeps LISTEN state, so it never goes back to the pool: it is taken over and closed.
	conn := pc.Hijack()
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}
	onListening()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var env envelope
		if err := json.Unmarshal([]byte(n.Payload), &env); err != nil {
			slog.Warn("event relay: malformed notification", "err", err)
			continue
		}
		h.Deliver(env.Key, env.Event)
	}
}

func notify(pool *pgxpool.Pool, key string, e Event) error {
	b, err := json.Marshal(envelope{Key: key, Event: e})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = pool.Exec(ctx, "SELECT pg_notify($1, $2)", channel, string(b))
	return err
}

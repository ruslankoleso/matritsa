// Package events fans application events out to connected browsers (SSE).
package events

import (
	"sync"
	"time"
)

type Event struct {
	Type          string    `json:"type"` // status | notification | queue | chat
	ApplicationID string    `json:"applicationId,omitempty"`
	ThreadID      string    `json:"threadId,omitempty"`
	Status        string    `json:"status,omitempty"`
	Text          string    `json:"text,omitempty"`
	At            time.Time `json:"at"`
}

// Hub is a pub/sub keyed by user ID (or "staff" for the whole bank team) for the streams open on this
// instance. With a sender (see Relay) events go through PostgreSQL and come back to every instance.
// Delivery is best-effort and never blocks the publisher; the UI refetches on reconnect.
type Hub struct {
	mu     sync.Mutex
	subs   map[string]map[chan Event]struct{}
	closed bool
	send   func(key string, e Event) error
}

// SetSender routes Publish through an external bus; the bus hands events back through Deliver.
func (h *Hub) SetSender(send func(key string, e Event) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.send = send
}

func NewHub() *Hub { return &Hub{subs: map[string]map[chan Event]struct{}{}} }

// Subscribe returns a channel for key and an idempotent cancel func.
func (h *Hub) Subscribe(key string) (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		close(ch)
		return ch, func() {}
	}
	if h.subs[key] == nil {
		h.subs[key] = map[chan Event]struct{}{}
	}
	h.subs[key][ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if _, active := h.subs[key][ch]; !active {
				return // Close already released it
			}
			delete(h.subs[key], ch)
			if len(h.subs[key]) == 0 {
				delete(h.subs, key)
			}
			close(ch)
		})
	}
}

// Close ends every open stream and refuses new ones; it is called on shutdown,
// because the HTTP server waits for long-lived event streams to finish.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for key, set := range h.subs {
		for ch := range set {
			close(ch)
		}
		delete(h.subs, key)
	}
}

// Publish sends an event to the streams of key on every instance. If the bus is unavailable, at least
// the streams on this instance get it.
func (h *Hub) Publish(key string, e Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	h.mu.Lock()
	send := h.send
	h.mu.Unlock()
	if send != nil && send(key, e) == nil {
		return // it comes back through Deliver, here and on the other instances
	}
	h.Deliver(key, e)
}

// Deliver hands an event to the streams of key open on this instance.
func (h *Hub) Deliver(key string, e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[key] {
		select {
		case ch <- e:
		default: // slow consumer: drop
		}
	}
}

func (h *Hub) Subscribers(key string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[key])
}

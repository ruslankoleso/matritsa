// Package events fans application events out to connected browsers (SSE).
package events

import (
	"sync"
	"time"
)

type Event struct {
	Type          string    `json:"type"` // status | notification | queue
	ApplicationID string    `json:"applicationId,omitempty"`
	Status        string    `json:"status,omitempty"`
	Text          string    `json:"text,omitempty"`
	At            time.Time `json:"at"`
}

// Hub is an in-memory pub/sub keyed by user ID (or "staff" for the whole bank team).
// Delivery is best-effort and never blocks the publisher; the UI refetches on reconnect.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[string]map[chan Event]struct{}{}} }

// Subscribe returns a channel for key and an idempotent cancel func.
func (h *Hub) Subscribe(key string) (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	if h.subs[key] == nil {
		h.subs[key] = map[chan Event]struct{}{}
	}
	h.subs[key][ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs[key], ch)
			if len(h.subs[key]) == 0 {
				delete(h.subs, key)
			}
			h.mu.Unlock()
			close(ch)
		})
	}
}

func (h *Hub) Publish(key string, e Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
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

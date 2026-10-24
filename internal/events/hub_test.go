package events

import (
	"testing"
	"time"
)

func recv(t *testing.T, ch <-chan Event) (Event, bool) {
	t.Helper()
	select {
	case e := <-ch:
		return e, true
	case <-time.After(100 * time.Millisecond):
		return Event{}, false
	}
}

func TestHub_PublishReachesOnlyTarget(t *testing.T) {
	h := NewHub()
	a, cancelA := h.Subscribe("user-a")
	b, cancelB := h.Subscribe("user-b")
	defer cancelA()
	defer cancelB()
	h.Publish("user-a", Event{Type: "status", ApplicationID: "1", Status: "OFFER_MADE"})
	if e, ok := recv(t, a); !ok || e.Status != "OFFER_MADE" {
		t.Fatalf("a must receive: %+v %v", e, ok)
	}
	if _, ok := recv(t, b); ok {
		t.Fatal("b must not receive a's event")
	}
}

func TestHub_MultipleSubscribersSameKey(t *testing.T) {
	h := NewHub()
	c1, x1 := h.Subscribe("staff")
	c2, x2 := h.Subscribe("staff")
	defer x1()
	defer x2()
	h.Publish("staff", Event{Type: "queue"})
	_, ok1 := recv(t, c1)
	_, ok2 := recv(t, c2)
	if !ok1 || !ok2 {
		t.Fatal("both tabs of the same key must receive")
	}
}

func TestHub_Unsubscribe_ClosesAndStops(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("u")
	cancel()
	cancel() // idempotent
	if _, open := <-ch; open {
		t.Fatal("channel must be closed after cancel")
	}
	h.Publish("u", Event{}) // must not panic
	if h.Subscribers("u") != 0 {
		t.Fatal("subscriber must be removed")
	}
}

func TestHub_SlowSubscriberDoesNotBlockPublisher(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe("slow")
	defer cancel()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.Publish("slow", Event{Type: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked by a slow subscriber")
	}
}

func TestHub_CloseEndsEveryStreamAndIsSafeWithCancel(t *testing.T) {
	h := NewHub()
	a, cancelA := h.Subscribe("user-a")
	b, cancelB := h.Subscribe("staff")
	h.Close()
	for _, ch := range []<-chan Event{a, b} {
		select {
		case _, open := <-ch:
			if open {
				t.Fatal("channel must be closed, got an event")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Close must release every subscriber")
		}
	}
	cancelA() // cancel after Close must not panic on a double close
	cancelB()
	h.Publish("user-a", Event{Type: "status"}) // and nobody is listening any more

	late, cancelLate := h.Subscribe("user-c")
	defer cancelLate()
	if _, open := <-late; open {
		t.Fatal("a subscriber that arrives after Close must get a closed channel")
	}
}

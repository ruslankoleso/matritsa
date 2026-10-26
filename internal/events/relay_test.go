package events_test

import (
	"context"
	"testing"
	"time"

	"matritsa/internal/events"
	"matritsa/internal/store/storetest"
)

// Two hubs on one database stand for two instances of the API behind a load balancer.
func TestRelay_EventReachesStreamsOnAnotherInstance(t *testing.T) {
	s := storetest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := events.NewHub(), events.NewHub()
	events.Relay(ctx, s.Pool, a)
	events.Relay(ctx, s.Pool, b)

	onB, stopB := b.Subscribe("client:1")
	defer stopB()
	onA, stopA := a.Subscribe("client:1")
	defer stopA()

	a.Publish("client:1", events.Event{Type: "status", ApplicationID: "app-1", Status: "OFFER_MADE"})
	for name, ch := range map[string]<-chan events.Event{"other instance": onB, "same instance": onA} {
		select {
		case e := <-ch:
			if e.ApplicationID != "app-1" || e.Status != "OFFER_MADE" {
				t.Fatalf("%s got %+v", name, e)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s did not get the event", name)
		}
	}
	// Exactly once on the publishing instance: the relay replaces local delivery, it does not add to it.
	select {
	case e := <-onA:
		t.Fatalf("duplicate event on the publishing instance: %+v", e)
	case <-time.After(300 * time.Millisecond):
	}
}

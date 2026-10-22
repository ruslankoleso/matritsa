package workflow

import (
	"errors"
	"os"
	"testing"
)

func TestNext_ValidTransitions(t *testing.T) {
	cases := []struct {
		from Status
		ev   Event
		want Status
	}{
		{Draft, Submit, Submitted},
		{Submitted, StartIntake, IntakeCheck},
		{IntakeCheck, IntakePassed, Enrichment},
		{IntakeCheck, IntakeNeedsInfo, NeedsInfo},
		{NeedsInfo, ClientProvidedInfo, Enrichment},
		{NeedsInfo, ClientAnswered, AnalystReview},
		{Enrichment, EnrichmentDone, Scoring},
		{Enrichment, EnrichmentFailed, AnalystReview},
		{Scoring, AutoOffer, OfferMade},
		{Scoring, SendToReview, AnalystReview},
		{Scoring, AutoReject, Rejected},
		{AnalystReview, AnalystOffer, OfferMade},
		{AnalystReview, AnalystRequestInfo, NeedsInfo},
		{AnalystReview, AnalystReject, Rejected},
		{OfferMade, ClientAccept, Signing},
		{OfferMade, ClientDecline, DeclinedByClient},
		{OfferMade, OfferExpired, Expired},
		{Signing, Disburse, Disbursed},
		{Draft, Cancel, Cancelled},
		{OfferMade, Cancel, Cancelled},
	}
	for _, c := range cases {
		got, err := Next(c.from, c.ev)
		if err != nil || got != c.want {
			t.Errorf("%s + %s = %s, %v; want %s", c.from, c.ev, got, err, c.want)
		}
	}
}

func TestNext_Invalid(t *testing.T) {
	cases := []struct {
		from Status
		ev   Event
	}{
		{Rejected, ClientAccept},
		{Draft, AnalystOffer},
		{Disbursed, Cancel},
		{Signing, Cancel},
		{Enrichment, Cancel},
		{Submitted, Submit},
	}
	for _, c := range cases {
		if _, err := Next(c.from, c.ev); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s + %s: want ErrInvalidTransition, got %v", c.from, c.ev, err)
		}
	}
}

func TestEveryStatusHasClientMessage(t *testing.T) {
	for _, s := range AllStatuses() {
		if ClientMessage(s) == "" {
			t.Errorf("status %s has no client message", s)
		}
	}
}

func TestTerminalStatesHaveNoAllowedEvents(t *testing.T) {
	for _, s := range AllStatuses() {
		if IsTerminal(s) && len(Allowed(s)) != 0 {
			t.Errorf("terminal %s allows %v", s, Allowed(s))
		}
		if !IsTerminal(s) && len(Allowed(s)) == 0 {
			t.Errorf("non-terminal %s is a dead end", s)
		}
	}
}

func TestEveryNonDraftStatusIsReachable(t *testing.T) {
	seen := map[Status]bool{Draft: true}
	queue := []Status{Draft}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, ev := range Allowed(s) {
			n, _ := Next(s, ev)
			if !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	for _, s := range AllStatuses() {
		if !seen[s] {
			t.Errorf("status %s unreachable", s)
		}
	}
}

// TestStateDiagramUpToDate keeps docs/state-diagram.md in sync with the table.
// Run with UPDATE_DOCS=1 to regenerate.
func TestStateDiagramUpToDate(t *testing.T) {
	const path = "../../docs/state-diagram.md"
	want := DiagramMarkdown()
	if os.Getenv("UPDATE_DOCS") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing %s (run with UPDATE_DOCS=1): %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s is stale; run UPDATE_DOCS=1 go test ./internal/workflow", path)
	}
}

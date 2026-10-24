package demo

import (
	"context"
	"matritsa/internal/appdata"
	"strings"
	"testing"

	"matritsa/internal/events"
	"matritsa/internal/integrations"
	"matritsa/internal/pipeline"
	"matritsa/internal/seed"
	"matritsa/internal/store"
	"matritsa/internal/store/storetest"
)

type sources struct{}

func (sources) BKI(_ context.Context, inn string) (*integrations.BKIReport, error) {
	if c, ok := seed.ByINN(inn); ok && c.BKI != nil {
		return c.BKI, nil
	}
	return &integrations.BKIReport{}, nil
}

func (sources) EGRUL(_ context.Context, inn string) (integrations.EGRULRecord, error) {
	if c, ok := seed.ByINN(inn); ok {
		return c.EGRUL(), nil
	}
	return integrations.EGRULRecord{}, integrations.ErrNotFound
}

func newPipeline(t *testing.T) (*store.Store, *pipeline.Pipeline) {
	t.Helper()
	st := storetest.New(t)
	if err := store.Seed(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	cfg := pipeline.DefaultConfig()
	cfg.RetryDelay = 0
	return st, pipeline.New(st, sources{}, events.NewHub(), cfg)
}

// Every curated application must end exactly where its description says: Run fails otherwise.
func TestRun_EveryDemoApplicationEndsWhereItsStoryNeedsIt(t *testing.T) {
	st, p := newPipeline(t)
	if err := Run(context.Background(), st, p); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, a := range seed.DemoApplications() {
		counts[a.Want]++
	}
	var total int
	got := map[string]int{}
	for _, c := range seed.Companies() {
		u, err := st.GetUserByLogin(context.Background(), c.Login)
		if err != nil {
			t.Fatal(err)
		}
		apps, err := st.ListApplicationsByUser(context.Background(), u.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range apps {
			got[a.Status]++
			total++
		}
	}
	if total != len(seed.DemoApplications()) {
		t.Fatalf("expected %d applications, found %d", len(seed.DemoApplications()), total)
	}
	for status, n := range counts {
		if got[status] != n {
			t.Errorf("status %s: want %d, got %d", status, n, got[status])
		}
	}
}

func TestRun_TheReviewerSeesEveryKindOfOutcomeAtOnce(t *testing.T) {
	st, p := newPipeline(t)
	if err := Run(context.Background(), st, p); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, a := range seed.DemoApplications() {
		kinds[a.Want] = true
	}
	for _, want := range []string{"ANALYST_REVIEW", "NEEDS_INFO", "OFFER_MADE", "REJECTED", "DISBURSED", "DECLINED_BY_CLIENT", "CANCELLED", "DRAFT"} {
		if !kinds[want] {
			t.Errorf("the demo set has no application ending in %s", want)
		}
	}
}

func TestRun_IsIdempotent_AndNeverTouchesAPersonWithOwnApplications(t *testing.T) {
	st, p := newPipeline(t)
	ctx := context.Background()
	if err := Run(ctx, st, p); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		n := 0
		for _, c := range seed.Companies() {
			u, _ := st.GetUserByLogin(ctx, c.Login)
			apps, _ := st.ListApplicationsByUser(ctx, u.ID)
			n += len(apps)
		}
		return n
	}
	first := count()
	if err := Run(ctx, st, p); err != nil {
		t.Fatal(err)
	}
	if count() != first {
		t.Fatalf("a second run must add nothing: %d then %d", first, count())
	}
}

// A rejection by the rules must say so and name the reason, so that nobody has to dig for it.
func TestRun_AutomaticRejectionsCarryTheirReasons(t *testing.T) {
	st, p := newPipeline(t)
	ctx := context.Background()
	if err := Run(ctx, st, p); err != nil {
		t.Fatal(err)
	}
	reason := func(login, part string) {
		t.Helper()
		u, _ := st.GetUserByLogin(ctx, login)
		apps, _ := st.ListApplicationsByUser(ctx, u.ID)
		for _, a := range apps {
			if a.Status != "REJECTED" {
				continue
			}
			d := appdata.Parse(a.Data)
			if d.Rejection == nil || d.Rejection.By != "system" {
				t.Fatalf("%s: the rejection must be marked as made by the system: %+v", login, d.Rejection)
			}
			for _, r := range d.Rejection.Reasons {
				if strings.Contains(r, part) {
					return
				}
			}
			t.Fatalf("%s: reasons %v do not mention %q", login, d.Rejection.Reasons, part)
		}
		t.Fatalf("%s has no rejected application", login)
	}
	reason("client4", "не является действующей")
	reason("client6", "просрочка более 90")
	reason("client9", "Долговая нагрузка")
	reason("client10", "банкротства")
}

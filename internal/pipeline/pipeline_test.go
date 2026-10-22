package pipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"matritsa/internal/appdata"
	"matritsa/internal/events"
	"matritsa/internal/integrations"
	"matritsa/internal/pipeline"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/seed"
	"matritsa/internal/store"
	"matritsa/internal/store/db"
	"matritsa/internal/store/storetest"
	"matritsa/internal/workflow"
)

const rub = int64(100)

// seedSources answers like the mocks but straight from seed data, with optional failures.
type seedSources struct {
	failFirst int32 // number of initial BKI calls that fail
	calls     atomic.Int32
	down      atomic.Bool
}

func (s *seedSources) BKI(_ context.Context, inn string) (*integrations.BKIReport, error) {
	n := s.calls.Add(1)
	if s.down.Load() || n <= s.failFirst {
		return nil, integrations.ErrUnavailable
	}
	if c, ok := seed.ByINN(inn); ok && c.BKI != nil {
		return c.BKI, nil
	}
	return &integrations.BKIReport{}, nil
}

func (s *seedSources) EGRUL(_ context.Context, inn string) (integrations.EGRULRecord, error) {
	if c, ok := seed.ByINN(inn); ok {
		return c.EGRUL(), nil
	}
	return integrations.EGRULRecord{}, integrations.ErrNotFound
}

type env struct {
	s   *store.Store
	p   *pipeline.Pipeline
	src *seedSources
	hub *events.Hub
}

func newEnv(t *testing.T, src *seedSources) env {
	t.Helper()
	s := storetest.New(t)
	if err := store.Seed(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	hub := events.NewHub()
	cfg := pipeline.DefaultConfig()
	cfg.RetryDelay = 0
	p := pipeline.New(s, src, hub, cfg)
	return env{s: s, p: p, src: src, hub: hub}
}

// submitted creates an application for the demo company and submits it.
func (e env) submitted(t *testing.T, inn string, pid products.ID, amount int64, term int, withDocs bool) db.Application {
	t.Helper()
	ctx := context.Background()
	co, err := e.s.GetCompanyByINN(ctx, inn)
	if err != nil {
		t.Fatal(err)
	}
	var user db.User
	for _, c := range seed.Companies() {
		if c.INN == inn {
			user, _ = e.s.GetUserByLogin(ctx, c.Login)
		}
	}
	app, err := e.s.CreateApplication(ctx, db.CreateApplicationParams{
		CompanyID: co.ID, UserID: user.ID, ProductID: string(pid), Amount: amount, TermMonths: int32(term),
		Purpose: "Закупка сырья", Data: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if withDocs {
		e.addDocs(t, app.ID, pid)
	}
	if err := e.p.Apply(ctx, app.ID, workflow.Submit, "client:"+user.ID, "", app.Version); err != nil {
		t.Fatal(err)
	}
	app, _ = e.s.GetApplication(ctx, app.ID)
	return app
}

func (e env) addDocs(t *testing.T, appID string, pid products.ID) {
	t.Helper()
	p, _ := products.Get(pid)
	for _, d := range p.Documents {
		if !d.Required {
			continue
		}
		if _, err := e.s.InsertDocument(context.Background(), db.InsertDocumentParams{
			ApplicationID: appID, Kind: d.Kind, Filename: d.Kind + ".pdf", ContentType: "application/pdf", Size: 1000, Path: "/tmp/x",
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func (e env) status(t *testing.T, id string) workflow.Status {
	t.Helper()
	a, err := e.s.GetApplication(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return workflow.Status(a.Status)
}

func (e env) trail(t *testing.T, id string) []string {
	t.Helper()
	evs, _ := e.s.ListEvents(context.Background(), id)
	var out []string
	for _, ev := range evs {
		out = append(out, ev.ToStatus)
	}
	return out
}

func TestPipeline_ReliableCompany_AutoOffer(t *testing.T) {
	e := newEnv(t, &seedSources{})
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	if err := e.p.Process(context.Background(), app.ID); err != nil {
		t.Fatal(err)
	}
	if st := e.status(t, app.ID); st != workflow.OfferMade {
		t.Fatalf("want OFFER_MADE, got %s", st)
	}
	want := []string{"SUBMITTED", "INTAKE_CHECK", "ENRICHMENT", "SCORING", "OFFER_MADE"}
	got := e.trail(t, app.ID)
	if len(got) != len(want) {
		t.Fatalf("audit trail %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("audit trail %v, want %v", got, want)
		}
	}
	offer, err := e.s.GetActiveOffer(context.Background(), app.ID)
	if err != nil || offer.Amount != 1_000_000*rub || offer.RateBp != 1650 || offer.Source != "auto" {
		t.Fatalf("offer: %+v %v", offer, err)
	}
	if !offer.ExpiresAt.Time.After(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatal("offer must be valid for about 7 days")
	}
	if _, err := e.s.GetScoringResult(context.Background(), app.ID); err != nil {
		t.Fatalf("scoring result must be stored: %v", err)
	}
}

func TestPipeline_Liquidated_Rejected_WithReasons(t *testing.T) {
	e := newEnv(t, &seedSources{})
	app := e.submitted(t, "7701234504", products.WorkingCapital, 500_000*rub, 12, true)
	_ = e.p.Process(context.Background(), app.ID)
	if st := e.status(t, app.ID); st != workflow.Rejected {
		t.Fatalf("want REJECTED, got %s", st)
	}
	a, _ := e.s.GetApplication(context.Background(), app.ID)
	d := appdata.Parse(a.Data)
	if d.Rejection == nil || len(d.Rejection.Reasons) == 0 || d.Rejection.By != "system" {
		t.Fatalf("rejection reasons missing: %+v", d.Rejection)
	}
}

func TestPipeline_Borderline_GoesToAnalystReview(t *testing.T) {
	e := newEnv(t, &seedSources{})
	app := e.submitted(t, "771234500002", products.WorkingCapital, 3_000_000*rub, 24, true)
	_ = e.p.Process(context.Background(), app.ID)
	if st := e.status(t, app.ID); st != workflow.AnalystReview {
		t.Fatalf("want ANALYST_REVIEW, got %s", st)
	}
	a, _ := e.s.GetApplication(context.Background(), app.ID)
	d := appdata.Parse(a.Data)
	if d.Review == nil || d.Review.Recommended == nil || d.Review.Recommended.Amount >= 3_000_000*rub {
		t.Fatalf("review must carry a reduced recommendation: %+v", d.Review)
	}
}

func TestPipeline_MissingDocs_NeedsInfo_ThenResumes(t *testing.T) {
	e := newEnv(t, &seedSources{})
	ctx := context.Background()
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, false)
	_ = e.p.Process(ctx, app.ID)
	if st := e.status(t, app.ID); st != workflow.NeedsInfo {
		t.Fatalf("want NEEDS_INFO, got %s", st)
	}
	a, _ := e.s.GetApplication(ctx, app.ID)
	d := appdata.Parse(a.Data)
	if d.InfoRequest == nil || len(d.InfoRequest.Items) == 0 {
		t.Fatalf("client must be told what is missing: %+v", d.InfoRequest)
	}
	e.addDocs(t, app.ID, products.WorkingCapital)
	if err := e.p.Apply(ctx, app.ID, workflow.ClientProvidedInfo, "client", "Загрузил документы", a.Version); err != nil {
		t.Fatal(err)
	}
	_ = e.p.Process(ctx, app.ID)
	if st := e.status(t, app.ID); st != workflow.OfferMade {
		t.Fatalf("want OFFER_MADE after documents were provided, got %s", st)
	}
}

func TestApply_StaleVersion_ErrConflict(t *testing.T) {
	e := newEnv(t, &seedSources{})
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	err := e.p.Apply(context.Background(), app.ID, workflow.StartIntake, "system", "", app.Version-1)
	if !errors.Is(err, pipeline.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestApply_InvalidTransition_NotConflict(t *testing.T) {
	e := newEnv(t, &seedSources{})
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	err := e.p.Apply(context.Background(), app.ID, workflow.ClientAccept, "client", "", app.Version)
	if !errors.Is(err, workflow.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestPipeline_BKIDown_StaysInEnrichment_AndRetries(t *testing.T) {
	src := &seedSources{failFirst: 2}
	e := newEnv(t, src)
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	if err := e.p.Process(context.Background(), app.ID); err != nil {
		t.Fatal(err)
	}
	if st := e.status(t, app.ID); st != workflow.OfferMade {
		t.Fatalf("after 2 failures and a success want OFFER_MADE, got %s", st)
	}
	if src.calls.Load() != 3 {
		t.Fatalf("want 3 BKI calls, got %d", src.calls.Load())
	}
}

func TestPipeline_BKIDownForever_GoesToStaffWithManualCheck(t *testing.T) {
	src := &seedSources{}
	src.down.Store(true)
	e := newEnv(t, src)
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	_ = e.p.Process(context.Background(), app.ID)
	if st := e.status(t, app.ID); st != workflow.AnalystReview {
		t.Fatalf("after max attempts want ANALYST_REVIEW, got %s", st)
	}
	a, _ := e.s.GetApplication(context.Background(), app.ID)
	if d := appdata.Parse(a.Data); d.Review == nil || !d.Review.ManualCheck {
		t.Fatalf("manual check flag expected: %+v", d.Review)
	}
	if got := src.calls.Load(); got != int32(pipeline.DefaultConfig().MaxEnrichAttempts) {
		t.Fatalf("want %d attempts, got %d", pipeline.DefaultConfig().MaxEnrichAttempts, got)
	}
}

func TestPipeline_WaitsBetweenRetries(t *testing.T) {
	src := &seedSources{}
	src.down.Store(true)
	s := storetest.New(t)
	_ = store.Seed(context.Background(), s)
	cfg := pipeline.DefaultConfig()
	cfg.RetryDelay = time.Hour
	e := env{s: s, p: pipeline.New(s, src, events.NewHub(), cfg), src: src}
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	_ = e.p.Process(context.Background(), app.ID)
	if st := e.status(t, app.ID); st != workflow.Enrichment {
		t.Fatalf("with a long retry delay the application waits in ENRICHMENT, got %s", st)
	}
	if src.calls.Load() != 1 {
		t.Fatalf("only one attempt before the delay, got %d", src.calls.Load())
	}
}

func TestApply_ConcurrentClientAndAnalyst_OneWins(t *testing.T) {
	e := newEnv(t, &seedSources{})
	ctx := context.Background()
	app := e.submitted(t, "771234500002", products.WorkingCapital, 3_000_000*rub, 24, true)
	_ = e.p.Process(ctx, app.ID)
	a, _ := e.s.GetApplication(ctx, app.ID)
	if workflow.Status(a.Status) != workflow.AnalystReview {
		t.Fatalf("setup: want ANALYST_REVIEW, got %s", a.Status)
	}
	var wg sync.WaitGroup
	var ok, conflict int32
	for _, ev := range []workflow.Event{workflow.AnalystReject, workflow.Cancel} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := e.p.Apply(ctx, app.ID, ev, "actor", "", a.Version)
			switch {
			case err == nil:
				atomic.AddInt32(&ok, 1)
			case errors.Is(err, pipeline.ErrConflict):
				atomic.AddInt32(&conflict, 1)
			default:
				t.Errorf("unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || conflict != 1 {
		t.Fatalf("exactly one must win: ok=%d conflict=%d", ok, conflict)
	}
	evs, _ := e.s.ListEvents(ctx, app.ID)
	last := evs[len(evs)-1]
	if workflow.IsTerminal(workflow.Status(last.ToStatus)) == false {
		t.Fatalf("final state must be terminal, got %s", last.ToStatus)
	}
}

func TestPipeline_PublishesStatusEventsToOwnerAndStaff(t *testing.T) {
	e := newEnv(t, &seedSources{})
	user, _ := e.s.GetUserByLogin(context.Background(), "client1")
	ownerCh, cancel1 := e.hub.Subscribe(user.ID)
	staffCh, cancel2 := e.hub.Subscribe("staff")
	defer cancel1()
	defer cancel2()
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	_ = e.p.Process(context.Background(), app.ID)
	drain := func(ch <-chan events.Event) (last events.Event, n int) {
		for {
			select {
			case ev := <-ch:
				last, n = ev, n+1
			case <-time.After(50 * time.Millisecond):
				return
			}
		}
	}
	if last, n := drain(ownerCh); n == 0 || last.Status != "OFFER_MADE" {
		t.Fatalf("owner must see the final status, got %d events, last %+v", n, last)
	}
	if _, n := drain(staffCh); n == 0 {
		t.Fatal("staff must be notified about queue changes")
	}
}

func TestPipeline_CreatesNotificationForVisibleMilestones(t *testing.T) {
	e := newEnv(t, &seedSources{})
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	_ = e.p.Process(context.Background(), app.ID)
	user, _ := e.s.GetUserByLogin(context.Background(), "client1")
	ns, _ := e.s.ListNotifications(context.Background(), user.ID)
	if len(ns) == 0 {
		t.Fatal("client must get an in-app notification when the offer is ready")
	}
}

func TestPipeline_ExpiredOffer_BecomesExpired(t *testing.T) {
	e := newEnv(t, &seedSources{})
	ctx := context.Background()
	app := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	_ = e.p.Process(ctx, app.ID)
	if _, err := e.s.Pool.Exec(ctx, "UPDATE offers SET expires_at = now() - interval '1 hour' WHERE application_id = $1", app.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.p.ExpireOffers(ctx); err != nil {
		t.Fatal(err)
	}
	if st := e.status(t, app.ID); st != workflow.Expired {
		t.Fatalf("want EXPIRED, got %s", st)
	}
}

func TestPipeline_GradeIgnoresRequestSize_ExpectedLossDoesNot(t *testing.T) {
	e := newEnv(t, &seedSources{})
	ctx := context.Background()
	small := e.submitted(t, "7701234501", products.WorkingCapital, 1_000_000*rub, 12, true)
	big := e.submitted(t, "7701234501", products.WorkingCapital, 8_000_000*rub, 24, true)
	_ = e.p.Process(ctx, small.ID)
	_ = e.p.Process(ctx, big.ID)
	a, err := e.s.GetScoringResult(ctx, small.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.s.GetScoringResult(ctx, big.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a.Grade != b.Grade || a.Score != b.Score {
		t.Fatalf("the same company must get the same class for any request: %s/%d vs %s/%d", a.Grade, a.Score, b.Grade, b.Score)
	}
	var ma, mb scoring.Metrics
	if err := json.Unmarshal(a.Metrics, &ma); err != nil || ma.ExpectedLoss <= 0 {
		t.Fatalf("expected loss must be stored with the result: %s (%v)", a.Metrics, err)
	}
	if err := json.Unmarshal(b.Metrics, &mb); err != nil {
		t.Fatal(err)
	}
	if !(mb.ExpectedLoss > ma.ExpectedLoss) || !(mb.CumulativePD > ma.CumulativePD) {
		t.Fatalf("a bigger and longer loan must carry more expected loss: %+v vs %+v", ma, mb)
	}
	if ma.LGD != 0.6 || ma.PD <= 0 {
		t.Fatalf("PD and LGD must be recorded: %+v", ma)
	}
}

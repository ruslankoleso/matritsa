// Package pipeline orchestrates an application through the credit process.
// Apply is the only place where a status changes: it validates the transition
// with the workflow state machine, writes the audit trail, notifies the
// client and the bank team, all in one transaction.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"matritsa/internal/appdata"
	"matritsa/internal/events"
	"matritsa/internal/integrations"
	"matritsa/internal/scoring"
	"matritsa/internal/store"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

// AnyVersion skips the optimistic-lock check (system actors only).
const AnyVersion int32 = -1

var ErrConflict = errors.New("application was changed by someone else, reload and try again")

// Sources are the external systems; *integrations.Client satisfies it.
type Sources interface {
	BKI(ctx context.Context, inn string) (*integrations.BKIReport, error)
	EGRUL(ctx context.Context, inn string) (integrations.EGRULRecord, error)
}

type Config struct {
	RetryDelay        time.Duration
	MaxEnrichAttempts int
	OfferTTL          time.Duration
	PollInterval      time.Duration
	Scoring           scoring.Config
}

func DefaultConfig() Config {
	return Config{RetryDelay: 5 * time.Second, MaxEnrichAttempts: 6, OfferTTL: 7 * 24 * time.Hour, PollInterval: time.Second, Scoring: scoring.DefaultConfig()}
}

// Mutation runs inside the transition transaction, after the status was changed.
type Mutation func(ctx context.Context, q *db.Queries, app db.Application) error

type Pipeline struct {
	s    *store.Store
	src  Sources
	hub  *events.Hub
	cfg  Config
	wake chan string
	now  func() time.Time
}

func New(s *store.Store, src Sources, hub *events.Hub, cfg Config) *Pipeline {
	return &Pipeline{s: s, src: src, hub: hub, cfg: cfg, wake: make(chan string, 64), now: time.Now}
}

// notifyStatuses are the milestones a client should be told about explicitly.
var notifyStatuses = map[workflow.Status]bool{
	workflow.NeedsInfo: true, workflow.AnalystReview: true, workflow.OfferMade: true,
	workflow.Rejected: true, workflow.Disbursed: true, workflow.Expired: true,
}

// Apply performs one transition. expectedVersion is the version the caller saw
// (or AnyVersion); a mismatch returns ErrConflict.
func (p *Pipeline) Apply(ctx context.Context, appID string, ev workflow.Event, actor, comment string, expectedVersion int32, muts ...Mutation) error {
	var updated db.Application
	var from workflow.Status
	var notified bool
	err := p.s.WithTx(ctx, func(q *db.Queries) error {
		app, err := q.GetApplicationForUpdate(ctx, appID)
		if err != nil {
			return fmt.Errorf("load application: %w", err)
		}
		if expectedVersion != AnyVersion && app.Version != expectedVersion {
			return ErrConflict
		}
		from = workflow.Status(app.Status)
		to, err := workflow.Next(from, ev)
		if err != nil {
			return err
		}
		updated, err = q.TransitionApplication(ctx, db.TransitionApplicationParams{ID: appID, Version: app.Version, Status: string(to)})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if _, err := q.InsertEvent(ctx, db.InsertEventParams{
			ApplicationID: appID, Actor: actor, Event: string(ev), FromStatus: string(from), ToStatus: string(to), Comment: comment, Payload: []byte(`{}`),
		}); err != nil {
			return err
		}
		for _, m := range muts {
			if err := m(ctx, q, updated); err != nil {
				return err
			}
		}
		if notifyStatuses[to] {
			notified = true
			aid := appID
			if _, err := q.InsertNotification(ctx, db.InsertNotificationParams{UserID: app.UserID, ApplicationID: &aid, Text: workflow.ClientMessage(to)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	to := workflow.Status(updated.Status)
	p.hub.Publish(updated.UserID, events.Event{Type: "status", ApplicationID: appID, Status: string(to), Text: workflow.ClientMessage(to)})
	if notified {
		p.hub.Publish(updated.UserID, events.Event{Type: "notification", ApplicationID: appID, Status: string(to), Text: workflow.ClientMessage(to)})
	}
	p.hub.Publish("staff", events.Event{Type: "queue", ApplicationID: appID, Status: string(to)})
	slog.Info("application transition", "app", appID, "event", ev, "from", from, "to", to, "actor", actor)
	return nil
}

// Start asks the background worker to process an application right away.
func (p *Pipeline) Start(_ context.Context, appID string) error {
	select {
	case p.wake <- appID:
	default: // the periodic scan will pick it up
	}
	return nil
}

// Run is the background worker: it advances applications in system states and expires offers.
func (p *Pipeline) Run(ctx context.Context) {
	t := time.NewTicker(p.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-p.wake:
			p.process(ctx, id)
		case <-t.C:
			p.scan(ctx)
		}
	}
}

func (p *Pipeline) scan(ctx context.Context) {
	apps, err := p.s.ListApplicationsByStatus(ctx, db.ListApplicationsByStatusParams{
		Column1: []string{string(workflow.Submitted), string(workflow.IntakeCheck), string(workflow.Enrichment), string(workflow.Scoring)}, Limit: 50,
	})
	if err != nil {
		slog.Error("scan failed", "err", err)
		return
	}
	for _, a := range apps {
		p.process(ctx, a.ID)
	}
	if err := p.ExpireOffers(ctx); err != nil {
		slog.Error("expire offers failed", "err", err)
	}
}

// process runs the background stages of one application. With several instances of the API every worker
// scans the same table; the lock lets one of them take the application, so the bureau and the registry are
// asked once, not once per instance. The version check in Apply would keep the data right either way.
func (p *Pipeline) process(ctx context.Context, id string) {
	release, ok, err := p.s.TryLock(ctx, "pipeline:"+id)
	if err != nil {
		slog.Error("pipeline lock failed", "app", id, "err", err)
		return
	}
	if !ok {
		return // another worker is on it
	}
	defer release()
	if err := p.Process(ctx, id); err != nil {
		slog.Error("process failed", "app", id, "err", err)
	}
}

// Process advances an application until it needs a person or must wait.
func (p *Pipeline) Process(ctx context.Context, appID string) error {
	for i := 0; i < 50; i++ {
		more, err := p.Step(ctx, appID)
		if errors.Is(err, ErrConflict) {
			return nil // somebody else moved it; they own the next step
		}
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
	return nil
}

// Step runs one stage. more=true means another stage can run immediately.
func (p *Pipeline) Step(ctx context.Context, appID string) (more bool, err error) {
	app, err := p.s.GetApplication(ctx, appID)
	if err != nil {
		return false, err
	}
	switch workflow.Status(app.Status) {
	case workflow.Submitted:
		return true, p.Apply(ctx, appID, workflow.StartIntake, "system", "", app.Version)
	case workflow.IntakeCheck:
		return true, p.intake(ctx, app)
	case workflow.Enrichment:
		return p.enrich(ctx, app)
	case workflow.Scoring:
		return true, p.score(ctx, app)
	}
	return false, nil
}

// ExpireOffers moves applications whose offer lapsed to EXPIRED.
func (p *Pipeline) ExpireOffers(ctx context.Context) error {
	ids, err := p.s.ListExpiredOfferApplications(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		err := p.Apply(ctx, id, workflow.OfferExpired, "system", "Срок действия предложения истёк", AnyVersion)
		if err != nil && !errors.Is(err, workflow.ErrInvalidTransition) && !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return nil
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// mutateData loads the current data inside the transaction, edits it and stores it back.
func mutateData(edit func(d *appdata.Data)) Mutation {
	return func(ctx context.Context, q *db.Queries, app db.Application) error {
		d := appdata.Parse(app.Data)
		edit(&d)
		return q.SetApplicationData(ctx, db.SetApplicationDataParams{ID: app.ID, Data: d.Marshal()})
	}
}

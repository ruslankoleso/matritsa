// Package demo files the curated applications of the demo people, so that whoever opens the product
// finds every outcome already there: automatic offers and rejections, cases waiting for an analyst,
// questions to the client, a counter-offer, a disbursed loan.
package demo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"matritsa/internal/appdata"
	"matritsa/internal/decision"
	"matritsa/internal/pipeline"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/seed"
	"matritsa/internal/store"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

// settled are the statuses in which the pipeline waits for a person or has finished.
var settled = map[workflow.Status]bool{
	workflow.AnalystReview: true, workflow.OfferMade: true, workflow.Rejected: true, workflow.NeedsInfo: true,
}

// Run files the applications through the real pipeline. It is idempotent: a person who already has
// applications is skipped, so a restart (or a person's own applications) never produces duplicates.
func Run(ctx context.Context, st *store.Store, p *pipeline.Pipeline) error {
	staff, err := st.GetUserByLogin(ctx, "staff1")
	if err != nil {
		return fmt.Errorf("staff account: %w", err)
	}
	// decided remembers, once per person, whether they already have applications of their own.
	decided := map[string]bool{}
	for _, a := range seed.DemoApplications() {
		user, err := st.GetUserByLogin(ctx, a.Login)
		if err != nil || user.CompanyID == nil {
			return fmt.Errorf("demo user %s: %w", a.Login, err)
		}
		skip, seen := decided[a.Login]
		if !seen {
			existing, err := st.ListApplicationsByUser(ctx, user.ID)
			if err != nil {
				return err
			}
			skip = len(existing) > 0
			decided[a.Login] = skip
		}
		if skip {
			continue
		}
		if err := file(ctx, st, p, user, staff, a); err != nil {
			return fmt.Errorf("%s %s: %w", a.Login, a.Purpose, err)
		}
	}
	slog.Info("demo applications ready")
	return nil
}

func file(ctx context.Context, st *store.Store, p *pipeline.Pipeline, user, staff db.User, a seed.DemoApp) error {
	prod, ok := products.Get(products.ID(a.Product))
	if !ok {
		return fmt.Errorf("unknown product %s", a.Product)
	}
	app, err := st.CreateApplication(ctx, db.CreateApplicationParams{
		CompanyID: *user.CompanyID, UserID: user.ID, ProductID: a.Product, Amount: a.Amount, TermMonths: int32(a.Term), Purpose: a.Purpose, Data: appdata.Data{}.Marshal(),
	})
	if err != nil {
		return err
	}
	if !a.NoDocs {
		for _, d := range prod.Documents {
			if !d.Required {
				continue
			}
			if _, err := st.InsertDocument(ctx, db.InsertDocumentParams{
				ApplicationID: app.ID, Kind: d.Kind, Filename: d.Title + ".pdf", ContentType: "application/pdf", Size: 184_320, Path: "demo/" + d.Kind + ".pdf",
			}); err != nil {
				return err
			}
		}
	}
	if !a.Submit {
		return expect(ctx, st, app.ID, a.Want)
	}
	client := "client:" + user.ID
	if err := p.Apply(ctx, app.ID, workflow.Submit, client, "Заявка отправлена клиентом", pipeline.AnyVersion); err != nil {
		return err
	}
	status, err := settle(ctx, st, p, app.ID)
	if err != nil {
		return err
	}
	if a.Staff != "" && status == workflow.AnalystReview {
		if err := staffAct(ctx, st, p, staff, app.ID, a); err != nil {
			return err
		}
	}
	switch {
	case a.Client == "accept":
		if err := p.Apply(ctx, app.ID, workflow.ClientAccept, client, "Клиент принял предложение", pipeline.AnyVersion); err != nil {
			return err
		}
		if err := p.Apply(ctx, app.ID, workflow.Disburse, "system", "Документы подписаны, деньги перечислены", pipeline.AnyVersion); err != nil {
			return err
		}
	case a.Client == "decline":
		if err := p.Apply(ctx, app.ID, workflow.ClientDecline, client, "Клиент отказался от предложения", pipeline.AnyVersion); err != nil {
			return err
		}
	case a.Cancel:
		if err := p.Apply(ctx, app.ID, workflow.Cancel, client, "Клиент отозвал заявку", pipeline.AnyVersion); err != nil {
			return err
		}
	}
	return expect(ctx, st, app.ID, a.Want)
}

// settle runs the pipeline until the application waits for a person or is finished; sources that are
// still starting are retried for a while.
func settle(ctx context.Context, st *store.Store, p *pipeline.Pipeline, id string) (workflow.Status, error) {
	deadline := time.Now().Add(90 * time.Second)
	for {
		if err := p.Process(ctx, id); err != nil {
			return "", err
		}
		cur, err := st.GetApplication(ctx, id)
		if err != nil {
			return "", err
		}
		if s := workflow.Status(cur.Status); settled[s] {
			return s, nil
		}
		if time.Now().After(deadline) {
			return workflow.Status(cur.Status), fmt.Errorf("the application did not settle: %s", cur.Status)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func staffAct(ctx context.Context, st *store.Store, p *pipeline.Pipeline, staff db.User, id string, a seed.DemoApp) error {
	actor := "staff:" + staff.ID
	cur, err := st.GetApplication(ctx, id)
	if err != nil {
		return err
	}
	data := appdata.Parse(cur.Data)
	switch a.Staff {
	case "counter":
		c := a.Counter
		offer := decision.Offer{Amount: c.Amount, TermMonths: c.Term, RateBP: c.RateBP, MonthlyPayment: scoring.MonthlyPayment(c.Amount, c.RateBP, c.Term), Source: "analyst"}
		return p.Apply(ctx, id, workflow.AnalystOffer, actor, a.StaffComment, pipeline.AnyVersion, p.StoreOffer(offer))
	case "approve":
		if data.Review == nil || data.Review.Recommended == nil {
			return fmt.Errorf("no recommendation to approve")
		}
		offer := *data.Review.Recommended
		offer.Source = "analyst"
		return p.Apply(ctx, id, workflow.AnalystOffer, actor, a.StaffComment, pipeline.AnyVersion, p.StoreOffer(offer))
	case "request_info":
		ask := func(ctx context.Context, q *db.Queries, app db.Application) error {
			d := appdata.Parse(app.Data)
			d.InfoRequest = &appdata.InfoRequest{Text: a.StaffComment, RequestedBy: "staff", At: time.Now()}
			return q.SetApplicationData(ctx, db.SetApplicationDataParams{ID: app.ID, Data: d.Marshal()})
		}
		return p.Apply(ctx, id, workflow.AnalystRequestInfo, actor, a.StaffComment, pipeline.AnyVersion, ask)
	case "reject":
		reasons := []string{"Заявка не соответствует требованиям банка"}
		if data.Review != nil && len(data.Review.Reasons) > 0 {
			reasons = data.Review.Reasons
		}
		reject := func(ctx context.Context, q *db.Queries, app db.Application) error {
			d := appdata.Parse(app.Data)
			d.Rejection = &appdata.Rejection{Reasons: reasons, By: "staff"}
			return q.SetApplicationData(ctx, db.SetApplicationDataParams{ID: app.ID, Data: d.Marshal()})
		}
		return p.Apply(ctx, id, workflow.AnalystReject, actor, a.StaffComment, pipeline.AnyVersion, reject)
	}
	return fmt.Errorf("unknown staff action %q", a.Staff)
}

func expect(ctx context.Context, st *store.Store, id, want string) error {
	cur, err := st.GetApplication(ctx, id)
	if err != nil {
		return err
	}
	if cur.Status != want {
		return fmt.Errorf("ended in %s, expected %s", cur.Status, want)
	}
	return nil
}

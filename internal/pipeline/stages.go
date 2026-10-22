package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"matritsa/internal/appdata"
	"matritsa/internal/decision"
	"matritsa/internal/integrations"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

// Missing lists, in plain language, what an application still lacks before it can be processed.
func (p *Pipeline) Missing(ctx context.Context, app db.Application) ([]string, error) {
	var missing []string
	prod, ok := products.Get(products.ID(app.ProductID))
	if !ok {
		missing = append(missing, "Выберите кредитный продукт")
	} else {
		for _, fe := range products.ValidateRequest(prod, app.Amount, int(app.TermMonths)) {
			missing = append(missing, fe.Message)
		}
	}
	if strings.TrimSpace(app.Purpose) == "" {
		missing = append(missing, "Расскажите, на что нужны деньги")
	}
	if ok {
		docs, err := p.s.ListDocuments(ctx, app.ID)
		if err != nil {
			return nil, err
		}
		have := map[string]bool{}
		for _, d := range docs {
			have[d.Kind] = true
		}
		for _, req := range prod.Documents {
			if req.Required && !have[req.Kind] {
				missing = append(missing, "Загрузите документ: "+req.Title)
			}
		}
	}
	return missing, nil
}

// intake checks that the application is complete and consistent.
func (p *Pipeline) intake(ctx context.Context, app db.Application) error {
	missing, err := p.Missing(ctx, app)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return p.Apply(ctx, app.ID, workflow.IntakePassed, "system", "Заявка заполнена полностью", app.Version)
	}
	req := appdata.InfoRequest{
		Text:        "Чтобы продолжить, нужно дополнить заявку.",
		Items:       missing,
		RequestedBy: "system",
		At:          p.now(),
	}
	return p.Apply(ctx, app.ID, workflow.IntakeNeedsInfo, "system", "Не хватает: "+strings.Join(missing, "; "), app.Version,
		mutateData(func(d *appdata.Data) { d.InfoRequest = &req }))
}

// enrich fetches registry and bureau data, retrying while the sources are down.
func (p *Pipeline) enrich(ctx context.Context, app db.Application) (more bool, err error) {
	data := appdata.Parse(app.Data)
	en := data.Enrichment
	if en == nil {
		en = &appdata.Enrichment{}
	}
	if !en.NextAt.IsZero() && p.now().Before(en.NextAt) {
		return false, nil // waiting for the next retry
	}
	co, err := p.s.GetCompany(ctx, app.CompanyID)
	if err != nil {
		return false, err
	}

	var srcErr error
	if en.EGRUL == nil {
		rec, err := p.src.EGRUL(ctx, co.Inn)
		switch {
		case errors.Is(err, integrations.ErrNotFound):
			en.EGRUL = &integrations.EGRULRecord{INN: co.Inn, Status: "not_found"}
		case err != nil:
			srcErr = err
		default:
			en.EGRUL = &rec
		}
	}
	if srcErr == nil && en.BKI == nil {
		rep, err := p.src.BKI(ctx, co.Inn)
		if err != nil {
			srcErr = err
		} else {
			en.BKI = rep
		}
	}

	if srcErr == nil {
		en.LastError, en.NextAt = "", time.Time{}
		return true, p.Apply(ctx, app.ID, workflow.EnrichmentDone, "system", "Данные из реестра и бюро кредитных историй получены", app.Version,
			mutateData(func(d *appdata.Data) { d.Enrichment = en }))
	}

	en.Attempts++
	en.LastError = srcErr.Error()
	if en.Attempts >= p.cfg.MaxEnrichAttempts {
		return true, p.Apply(ctx, app.ID, workflow.EnrichmentFailed, "system",
			fmt.Sprintf("Внешний источник недоступен после %d попыток — нужна ручная проверка", en.Attempts), app.Version,
			mutateData(func(d *appdata.Data) {
				d.Enrichment = en
				d.Review = &appdata.Review{ManualCheck: true, Reasons: []string{"Внешние источники данных временно недоступны — проверьте компанию вручную"}}
			}))
	}
	en.NextAt = p.now().Add(p.cfg.RetryDelay)
	err = p.s.WithTx(ctx, func(q *db.Queries) error {
		d := appdata.Parse(app.Data)
		d.Enrichment = en
		if err := q.SetApplicationData(ctx, db.SetApplicationDataParams{ID: app.ID, Data: d.Marshal()}); err != nil {
			return err
		}
		_, err := q.InsertEvent(ctx, db.InsertEventParams{
			ApplicationID: app.ID, Actor: "system", Event: "EnrichmentRetry", FromStatus: app.Status, ToStatus: app.Status,
			Comment: fmt.Sprintf("Внешний источник временно недоступен (попытка %d из %d)", en.Attempts, p.cfg.MaxEnrichAttempts), Payload: []byte(`{}`),
		})
		return err
	})
	return true, err // let Process loop: it returns once NextAt is in the future
}

// score evaluates risk, decides and routes the application.
func (p *Pipeline) score(ctx context.Context, app db.Application) error {
	data := appdata.Parse(app.Data)
	prod, ok := products.Get(products.ID(app.ProductID))
	if !ok || data.Enrichment == nil || data.Enrichment.EGRUL == nil {
		return p.Apply(ctx, app.ID, workflow.AutoReject, "system", "Недостаточно данных для оценки", app.Version,
			mutateData(func(d *appdata.Data) {
				d.Rejection = &appdata.Rejection{By: "system", Reasons: []string{"Не удалось получить данные для оценки заявки"}}
			}))
	}
	co, err := p.s.GetCompany(ctx, app.CompanyID)
	if err != nil {
		return err
	}
	fin, err := p.s.ListFinancials(ctx, app.CompanyID)
	if err != nil {
		return err
	}
	revenue := make([]int64, len(fin))
	for i, f := range fin {
		revenue[i] = f.Revenue
	}
	age := int(p.now().Sub(co.RegisteredAt.Time).Hours() / 24 / 30)
	in := scoring.Input{
		CompanyAgeMonths: age, MonthlyRevenue: revenue, MonthlyObligations: co.MonthlyObligations,
		BKI: data.Enrichment.BKI, EGRUL: *data.Enrichment.EGRUL,
		Amount: app.Amount, TermMonths: int(app.TermMonths), RateBP: prod.BaseRateBP, OKVEDGroup: int(co.OkvedGroup), LGD: prod.LGD,
	}
	res := scoring.Score(in, p.cfg.Scoring)
	dec := decision.Decide(res, prod, app.Amount, int(app.TermMonths))

	save := func(ctx context.Context, q *db.Queries, a db.Application) error {
		factors, _ := json.Marshal(res.Factors)
		stops, _ := json.Marshal(append([]string{}, res.HardStops...))
		en, _ := json.Marshal(data.Enrichment)
		metrics, _ := json.Marshal(res.Metrics())
		return q.UpsertScoringResult(ctx, db.UpsertScoringResultParams{
			ApplicationID: a.ID, Score: int32(res.Score), Grade: string(res.Grade), Factors: factors, HardStops: stops,
			MaxAffordable: res.MaxAffordableAmount, DebtLoad: res.DebtLoad, Enrichment: en, Metrics: metrics,
		})
	}
	summary := fmt.Sprintf("Класс риска %s, %d баллов из 100", res.Grade, res.Score)

	switch dec.Outcome {
	case decision.AutoOffer:
		return p.Apply(ctx, app.ID, workflow.AutoOffer, "system", summary+"; решение принято автоматически", app.Version, save,
			p.storeOffer(*dec.Offer))
	case decision.Review:
		return p.Apply(ctx, app.ID, workflow.SendToReview, "system", summary+"; нужна проверка сотрудником", app.Version, save,
			mutateData(func(d *appdata.Data) { d.Review = &appdata.Review{Recommended: dec.Recommended, Reasons: dec.Reasons} }))
	default:
		return p.Apply(ctx, app.ID, workflow.AutoReject, "system", summary+"; отказ", app.Version, save,
			mutateData(func(d *appdata.Data) { d.Rejection = &appdata.Rejection{By: "system", Reasons: dec.Reasons} }))
	}
}

// StoreOffer returns a mutation that makes offer the only active offer of the application.
func (p *Pipeline) StoreOffer(o decision.Offer) Mutation { return p.storeOffer(o) }

func (p *Pipeline) storeOffer(o decision.Offer) Mutation {
	return func(ctx context.Context, q *db.Queries, app db.Application) error {
		if err := q.DeactivateOffers(ctx, app.ID); err != nil {
			return err
		}
		_, err := q.InsertOffer(ctx, db.InsertOfferParams{
			ApplicationID: app.ID, Amount: o.Amount, TermMonths: int32(o.TermMonths), RateBp: int32(o.RateBP),
			MonthlyPayment: o.MonthlyPayment, Source: o.Source, ExpiresAt: ts(p.now().Add(p.cfg.OfferTTL)),
		})
		return err
	}
}

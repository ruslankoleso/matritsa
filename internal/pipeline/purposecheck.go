package pipeline

import (
	"context"
	"encoding/json"
	"strconv"

	"matritsa/internal/appdata"
	"matritsa/internal/decision"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

// EventPurposeChecked is the journal entry of the verdict of the analyst on the purpose and the amount. It stays
// with the bank: the client sees the outcome of the application, not the working notes.
const EventPurposeChecked = "PurposeChecked"

// CheckPurpose records the verdict of the analyst on the purpose and the amount (criterion 12 of the scorecard) and
// scores the application again: the points, the class, the recommendation and the reasons for the analyst follow
// the verdict. The status does not change; the version does, so a card opened earlier cannot be decided on stale figures.
func (p *Pipeline) CheckPurpose(ctx context.Context, appID, staffID, verdict, comment string) error {
	return p.s.WithTx(ctx, func(q *db.Queries) error {
		app, err := q.GetApplicationForUpdate(ctx, appID)
		if err != nil {
			return err
		}
		data := appdata.Parse(app.Data)
		prod, ok := products.Get(products.ID(app.ProductID))
		if workflow.Status(app.Status) != workflow.AnalystReview || !ok || data.Enrichment == nil || data.Enrichment.EGRUL == nil {
			return ErrConflict
		}
		data.PurposeVerdict = verdict
		in, err := p.scoreInput(ctx, app, data, prod)
		if err != nil {
			return err
		}
		res, dec := p.evaluate(in, prod, app)
		if data.Review == nil {
			data.Review = &appdata.Review{}
		}
		data.Review.Recommended, data.Review.Reasons = dec.Recommended, dec.Reasons
		switch dec.Outcome {
		case decision.AutoOffer: // the analyst has the application anyway: the offer is a recommendation
			rec := *dec.Offer
			rec.Source = "recommended"
			data.Review.Recommended = &rec
		case decision.Reject:
			data.Review.Reasons = append([]string{"Система автоматически отказала бы:"}, dec.Reasons...)
		}
		if err := saveScoring(ctx, q, app.ID, res, data); err != nil {
			return err
		}
		if err := q.BumpApplicationData(ctx, db.BumpApplicationDataParams{ID: app.ID, Data: data.Marshal()}); err != nil {
			return err
		}
		_, err = q.InsertEvent(ctx, db.InsertEventParams{
			ApplicationID: app.ID, Actor: "staff:" + staffID, Event: EventPurposeChecked, FromStatus: app.Status, ToStatus: app.Status,
			Comment: purposeComment(verdict, comment, res), Payload: []byte(`{}`),
		})
		return err
	})
}

func purposeComment(verdict, comment string, res scoring.Result) string {
	text := map[string]string{
		scoring.PurposeConfirmed: "Цель и сумма подтверждены", scoring.PurposeClarify: "Цель и сумма требуют уточнения", scoring.PurposeRejected: "Цель и сумма не подтверждены",
	}[verdict]
	if comment != "" {
		text += ": " + comment
	}
	return text + ". Пересчёт: класс " + string(res.Grade) + ", " + strconv.Itoa(res.Score) + " баллов из 100"
}

// saveScoring stores the result of the model next to the application.
func saveScoring(ctx context.Context, q *db.Queries, appID string, res scoring.Result, data appdata.Data) error {
	factors, _ := json.Marshal(res.Factors)
	stops, _ := json.Marshal(append([]string{}, res.HardStops...))
	en, _ := json.Marshal(data.Enrichment)
	metrics, _ := json.Marshal(res.Metrics())
	return q.UpsertScoringResult(ctx, db.UpsertScoringResultParams{
		ApplicationID: appID, Score: int32(res.Score), Grade: string(res.Grade), Factors: factors, HardStops: stops,
		MaxAffordable: res.MaxAffordableAmount, DebtLoad: res.DebtLoad, Enrichment: en, Metrics: metrics,
	})
}

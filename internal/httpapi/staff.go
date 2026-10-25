package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"matritsa/internal/appdata"
	"matritsa/internal/auth"
	"matritsa/internal/decision"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

func (s *Server) GetQueue(w http.ResponseWriter, r *http.Request, params gen.GetQueueParams) {
	arg := db.ListQueueParams{}
	if params.Status != nil {
		arg.Status = strings.TrimSpace(*params.Status)
	}
	if params.Q != nil {
		arg.Q = strings.TrimSpace(*params.Q)
	}
	rows, err := s.Store.ListQueue(r.Context(), arg)
	if err != nil {
		internalError(w, r, err)
		return
	}
	names := map[string]string{}
	items := make([]gen.QueueItem, 0, len(rows))
	for _, row := range rows {
		mc, _ := row.ManualCheck.(bool)
		it := gen.QueueItem{
			Id: uid(row.ID), CompanyName: row.CompanyName, Inn: row.CompanyInn, ProductTitle: productTitle(row.ProductID),
			Amount: row.Amount, TermMonths: int(row.TermMonths), Status: gen.Status(row.Status), UpdatedAt: row.UpdatedAt.Time,
			WaitingMinutes: int(time.Since(row.UpdatedAt.Time).Minutes()), ManualCheck: mc, Automatic: row.Automatic.Valid && row.Automatic.Bool,
		}
		if row.Grade.Valid {
			g := row.Grade.String
			it.Grade = &g
		}
		if row.Score.Valid {
			sc := int(row.Score.Int32)
			it.Score = &sc
		}
		if row.AssignedTo != nil {
			if n, ok := names[*row.AssignedTo]; ok {
				it.AssignedTo = &n
			} else if u, err := s.Store.GetUser(r.Context(), *row.AssignedTo); err == nil {
				names[*row.AssignedTo] = u.FullName
				n := u.FullName
				it.AssignedTo = &n
			}
		}
		items = append(items, it)
	}
	counts := map[string]int{}
	cr, err := s.Store.CountApplicationsByStatus(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	for _, c := range cr {
		counts[c.Status] = int(c.N)
	}
	writeJSON(w, http.StatusOK, gen.Queue{Items: items, Counts: counts})
}

func (s *Server) loadScoring(ctx context.Context, appID string) (*scoring.Result, error) {
	sr, err := s.Store.GetScoringResult(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res := scoring.Result{Score: int(sr.Score), Grade: scoring.Grade(sr.Grade), MaxAffordableAmount: sr.MaxAffordable, DebtLoad: sr.DebtLoad}
	_ = json.Unmarshal(sr.Factors, &res.Factors)
	_ = json.Unmarshal(sr.HardStops, &res.HardStops)
	var m scoring.Metrics // results stored before the expected-loss model have empty metrics
	if json.Unmarshal(sr.Metrics, &m) == nil {
		res.PD, res.CumulativePD, res.LGD, res.EAD, res.ExpectedLoss, res.ELRate, res.RiskPremiumBP, res.Payment = m.PD, m.CumulativePD, m.LGD, m.EAD, m.ExpectedLoss, m.ELRate, m.RiskPremiumBP, m.Payment
		res.RepaidTotal, res.RepaidCoverage = m.RepaidTotal, m.RepaidCoverage
		res.DSCR, res.MonthlyCashFlow, res.MaxAmountByLoad, res.MaxAmountByDSCR = m.DSCR, m.MonthlyCashFlow, m.MaxAmountByLoad, m.MaxAmountByDSCR
		res.HasCoverFigures = m.V >= 2
	}
	return &res, nil
}

func (s *Server) staffCard(ctx context.Context, a db.Application) (gen.StaffApplication, error) {
	view, err := s.loadApplication(ctx, a)
	if err != nil {
		return gen.StaffApplication{}, err
	}
	evs, err := s.Store.ListEvents(ctx, a.ID)
	if err != nil {
		return gen.StaffApplication{}, err
	}
	data := appdata.Parse(a.Data)
	card := gen.StaffApplication{Application: view, Events: toTimeline(evs), AllowedActions: []gen.StaffApplicationAllowedActions{}, Version: &view.Version}

	res, err := s.loadScoring(ctx, a.ID)
	if err != nil {
		return gen.StaffApplication{}, err
	}
	if res != nil {
		factors := make([]gen.Factor, len(res.Factors))
		for i, f := range res.Factors {
			factors[i] = gen.Factor{Key: f.Key, Title: f.Title, Points: f.Points, Max: f.Max, Note: f.Note}
			if f.NoData {
				nd := true
				factors[i].NoData = &nd
			}
		}
		stops := res.HardStops
		if stops == nil {
			stops = []string{}
		}
		card.Scoring = &gen.ScoringResult{
			Score: res.Score, Grade: gen.ScoringResultGrade(res.Grade), Factors: factors, HardStops: stops,
			MaxAffordableAmount: res.MaxAffordableAmount, DebtLoad: float32(res.DebtLoad),
		}
		q := scoring.QualityOf(res.Grade)
		card.Scoring.QualityCategory, card.Scoring.ReserveRange, card.Scoring.RiskLabel = &q.Category, &q.Reserve, &q.Risk
		maxLoad := float32(scoring.DefaultConfig().MaxLoad)
		card.Scoring.MaxLoad = &maxLoad
		if res.HasCoverFigures {
			rt, rc, minRc := res.RepaidTotal, float32(res.RepaidCoverage), float32(scoring.DefaultConfig().MinRepaidCoverage)
			card.Scoring.RepaidTotal, card.Scoring.RepaidCoverage, card.Scoring.MinRepaidCoverage = &rt, &rc, &minRc
			dscr, minDscr, cash := float32(res.DSCR), float32(scoring.DefaultConfig().MinDSCR), res.MonthlyCashFlow
			byLoad, byDscr := res.MaxAmountByLoad, res.MaxAmountByDSCR
			card.Scoring.Dscr, card.Scoring.MinDscr, card.Scoring.MonthlyCashFlow, card.Scoring.MaxAmountByLoad, card.Scoring.MaxAmountByDscr = &dscr, &minDscr, &cash, &byLoad, &byDscr
		}
		if res.EAD > 0 {
			pd, cum, lgd, rate, prem := float32(res.PD), float32(res.CumulativePD), float32(res.LGD), float32(res.ELRate), res.RiskPremiumBP
			ead, el, pay := res.EAD, res.ExpectedLoss, res.Payment
			card.Scoring.Pd, card.Scoring.CumulativePd, card.Scoring.Lgd, card.Scoring.Ead = &pd, &cum, &lgd, &ead
			card.Scoring.ExpectedLoss, card.Scoring.ExpectedLossRate, card.Scoring.RiskPremiumBp, card.Scoring.Payment = &el, &rate, &prem, &pay
		}
	}
	if en := data.Enrichment; en != nil {
		e := gen.Enrichment{Attempts: en.Attempts}
		if en.LastError != "" {
			le := en.LastError
			e.LastError = &le
		}
		if en.BKI != nil {
			closed, repaid := en.BKI.ClosedLoans, en.BKI.TotalRepaid
			restructured := en.BKI.Restructured
			e.Bki = &gen.BKIReport{HasHistory: en.BKI.HasHistory, ActiveLoans: en.BKI.ActiveLoans, TotalDebt: en.BKI.TotalDebt, MaxOverdueDays: en.BKI.MaxOverdueDays, Score: en.BKI.Score, ClosedLoans: &closed, TotalRepaid: &repaid, Restructured: &restructured}
		}
		if en.EGRUL != nil {
			g := en.EGRUL
			rec := gen.EGRULRecord{Inn: g.INN, Name: g.Name, Status: g.Status, Bankrupt: &g.Bankrupt}
			if !g.RegisteredAt.IsZero() {
				rec.RegisteredAt = &g.RegisteredAt
			}
			rec.Okved, rec.Director, rec.Address = &g.OKVED, &g.Director, &g.Address
			lit := g.Litigation
			rec.Litigation = &lit
			if f := g.Finance; f != nil {
				rec.Finance = &gen.FinanceData{GrossMargin: float32(f.GrossMargin), NoiMargin: float32(f.NOIMargin), CurrentRatio: float32(f.CurrentRatio), StatementAnomaly: f.StatementAnomaly}
			}
			e.Egrul = &rec
		}
		card.Enrichment = &e
	}
	if rv := data.Review; rv != nil {
		if rv.Recommended != nil {
			o := fromDecisionOffer(*rv.Recommended, a, time.Now().Add(s.OfferTTL))
			card.Recommended = &o
		}
		if len(rv.Reasons) > 0 {
			card.Reasons = &rv.Reasons
		}
		if rv.ManualCheck {
			t := true
			card.ManualCheck = &t
		}
	}
	if a.AssignedTo != nil {
		if u, err := s.Store.GetUser(ctx, *a.AssignedTo); err == nil {
			n := u.FullName
			card.AssignedTo = &n
		}
	}
	if workflow.Status(a.Status) == workflow.AnalystReview {
		card.AllowedActions = []gen.StaffApplicationAllowedActions{
			gen.StaffApplicationAllowedActionsApprove, gen.StaffApplicationAllowedActionsCounter,
			gen.StaffApplicationAllowedActionsRequestInfo, gen.StaffApplicationAllowedActionsReject,
		}
	}
	return card, nil
}

func (s *Server) staffApp(w http.ResponseWriter, r *http.Request, id string) (db.Application, bool) {
	a, err := s.Store.GetApplication(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена")
		return db.Application{}, false
	}
	if err != nil {
		internalError(w, r, err)
		return db.Application{}, false
	}
	return a, true
}

func (s *Server) respondCard(w http.ResponseWriter, r *http.Request, id string) {
	a, ok := s.staffApp(w, r, id)
	if !ok {
		return
	}
	card, err := s.staffCard(r.Context(), a)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) GetStaffApplication(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	s.respondCard(w, r, id.String())
}

func (s *Server) ClaimApplication(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	claims, _ := auth.FromContext(r.Context())
	a, ok := s.staffApp(w, r, id.String())
	if !ok {
		return
	}
	if err := s.Store.AssignApplication(r.Context(), db.AssignApplicationParams{ID: a.ID, AssignedTo: &claims.UserID}); err != nil {
		internalError(w, r, err)
		return
	}
	s.respondCard(w, r, a.ID)
}

func (s *Server) MakeDecision(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	claims, _ := auth.FromContext(r.Context())
	var req gen.DecisionRequest
	if !decodeBody(w, r, &req) {
		return
	}
	a, ok := s.staffApp(w, r, id.String())
	if !ok {
		return
	}
	if workflow.Status(a.Status) != workflow.AnalystReview {
		writeError(w, http.StatusConflict, "invalid_state", "По этой заявке решение сейчас не требуется")
		return
	}
	comment := ""
	if req.Comment != nil {
		comment = strings.TrimSpace(*req.Comment)
	}
	prod, _ := products.Get(products.ID(a.ProductID))
	data := appdata.Parse(a.Data)
	actor := "staff:" + claims.UserID
	version := int32(req.Version)

	var err error
	switch req.Action {
	case gen.DecisionRequestActionApprove:
		offer, oerr := s.approvalOffer(r.Context(), a, prod, data)
		if oerr != nil {
			internalError(w, r, oerr)
			return
		}
		if comment == "" {
			comment = "Одобрено сотрудником банка"
		}
		err = s.Pipeline.Apply(r.Context(), a.ID, workflow.AnalystOffer, actor, comment, version, s.Pipeline.StoreOffer(offer))
	case gen.DecisionRequestActionCounter:
		if req.Offer == nil {
			writeFieldErrors(w, http.StatusUnprocessableEntity, "Укажите новые условия", []fieldErr{{"offer", "Заполните сумму, срок и ставку"}})
			return
		}
		var fes []fieldErr
		for _, fe := range products.ValidateRequest(prod, req.Offer.Amount, req.Offer.TermMonths) {
			fes = append(fes, fieldErr{"offer." + fe.Field, fe.Message})
		}
		if req.Offer.RateBp < 100 || req.Offer.RateBp > 5000 {
			fes = append(fes, fieldErr{"offer.rateBp", "Ставка должна быть от 1% до 50% годовых"})
		}
		if len(fes) > 0 {
			writeFieldErrors(w, http.StatusUnprocessableEntity, "Проверьте условия предложения", fes)
			return
		}
		offer := decision.Offer{
			Amount: req.Offer.Amount, TermMonths: req.Offer.TermMonths, RateBP: req.Offer.RateBp, Source: "analyst",
			MonthlyPayment: scoring.MonthlyPayment(req.Offer.Amount, req.Offer.RateBp, req.Offer.TermMonths),
		}
		if comment == "" {
			comment = "Предложены другие условия"
		}
		err = s.Pipeline.Apply(r.Context(), a.ID, workflow.AnalystOffer, actor, comment, version, s.Pipeline.StoreOffer(offer))
	case gen.DecisionRequestActionRequestInfo:
		if comment == "" {
			writeFieldErrors(w, http.StatusUnprocessableEntity, "Напишите, что нужно уточнить", []fieldErr{{"comment", "Клиент увидит этот вопрос — сформулируйте, что именно нужно"}})
			return
		}
		ask := func(ctx context.Context, q *db.Queries, app db.Application) error {
			d := appdata.Parse(app.Data)
			d.InfoRequest = &appdata.InfoRequest{Text: comment, RequestedBy: "staff", At: time.Now()}
			return q.SetApplicationData(ctx, db.SetApplicationDataParams{ID: app.ID, Data: d.Marshal()})
		}
		err = s.Pipeline.Apply(r.Context(), a.ID, workflow.AnalystRequestInfo, actor, comment, version, ask)
	case gen.DecisionRequestActionReject:
		if comment == "" {
			writeFieldErrors(w, http.StatusUnprocessableEntity, "Укажите причину отказа", []fieldErr{{"comment", "Клиент увидит комментарий — объясните причину"}})
			return
		}
		reasons := []string{"Заявка не соответствует требованиям банка"}
		if data.Review != nil && len(data.Review.Reasons) > 0 {
			reasons = data.Review.Reasons
		}
		reject := func(ctx context.Context, q *db.Queries, app db.Application) error {
			d := appdata.Parse(app.Data)
			d.Rejection = &appdata.Rejection{Reasons: reasons, By: "staff"}
			return q.SetApplicationData(ctx, db.SetApplicationDataParams{ID: app.ID, Data: d.Marshal()})
		}
		err = s.Pipeline.Apply(r.Context(), a.ID, workflow.AnalystReject, actor, comment, version, reject)
	default:
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Неизвестное действие", []fieldErr{{"action", "Выберите действие из списка"}})
		return
	}
	if err != nil {
		s.applyError(w, r, err)
		return
	}
	s.respondCard(w, r, a.ID)
}

// approvalOffer is the offer made on "approve": the system's recommendation, or the requested
// terms at the product's rate (plus a risk premium when a score exists).
func (s *Server) approvalOffer(ctx context.Context, a db.Application, prod products.Product, data appdata.Data) (decision.Offer, error) {
	if data.Review != nil && data.Review.Recommended != nil {
		o := *data.Review.Recommended
		o.Source = "analyst"
		return o, nil
	}
	rate := prod.BaseRateBP
	if res, err := s.loadScoring(ctx, a.ID); err != nil {
		return decision.Offer{}, err
	} else if res != nil {
		if o, ok := decision.MakeOffer(prod, res.Grade, a.Amount, int(a.TermMonths), "analyst"); ok {
			return o, nil
		}
	}
	return decision.Offer{
		Amount: a.Amount, TermMonths: int(a.TermMonths), RateBP: rate, Source: "analyst",
		MonthlyPayment: scoring.MonthlyPayment(a.Amount, rate, int(a.TermMonths)),
	}, nil
}

// storedDecision is the decision the system made when it scored the application, read back from
// what was stored then: the recommendation, the reasons, the automatic offer. Explaining it must
// not recalculate with today's rules, or the text would contradict the card next to it.
func (s *Server) storedDecision(ctx context.Context, a db.Application, res scoring.Result) (decision.Decision, error) {
	data := appdata.Parse(a.Data)
	switch {
	case data.Rejection != nil && data.Rejection.By == "system":
		return decision.Decision{Outcome: decision.Reject, Reasons: data.Rejection.Reasons}, nil
	case data.Review != nil:
		return decision.Decision{Outcome: decision.Review, Recommended: data.Review.Recommended, Reasons: data.Review.Reasons}, nil
	}
	if o, err := s.Store.GetActiveOffer(ctx, a.ID); err == nil && o.Source == "auto" {
		offer := decision.Offer{Amount: o.Amount, TermMonths: int(o.TermMonths), RateBP: int(o.RateBp), MonthlyPayment: o.MonthlyPayment, Source: o.Source}
		return decision.Decision{Outcome: decision.AutoOffer, Offer: &offer}, nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return decision.Decision{}, err
	}
	// nothing was stored (an old record): the best available is a calculation with the current rules
	prod, _ := products.Get(products.ID(a.ProductID))
	return decision.Decide(res, prod, a.Amount, int(a.TermMonths)), nil
}

func (s *Server) GetExplanation(w http.ResponseWriter, r *http.Request, id gen.ApplicationId, params gen.GetExplanationParams) {
	a, ok := s.staffApp(w, r, id.String())
	if !ok {
		return
	}
	res, err := s.loadScoring(r.Context(), a.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if res == nil {
		writeJSON(w, http.StatusOK, gen.Explanation{
			Text:   "Автоматическая оценка не проводилась: внешние источники данных были недоступны. Проверьте компанию вручную и примите решение.",
			Source: gen.ExplanationSourceRules,
		})
		return
	}
	d, err := s.storedDecision(r.Context(), a, *res)
	if err != nil {
		internalError(w, r, err)
		return
	}
	audience := "staff"
	if params.Audience != nil && *params.Audience == gen.GetExplanationParamsAudienceClient {
		audience = "client"
	}
	text, source := s.Explainer.Explain(r.Context(), *res, d, audience)
	writeJSON(w, http.StatusOK, gen.Explanation{Text: text, Source: gen.ExplanationSource(source)})
}

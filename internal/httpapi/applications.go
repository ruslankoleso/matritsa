package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"matritsa/internal/appdata"
	"matritsa/internal/auth"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/pipeline"
	"matritsa/internal/products"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

// own loads the application and checks that it belongs to the caller; otherwise it answers 404.
func (s *Server) own(w http.ResponseWriter, r *http.Request, id string) (db.Application, auth.Claims, bool) {
	claims, _ := auth.FromContext(r.Context())
	app, err := s.Store.GetApplication(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && app.UserID != claims.UserID) {
		writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена")
		return db.Application{}, claims, false
	}
	if err != nil {
		internalError(w, r, err)
		return db.Application{}, claims, false
	}
	return app, claims, true
}

// applyError maps pipeline errors to HTTP answers.
func (s *Server) applyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, workflow.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_state", "Сейчас это действие недоступно: статус заявки уже изменился")
	case isConflict(err):
		writeError(w, http.StatusConflict, "conflict", "Заявку только что изменили. Обновите страницу и попробуйте снова.")
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена")
	default:
		internalError(w, r, err)
	}
}

func (s *Server) respondApplication(w http.ResponseWriter, r *http.Request, id string, code int) {
	app, err := s.Store.GetApplication(r.Context(), id)
	if err != nil {
		s.applyError(w, r, err)
		return
	}
	out, err := s.loadApplication(r.Context(), app)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, code, out)
}

func (s *Server) ListApplications(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	apps, err := s.Store.ListApplicationsByUser(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	out := make([]gen.ApplicationSummary, len(apps))
	for i, a := range apps {
		out[i] = toSummary(a)
	}
	writeJSON(w, http.StatusOK, out)
}

func productErrors(in gen.ApplicationInput, prod products.Product, amount int64, term int) []fieldErr {
	var fes []fieldErr
	for _, fe := range products.ValidateRequest(prod, amount, term) {
		fes = append(fes, fieldErr{fe.Field, fe.Message})
	}
	return fes
}

func (s *Server) CreateApplication(w http.ResponseWriter, r *http.Request) {
	var in gen.ApplicationInput
	if !decodeBody(w, r, &in) {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	user, err := s.Store.GetUser(r.Context(), claims.UserID)
	if err != nil || user.CompanyID == nil {
		writeError(w, http.StatusUnprocessableEntity, "no_company", "К вашему профилю не привязана компания")
		return
	}
	if in.ProductId == nil {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Выберите кредитный продукт", []fieldErr{{"productId", "Выберите кредитный продукт"}})
		return
	}
	prod, ok := products.Get(products.ID(*in.ProductId))
	if !ok {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Такого продукта нет", []fieldErr{{"productId", "Выберите продукт из списка"}})
		return
	}
	amount, term := prod.MinAmount, prod.MinTerm
	if in.Amount != nil {
		amount = *in.Amount
	}
	if in.TermMonths != nil {
		term = *in.TermMonths
	}
	if fes := productErrors(in, prod, amount, term); len(fes) > 0 {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Проверьте параметры заявки", fes)
		return
	}
	data := appdata.Data{}
	if in.Fields != nil {
		data.Fields = *in.Fields
	}
	purpose := ""
	if in.Purpose != nil {
		purpose = strings.TrimSpace(*in.Purpose)
	}
	app, err := s.Store.CreateApplication(r.Context(), db.CreateApplicationParams{
		CompanyID: *user.CompanyID, UserID: user.ID, ProductID: string(prod.ID), Amount: amount, TermMonths: int32(term), Purpose: purpose, Data: data.Marshal(),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	s.respondApplication(w, r, app.ID, http.StatusCreated)
}

func (s *Server) GetApplication(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	if _, _, ok := s.own(w, r, id.String()); !ok {
		return
	}
	s.respondApplication(w, r, id.String(), http.StatusOK)
}

func (s *Server) UpdateApplication(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	app, _, ok := s.own(w, r, id.String())
	if !ok {
		return
	}
	var in gen.ApplicationInput
	if !decodeBody(w, r, &in) {
		return
	}
	if st := workflow.Status(app.Status); st != workflow.Draft && st != workflow.NeedsInfo {
		writeError(w, http.StatusConflict, "invalid_state", "Заявку уже нельзя менять")
		return
	}
	productID, amount, term, purpose := app.ProductID, app.Amount, int(app.TermMonths), app.Purpose
	if in.ProductId != nil {
		productID = *in.ProductId
	}
	if in.Amount != nil {
		amount = *in.Amount
	}
	if in.TermMonths != nil {
		term = *in.TermMonths
	}
	if in.Purpose != nil {
		purpose = strings.TrimSpace(*in.Purpose)
	}
	prod, ok := products.Get(products.ID(productID))
	if !ok {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Такого продукта нет", []fieldErr{{"productId", "Выберите продукт из списка"}})
		return
	}
	if fes := productErrors(in, prod, amount, term); len(fes) > 0 {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Проверьте параметры заявки", fes)
		return
	}
	data := appdata.Parse(app.Data)
	if in.Fields != nil {
		if data.Fields == nil {
			data.Fields = map[string]string{}
		}
		for k, v := range *in.Fields {
			data.Fields[k] = v
		}
	}
	_, err := s.Store.UpdateApplicationDraft(r.Context(), db.UpdateApplicationDraftParams{
		ID: app.ID, ProductID: string(prod.ID), Amount: amount, TermMonths: int32(term), Purpose: purpose, Data: data.Marshal(),
	})
	if err != nil {
		s.applyError(w, r, err)
		return
	}
	s.respondApplication(w, r, app.ID, http.StatusOK)
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func (s *Server) SubmitApplication(w http.ResponseWriter, r *http.Request, id gen.ApplicationId, params gen.SubmitApplicationParams) {
	app, claims, ok := s.own(w, r, id.String())
	if !ok {
		return
	}
	key := strings.TrimSpace(params.IdempotencyKey)
	if len(key) < 8 || len(key) > 80 {
		writeError(w, http.StatusBadRequest, "bad_request", "Idempotency-Key должен быть длиной от 8 до 80 символов")
		return
	}
	if app.IdempotencyKey.Valid && app.IdempotencyKey.String == key {
		s.respondApplication(w, r, app.ID, http.StatusOK) // repeated request: same answer
		return
	}
	if workflow.Status(app.Status) != workflow.Draft {
		writeError(w, http.StatusConflict, "invalid_state", "Заявка уже отправлена")
		return
	}
	missing, err := s.Pipeline.Missing(r.Context(), app)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if len(missing) > 0 {
		fes := make([]fieldErr, len(missing))
		for i, m := range missing {
			fes[i] = fieldErr{"application", m}
		}
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Чтобы отправить заявку, нужно дополнить её", fes)
		return
	}
	setKey := func(ctx context.Context, q *db.Queries, a db.Application) error {
		return q.SetIdempotencyKey(ctx, db.SetIdempotencyKeyParams{ID: a.ID, IdempotencyKey: pgtype.Text{String: key, Valid: true}})
	}
	err = s.Pipeline.Apply(r.Context(), app.ID, workflow.Submit, "client:"+claims.UserID, "Заявка отправлена клиентом", app.Version, setKey)
	if err != nil {
		// A concurrent request with the same key may have won the race.
		if cur, gerr := s.Store.GetApplication(r.Context(), app.ID); gerr == nil && cur.IdempotencyKey.Valid && cur.IdempotencyKey.String == key {
			s.respondApplication(w, r, app.ID, http.StatusOK)
			return
		}
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "duplicate_key", "Этот ключ запроса уже использован")
			return
		}
		s.applyError(w, r, err)
		return
	}
	_ = s.Pipeline.Start(r.Context(), app.ID)
	s.respondApplication(w, r, app.ID, http.StatusOK)
}

func (s *Server) RespondToInfoRequest(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	app, claims, ok := s.own(w, r, id.String())
	if !ok {
		return
	}
	var in gen.InfoResponse
	if !decodeBody(w, r, &in) {
		return
	}
	if workflow.Status(app.Status) != workflow.NeedsInfo {
		writeError(w, http.StatusConflict, "invalid_state", "Сейчас от вас ничего не требуется")
		return
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Напишите ответ", []fieldErr{{"text", "Напишите пару слов в ответ на вопрос банка"}})
		return
	}
	data := appdata.Parse(app.Data)
	event := workflow.ClientProvidedInfo
	if data.InfoRequest != nil && data.InfoRequest.RequestedBy == "staff" {
		event = workflow.ClientAnswered
	}
	if event == workflow.ClientProvidedInfo {
		missing, err := s.Pipeline.Missing(r.Context(), app)
		if err != nil {
			internalError(w, r, err)
			return
		}
		if len(missing) > 0 {
			fes := make([]fieldErr, len(missing))
			for i, m := range missing {
				fes[i] = fieldErr{"application", m}
			}
			writeFieldErrors(w, http.StatusUnprocessableEntity, "Пока не всё готово", fes)
			return
		}
	}
	addAnswer := func(ctx context.Context, q *db.Queries, a db.Application) error {
		d := appdata.Parse(a.Data)
		d.InfoAnswers = append(d.InfoAnswers, appdata.InfoAnswer{Text: text, At: time.Now()})
		return q.SetApplicationData(ctx, db.SetApplicationDataParams{ID: a.ID, Data: d.Marshal()})
	}
	if err := s.Pipeline.Apply(r.Context(), app.ID, event, "client:"+claims.UserID, text, app.Version, addAnswer); err != nil {
		s.applyError(w, r, err)
		return
	}
	_ = s.Pipeline.Start(r.Context(), app.ID)
	s.respondApplication(w, r, app.ID, http.StatusOK)
}

func (s *Server) AcceptOffer(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	app, claims, ok := s.own(w, r, id.String())
	if !ok {
		return
	}
	if workflow.Status(app.Status) == workflow.OfferMade {
		if o, err := s.Store.GetActiveOffer(r.Context(), app.ID); err == nil && o.ExpiresAt.Time.Before(time.Now()) {
			_ = s.Pipeline.ExpireOffers(r.Context())
			writeError(w, http.StatusConflict, "offer_expired", "Срок действия предложения истёк. Можно подать новую заявку.")
			return
		}
	}
	actor := "client:" + claims.UserID
	if err := s.Pipeline.Apply(r.Context(), app.ID, workflow.ClientAccept, actor, "Клиент принял предложение", app.Version); err != nil {
		s.applyError(w, r, err)
		return
	}
	// Signing and disbursement are simulated: no real documents or money move.
	if err := s.Pipeline.Apply(r.Context(), app.ID, workflow.Disburse, "system", "Документы подписаны, деньги отправлены (демо — реальных операций нет)", pipeline.AnyVersion); err != nil {
		s.applyError(w, r, err)
		return
	}
	s.respondApplication(w, r, app.ID, http.StatusOK)
}

func (s *Server) simpleTransition(w http.ResponseWriter, r *http.Request, id gen.ApplicationId, ev workflow.Event, comment string) {
	app, claims, ok := s.own(w, r, id.String())
	if !ok {
		return
	}
	if err := s.Pipeline.Apply(r.Context(), app.ID, ev, "client:"+claims.UserID, comment, app.Version); err != nil {
		s.applyError(w, r, err)
		return
	}
	s.respondApplication(w, r, app.ID, http.StatusOK)
}

func (s *Server) DeclineOffer(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	s.simpleTransition(w, r, id, workflow.ClientDecline, "Клиент отказался от предложения")
}

func (s *Server) CancelApplication(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	s.simpleTransition(w, r, id, workflow.Cancel, "Клиент отозвал заявку")
}

func (s *Server) GetTimeline(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	if _, _, ok := s.own(w, r, id.String()); !ok {
		return
	}
	evs, err := s.Store.ListEvents(r.Context(), id.String())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toTimeline(evs))
}

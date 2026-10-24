package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"matritsa/internal/auth"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/integrations"
	"matritsa/internal/seed"
	"matritsa/internal/store"
	"matritsa/internal/store/db"
)

var loginPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

var registrationExamples = sync.OnceValue(seed.ExampleINNs)

// checkSignup validates the sign-up form and returns the cleaned login.
func checkSignup(req gen.RegisterRequest) (login string, errs []fieldErr) {
	login = strings.ToLower(strings.TrimSpace(req.Login))
	if !loginPattern.MatchString(login) {
		errs = append(errs, fieldErr{"login", "Логин — от 3 до 32 символов: латинские буквы, цифры, точка, дефис, подчёркивание"})
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(req.FullName)); n < 2 || n > 100 {
		errs = append(errs, fieldErr{"fullName", "Укажите ФИО, до 100 символов"})
	}
	// bcrypt only uses the first 72 bytes; a longer password would be silently cut.
	if utf8.RuneCountInString(req.Password) < 8 || len(req.Password) > 72 {
		errs = append(errs, fieldErr{"password", "Пароль — от 8 символов, не длиннее 72 байт"})
	}
	if !validINN(strings.TrimSpace(req.Inn)) {
		errs = append(errs, fieldErr{"inn", "ИНН состоит из 10 цифр (для организаций) или 12 цифр (для ИП)"})
	}
	if !req.Consent {
		errs = append(errs, fieldErr{"consent", "Нужно согласие на обработку данных"})
	}
	return login, errs
}

func isUniqueOn(err error, constraint string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == constraint
}

// Register creates a client account and the company behind it, then signs the person in.
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	key := clientIP(r)
	if !s.RegisterLimiter.Allowed(key) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "Слишком много регистраций с вашего адреса. Попробуйте позже.")
		return
	}
	s.RegisterLimiter.Fail(key) // every attempt counts, not only failed ones
	var req gen.RegisterRequest
	if !decodeBody(w, r, &req) {
		return
	}
	login, errs := checkSignup(req)
	if len(errs) > 0 {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Проверьте форму", errs)
		return
	}
	inn := strings.TrimSpace(req.Inn)
	if !s.innIsFree(w, r, inn) {
		return
	}

	// The registry (simulated) must know the company; its data is what the account is tied to.
	if _, err := s.Lookup.EGRUL(r.Context(), inn); errors.Is(err, integrations.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Компания с таким ИНН не найдена в реестре")
		return
	} else if err != nil {
		writeError(w, http.StatusServiceUnavailable, "registry_unavailable", "Реестр временно недоступен. Попробуйте чуть позже.")
		return
	}
	co, ok := seed.Synthesize(inn)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Компания с таким ИНН не найдена в реестре")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internalError(w, r, err)
		return
	}

	var user db.User
	err = s.Store.WithTx(r.Context(), func(q *db.Queries) error {
		company, err := q.InsertCompany(r.Context(), db.InsertCompanyParams{
			Inn: inn, Name: co.Name, Kind: co.Kind, RegisteredAt: pgtype.Date{Time: co.RegisteredAt, Valid: true},
			Okved: co.OKVED, OkvedGroup: int32(co.OKVEDGroup), Director: co.Director, Address: co.Address, MonthlyObligations: co.MonthlyObligations,
		})
		if err != nil {
			return err
		}
		if err := store.SaveFinancials(r.Context(), q, company.ID, co.Revenue); err != nil {
			return err
		}
		coID := company.ID
		user, err = q.InsertClient(r.Context(), db.InsertClientParams{
			Login: login, PasswordHash: hash, CompanyID: &coID, FullName: strings.TrimSpace(req.FullName),
		})
		return err
	})
	switch {
	case isUniqueOn(err, "companies_inn_key"):
		writeError(w, http.StatusConflict, "company_registered", "Эта компания уже зарегистрирована. Войдите под её аккаунтом.")
		return
	case isUniqueOn(err, "users_login_key"):
		writeFieldErrors(w, http.StatusConflict, "Логин занят", []fieldErr{{"login", "Такой логин уже есть, придумайте другой"}})
		return
	case err != nil:
		internalError(w, r, err)
		return
	}
	s.issueTokensWithStatus(w, r, user, http.StatusCreated)
}

// innIsFree answers 409 for a company that already has an account and 422 for a wrong check digit.
// The existing company goes first: the demo companies have invented INNs that fail the checksum,
// and "already registered" is the honest answer for them.
func (s *Server) innIsFree(w http.ResponseWriter, r *http.Request, inn string) bool {
	if _, err := s.Store.GetCompanyByINN(r.Context(), inn); err == nil {
		writeError(w, http.StatusConflict, "company_registered", "Эта компания уже зарегистрирована. Войдите под её аккаунтом.")
		return false
	}
	if !seed.ValidINN(inn) {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Проверьте ИНН", []fieldErr{{"inn", "Контрольная сумма ИНН не сходится. Можно взять тестовый ИНН из подсказки"}})
		return false
	}
	return true
}

// LookupCompanyForSignup shows the company behind an INN before the account is created.
func (s *Server) LookupCompanyForSignup(w http.ResponseWriter, r *http.Request, params gen.LookupCompanyForSignupParams) {
	inn := strings.TrimSpace(params.Inn)
	if !validINN(inn) {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Проверьте ИНН", []fieldErr{{"inn", "ИНН состоит из 10 цифр (для организаций) или 12 цифр (для ИП)"}})
		return
	}
	if !s.innIsFree(w, r, inn) {
		return
	}
	s.writeCompany(w, r, inn)
}

func (s *Server) RegistrationExamples(w http.ResponseWriter, _ *http.Request) {
	ex := registrationExamples()
	out := make([]gen.RegistrationExample, 0, len(ex))
	for _, e := range ex {
		out = append(out, gen.RegistrationExample{Inn: e.INN, Name: e.Name, Note: e.Note})
	}
	writeJSON(w, http.StatusOK, out)
}

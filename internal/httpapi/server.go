// Package httpapi implements the OpenAPI contract (api/openapi.yaml).
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"matritsa/internal/ai"
	"matritsa/internal/auth"
	"matritsa/internal/events"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/pipeline"
	"matritsa/internal/store"
)

// Deps are the collaborators of the HTTP layer.
type Deps struct {
	Store     *store.Store
	Pipeline  *pipeline.Pipeline
	Hub       *events.Hub
	Issuer    *auth.Issuer
	Lookup    pipeline.Sources // EGRUL lookup for the wizard
	Assistant *ai.Assistant
	Chat      *ai.Chat
	// ChatLimiter counts every question a person asks the helper.
	ChatLimiter  *auth.Limiter
	Explainer    *ai.Explainer
	DataDir      string
	LoginLimiter *auth.Limiter
	// RegisterLimiter counts sign-up attempts per address, successful ones included.
	RegisterLimiter *auth.Limiter
	Heartbeat       time.Duration
	OfferTTL        time.Duration
}

type Server struct {
	Deps
}

func New(d Deps) *Server {
	if d.Heartbeat == 0 {
		d.Heartbeat = 20 * time.Second
	}
	if d.OfferTTL == 0 {
		d.OfferTTL = 7 * 24 * time.Hour
	}
	if d.LoginLimiter == nil {
		d.LoginLimiter = auth.NewLimiter(5, time.Minute)
	}
	if d.Chat == nil {
		d.Chat = ai.NewChat(nil)
	}
	if d.ChatLimiter == nil {
		d.ChatLimiter = auth.NewLimiter(30, 10*time.Minute)
	}
	if d.RegisterLimiter == nil {
		d.RegisterLimiter = auth.NewLimiter(10, time.Hour)
	}
	return &Server{Deps: d}
}

// Handler assembles probes and the generated API router behind auth and role guards.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, requestLog, timeoutExcept(requestTimeout, "/api/v1/events"))
	r.Method("GET", "/healthz", Health(s.Store.Pool))
	r.Method("GET", "/readyz", Ready(s.Store.Pool))

	gen.HandlerWithOptions(s, gen.ChiServerOptions{
		BaseURL:     "/api/v1",
		BaseRouter:  r,
		Middlewares: []gen.MiddlewareFunc{s.guard},
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, "bad_request", "Не удалось разобрать запрос: "+err.Error())
		},
	})
	return r
}

// requestTimeout bounds an ordinary request (an upload of 5 MB on a slow link still fits).
const requestTimeout = time.Minute

// timeoutExcept gives every request a deadline, except the listed long-lived paths.
func timeoutExcept(d time.Duration, paths ...string) func(http.Handler) http.Handler {
	limit := middleware.Timeout(d)
	return func(next http.Handler) http.Handler {
		limited := limit(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range paths {
				if r.URL.Path == p {
					next.ServeHTTP(w, r)
					return
				}
			}
			limited.ServeHTTP(w, r)
		})
	}
}

var publicPaths = map[string]bool{
	"/api/v1/auth/login": true, "/api/v1/auth/refresh": true, "/api/v1/products": true,
	"/api/v1/auth/register": true, "/api/v1/auth/company": true, "/api/v1/auth/registration-examples": true,
}

// bothRoles are reachable by clients and bank staff alike.
var bothRoles = map[string]bool{"/api/v1/me": true, "/api/v1/events": true}

// guard authenticates and applies role rules by path.
func (s *Server) guard(next http.Handler) http.Handler {
	authed := auth.Authenticate(s.Issuer)
	authedQuery := auth.AuthenticateWithQuery(s.Issuer)
	staffOnly := auth.RequireRole("staff")
	supportOnly := auth.RequireRole("support")
	clientOnly := auth.RequireRole("client")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case publicPaths[p]:
			next.ServeHTTP(w, r)
		case p == "/api/v1/events":
			authedQuery(next).ServeHTTP(w, r)
		case bothRoles[p]:
			authed(next).ServeHTTP(w, r)
		case strings.HasPrefix(p, "/api/v1/staff/"):
			authed(staffOnly(next)).ServeHTTP(w, r)
		case strings.HasPrefix(p, "/api/v1/support/"):
			authed(supportOnly(next)).ServeHTTP(w, r)
		default:
			authed(clientOnly(next)).ServeHTTP(w, r)
		}
	})
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "ms", time.Since(start).Milliseconds(), "id", middleware.GetReqID(r.Context()))
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, gen.Error{Code: errCode, Message: msg})
}

type fieldErr struct{ Field, Message string }

func writeFieldErrors(w http.ResponseWriter, code int, msg string, fes []fieldErr) {
	fields := make([]struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}, len(fes))
	for i, f := range fes {
		fields[i].Field, fields[i].Message = f.Field, f.Message
	}
	writeJSON(w, code, gen.Error{Code: "validation_failed", Message: msg, Fields: &fields})
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Не удалось прочитать запрос")
		return false
	}
	return true
}

func internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("internal error", "path", r.URL.Path, "err", err, "id", middleware.GetReqID(r.Context()))
	writeError(w, http.StatusInternalServerError, "internal", "Произошла ошибка. Повторите попытку позже.")
}

// errConflict maps pipeline/workflow conflicts to HTTP 409.
func isConflict(err error) bool {
	return errors.Is(err, pipeline.ErrConflict)
}

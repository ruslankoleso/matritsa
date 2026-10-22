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
	Store        *store.Store
	Pipeline     *pipeline.Pipeline
	Hub          *events.Hub
	Issuer       *auth.Issuer
	Lookup       pipeline.Sources // EGRUL lookup for the wizard
	Assistant    *ai.Assistant
	Explainer    *ai.Explainer
	DataDir      string
	LoginLimiter *auth.Limiter
	Heartbeat    time.Duration
	OfferTTL     time.Duration
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
	return &Server{Deps: d}
}

// Handler assembles probes and the generated API router behind auth and role guards.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, requestLog)
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

var publicPaths = map[string]bool{"/api/v1/auth/login": true, "/api/v1/auth/refresh": true, "/api/v1/products": true}

// bothRoles are reachable by clients and bank staff alike.
var bothRoles = map[string]bool{"/api/v1/me": true, "/api/v1/events": true}

// guard authenticates and applies role rules by path.
func (s *Server) guard(next http.Handler) http.Handler {
	authed := auth.Authenticate(s.Issuer)
	authedQuery := auth.AuthenticateWithQuery(s.Issuer)
	staffOnly := auth.RequireRole("staff")
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
	writeError(w, http.StatusInternalServerError, "internal", "Что-то пошло не так. Попробуйте ещё раз чуть позже.")
}

// errConflict maps pipeline/workflow conflicts to HTTP 409.
func isConflict(err error) bool {
	return errors.Is(err, pipeline.ErrConflict)
}

package httpapi

import (
	"net"
	"net/http"
	"strings"

	"matritsa/internal/auth"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/store/db"
)

// dummyHash makes unknown logins cost the same as wrong passwords (no user enumeration by timing).
var dummyHash, _ = auth.HashPassword("not-a-real-password")

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req gen.LoginRequest
	if !decodeBody(w, r, &req) {
		return
	}
	login := strings.ToLower(strings.TrimSpace(req.Login))
	// Attempts are counted per address and login, so a stranger guessing passwords
	// blocks only themselves and cannot lock a demo account for everybody else.
	key := clientIP(r) + "|" + login
	if !s.LoginLimiter.Allowed(key) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "Слишком много попыток входа. Подождите минуту и попробуйте снова.")
		return
	}
	user, err := s.Store.GetUserByLogin(r.Context(), login)
	hash := dummyHash
	if err == nil {
		hash = user.PasswordHash
	}
	if !auth.CheckPassword(hash, req.Password) || err != nil {
		s.LoginLimiter.Fail(key)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Неверный логин или пароль")
		return
	}
	s.LoginLimiter.Reset(key)
	s.issueTokens(w, r, user)
}

// clientIP is the address the request came from: nginx sets X-Forwarded-For to the real peer.
func clientIP(r *http.Request) string {
	if first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-For"), ","); strings.TrimSpace(first) != "" {
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req gen.RefreshRequest
	if !decodeBody(w, r, &req) {
		return
	}
	claims, err := s.Issuer.Parse(req.RefreshToken, auth.TypeRefresh)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "Сессия истекла, войдите снова")
		return
	}
	user, err := s.Store.GetUser(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "Сессия истекла, войдите снова")
		return
	}
	s.issueTokens(w, r, user)
}

func (s *Server) issueTokens(w http.ResponseWriter, r *http.Request, user db.User) {
	access, refresh, err := s.Issuer.Issue(user.ID, user.Role)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.TokenPair{AccessToken: access, RefreshToken: refresh, Role: gen.TokenPairRole(user.Role), FullName: user.FullName})
}

func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	user, err := s.Store.GetUser(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "Сессия истекла, войдите снова")
		return
	}
	me := gen.Me{Login: user.Login, FullName: user.FullName, Role: gen.MeRole(user.Role)}
	_ = me.Id.UnmarshalText([]byte(user.ID))
	if user.CompanyID != nil {
		co, err := s.Store.GetCompany(r.Context(), *user.CompanyID)
		if err == nil {
			c := toCompany(co)
			me.Company = &c
		}
	}
	writeJSON(w, http.StatusOK, me)
}

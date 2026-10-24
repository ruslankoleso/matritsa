// Package auth provides password hashing, JWT tokens, role checks and a login rate limiter.
// Sign-in is login+password only: the case forbids SMS/email in the user flow.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"

	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
)

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	return string(h), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

type Claims struct {
	UserID string
	Role   string
}

type tokenClaims struct {
	Role string `json:"role"`
	Type string `json:"typ"`
	jwt.RegisteredClaims
}

type Issuer struct {
	secret []byte
	Now    func() time.Time
}

func NewIssuer(secret string) *Issuer { return &Issuer{secret: []byte(secret), Now: time.Now} }

func (i *Issuer) sign(userID, role, typ string, ttl time.Duration) (string, error) {
	now := i.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		Role: role, Type: typ,
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl))},
	}).SignedString(i.secret)
}

// Issue returns an access and a refresh token.
func (i *Issuer) Issue(userID, role string) (access, refresh string, err error) {
	if access, err = i.sign(userID, role, TypeAccess, AccessTTL); err != nil {
		return
	}
	refresh, err = i.sign(userID, role, TypeRefresh, RefreshTTL)
	return
}

// Parse validates the signature, expiry and token type.
func (i *Issuer) Parse(token, wantType string) (Claims, error) {
	var tc tokenClaims
	_, err := jwt.ParseWithClaims(token, &tc, func(*jwt.Token) (any, error) { return i.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(i.Now))
	if err != nil {
		return Claims{}, err
	}
	if tc.Type != wantType {
		return Claims{}, fmt.Errorf("wrong token type %q", tc.Type)
	}
	return Claims{UserID: tc.Subject, Role: tc.Role}, nil
}

type ctxKey struct{}

func NewContext(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

func deny(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = fmt.Fprintf(w, `{"code":%q,"message":%q}`, http.StatusText(code), msg)
}

func authenticate(iss *Issuer, allowQuery bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tok == r.Header.Get("Authorization") {
				tok = ""
			}
			if tok == "" && allowQuery {
				tok = r.URL.Query().Get("access_token")
			}
			if tok == "" {
				deny(w, http.StatusUnauthorized, "Войдите, чтобы продолжить")
				return
			}
			c, err := iss.Parse(tok, TypeAccess)
			if err != nil {
				deny(w, http.StatusUnauthorized, "Сессия истекла, войдите снова")
				return
			}
			next.ServeHTTP(w, r.WithContext(NewContext(r.Context(), c)))
		})
	}
}

// Authenticate requires a valid Bearer access token.
func Authenticate(iss *Issuer) func(http.Handler) http.Handler { return authenticate(iss, false) }

// AuthenticateWithQuery additionally accepts ?access_token= (EventSource cannot set headers).
func AuthenticateWithQuery(iss *Issuer) func(http.Handler) http.Handler {
	return authenticate(iss, true)
}

// RequireRole lets only the listed roles through; Authenticate must run first.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := FromContext(r.Context())
			if !ok {
				deny(w, http.StatusUnauthorized, "Войдите, чтобы продолжить")
				return
			}
			for _, role := range roles {
				if c.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			deny(w, http.StatusForbidden, "Этот раздел недоступен для вашей роли")
		})
	}
}

// Limiter blocks a key (the caller picks it: here sign-in address + login) after too many recent failures. Only failures count: successful
// sign-ins never consume attempts, so people can switch between accounts freely.
type Limiter struct {
	max    int
	window time.Duration
	Now    func() time.Time
	mu     sync.Mutex
	fails  map[string][]time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, Now: time.Now, fails: map[string][]time.Time{}}
}

func (l *Limiter) recent(key string) []time.Time {
	now := l.Now()
	kept := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	l.fails[key] = kept
	return kept
}

// Allowed reports whether the key may try to sign in now.
func (l *Limiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) < l.max
}

// sweepAbove is the number of tracked keys after which expired ones are dropped,
// so guessing with endless random logins cannot grow the map without bound.
const sweepAbove = 4096

// Fail records a failed attempt.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.fails) > sweepAbove {
		l.sweep()
	}
	l.fails[key] = append(l.recent(key), l.Now())
}

// sweep forgets every key whose failures all fell out of the window. The caller holds mu.
func (l *Limiter) sweep() {
	for key := range l.fails {
		if len(l.recent(key)) == 0 {
			delete(l.fails, key)
		}
	}
}

// Reset forgets earlier failures (after a successful sign-in).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

var ErrInvalidCredentials = errors.New("invalid credentials")

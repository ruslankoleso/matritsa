package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealthz_AlwaysOK(t *testing.T) {
	rec := httptest.NewRecorder()
	Health(fakePinger{err: errors.New("down")}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rec.Code)
	}
}

func TestReadyz_DBDown_503(t *testing.T) {
	h := Ready(fakePinger{err: errors.New("down")})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz with db down = %d, want 503", rec.Code)
	}
	rec = httptest.NewRecorder()
	Ready(fakePinger{}).ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz with db up = %d, want 200", rec.Code)
	}
}

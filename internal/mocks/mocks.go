// Package mocks implements the simulated external systems: a credit bureau (БКИ)
// and the state registry (ЕГРЮЛ/ФНС). Responses are deterministic per INN.
package mocks

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"matritsa/internal/integrations"
	"matritsa/internal/seed"
)

// FailSwitch lets a demo make the mock "go down" at runtime (POST /mock/fail?on=true).
type FailSwitch struct{ on atomic.Bool }

func (f *FailSwitch) Set(v bool) { f.on.Store(v) }
func (f *FailSwitch) On() bool   { return f != nil && f.on.Load() }

type Options struct {
	Latency time.Duration
	Fail    *FailSwitch
}

func (o Options) gate(w http.ResponseWriter, r *http.Request) bool {
	if o.Latency > 0 {
		select {
		case <-time.After(o.Latency):
		case <-r.Context().Done():
			return false
		}
	}
	if r.Header.Get("X-Mock-Fail") != "" || o.Fail.On() {
		http.Error(w, `{"error":"service temporarily unavailable"}`, http.StatusServiceUnavailable)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func admin(mux *http.ServeMux, o Options) {
	mux.HandleFunc("POST /mock/fail", func(w http.ResponseWriter, r *http.Request) {
		if o.Fail != nil {
			o.Fail.Set(r.URL.Query().Get("on") == "true")
		}
		writeJSON(w, map[string]bool{"fail": o.Fail.On()})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]string{"status": "ok"}) })
}

// BKI serves GET /bki/{inn}. Unknown INNs have no credit history (200, hasHistory=false).
func BKI(o Options) http.Handler {
	mux := http.NewServeMux()
	admin(mux, o)
	mux.HandleFunc("GET /bki/{inn}", func(w http.ResponseWriter, r *http.Request) {
		if !o.gate(w, r) {
			return
		}
		if c, ok := seed.ByINN(r.PathValue("inn")); ok && c.BKI != nil {
			writeJSON(w, c.BKI)
			return
		}
		writeJSON(w, integrations.BKIReport{HasHistory: false})
	})
	return mux
}

// EGRUL serves GET /egrul/{inn}. Unknown INNs return 404.
func EGRUL(o Options) http.Handler {
	mux := http.NewServeMux()
	admin(mux, o)
	mux.HandleFunc("GET /egrul/{inn}", func(w http.ResponseWriter, r *http.Request) {
		if !o.gate(w, r) {
			return
		}
		inn := strings.TrimSpace(r.PathValue("inn"))
		if c, ok := seed.ByINN(inn); ok {
			writeJSON(w, c.EGRUL())
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})
	return mux
}

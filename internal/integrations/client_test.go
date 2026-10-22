package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastClient(bki, egrul string) *Client {
	c := NewClient(bki, egrul)
	c.Timeout = 150 * time.Millisecond
	c.Backoff = 5 * time.Millisecond
	return c
}

func TestClient_RetriesThenErrUnavailable(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := fastClient(srv.URL, srv.URL).BKI(context.Background(), "7701234501")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("want exactly 3 attempts, got %d", got)
	}
}

func TestClient_RecoversAfterTransientFailure(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(BKIReport{HasHistory: true, Score: 700})
	}))
	defer srv.Close()
	rep, err := fastClient(srv.URL, srv.URL).BKI(context.Background(), "7701234501")
	if err != nil || rep == nil || rep.Score != 700 {
		t.Fatalf("want success on 3rd attempt, got %+v %v", rep, err)
	}
}

func TestClient_TimeoutRespected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	start := time.Now()
	_, err := fastClient(srv.URL, srv.URL).EGRUL(context.Background(), "7701234501")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable on timeout, got %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("timeouts not respected: took %v", d)
	}
}

func TestClient_EGRUL_NotFoundIsNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := fastClient(srv.URL, srv.URL).EGRUL(context.Background(), "0000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("404 must not be retried, calls=%d", calls)
	}
}

func TestClient_ContextCancelStopsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fastClient(srv.URL, srv.URL).BKI(ctx, "1"); err == nil {
		t.Fatal("cancelled context must fail")
	}
}

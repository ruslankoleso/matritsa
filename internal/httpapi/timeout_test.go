package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeout_AppliesExceptToEventStream(t *testing.T) {
	var hasDeadline bool
	h := timeoutExcept(time.Minute, "/api/v1/events")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/applications", nil))
	if !hasDeadline {
		t.Fatal("ordinary requests must have a deadline")
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/events", nil))
	if hasDeadline {
		t.Fatal("the long-lived event stream must not be cut by the request timeout")
	}
}

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPassword_HashAndCheck(t *testing.T) {
	h, err := HashPassword("demo1234")
	if err != nil || h == "demo1234" {
		t.Fatalf("hash failed: %v", err)
	}
	if !CheckPassword(h, "demo1234") || CheckPassword(h, "wrong") || CheckPassword("garbage", "demo1234") {
		t.Fatal("password check wrong")
	}
}

func TestIssuer_IssueAndParse(t *testing.T) {
	iss := NewIssuer("secret-secret-secret-secret-1234")
	access, refresh, err := iss.Issue("u1", "client")
	if err != nil {
		t.Fatal(err)
	}
	c, err := iss.Parse(access, TypeAccess)
	if err != nil || c.UserID != "u1" || c.Role != "client" {
		t.Fatalf("parse access: %+v %v", c, err)
	}
	if _, err := iss.Parse(refresh, TypeAccess); err == nil {
		t.Fatal("refresh token must not be accepted as access")
	}
	if _, err := iss.Parse(access, TypeRefresh); err == nil {
		t.Fatal("access token must not be accepted as refresh")
	}
}

func TestExpiredToken_Rejected(t *testing.T) {
	iss := NewIssuer("secret-secret-secret-secret-1234")
	now := time.Now()
	iss.Now = func() time.Time { return now.Add(-time.Hour) }
	access, _, _ := iss.Issue("u1", "client")
	iss.Now = func() time.Time { return now }
	if _, err := iss.Parse(access, TypeAccess); err == nil {
		t.Fatal("15-minute token issued an hour ago must be expired")
	}
}

func TestWrongSecret_Rejected(t *testing.T) {
	a := NewIssuer("secret-secret-secret-secret-1234")
	b := NewIssuer("another-another-another-another-9")
	tok, _, _ := a.Issue("u1", "staff")
	if _, err := b.Parse(tok, TypeAccess); err == nil {
		t.Fatal("token signed with another secret must be rejected")
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := FromContext(r.Context())
		_, _ = w.Write([]byte(c.UserID))
	})
}

func do(h http.Handler, token, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestProtectedRoute_NoToken_401(t *testing.T) {
	iss := NewIssuer("secret-secret-secret-secret-1234")
	if rec := do(Authenticate(iss)(okHandler()), "", "/x"); rec.Code != 401 {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if rec := do(Authenticate(iss)(okHandler()), "garbage", "/x"); rec.Code != 401 {
		t.Fatalf("want 401 for garbage, got %d", rec.Code)
	}
}

func TestStaffRoute_ClientToken_403(t *testing.T) {
	iss := NewIssuer("secret-secret-secret-secret-1234")
	clientTok, _, _ := iss.Issue("c1", "client")
	staffTok, _, _ := iss.Issue("s1", "staff")
	h := Authenticate(iss)(RequireRole("staff")(okHandler()))
	if rec := do(h, clientTok, "/staff/queue"); rec.Code != 403 {
		t.Fatalf("client on staff route: want 403, got %d", rec.Code)
	}
	if rec := do(h, staffTok, "/staff/queue"); rec.Code != 200 || rec.Body.String() != "s1" {
		t.Fatalf("staff on staff route: %d %s", rec.Code, rec.Body)
	}
}

func TestQueryToken_OnlyWhenAllowed(t *testing.T) {
	iss := NewIssuer("secret-secret-secret-secret-1234")
	tok, _, _ := iss.Issue("c1", "client")
	if rec := do(Authenticate(iss)(okHandler()), "", "/events?access_token="+tok); rec.Code != 401 {
		t.Fatalf("query token must be refused by default, got %d", rec.Code)
	}
	if rec := do(AuthenticateWithQuery(iss)(okHandler()), "", "/events?access_token="+tok); rec.Code != 200 {
		t.Fatalf("query token must work for SSE route, got %d", rec.Code)
	}
}

func TestLimiter_BlocksAfterFiveFailuresPerMinute(t *testing.T) {
	now := time.Now()
	l := NewLimiter(5, time.Minute)
	l.Now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		if !l.Allowed("client1") {
			t.Fatalf("attempt %d must be allowed", i+1)
		}
		l.Fail("client1")
	}
	if l.Allowed("client1") {
		t.Fatal("after 5 failures the 6th attempt must be blocked")
	}
	if !l.Allowed("client2") {
		t.Fatal("other key unaffected")
	}
	now = now.Add(61 * time.Second)
	if !l.Allowed("client1") {
		t.Fatal("window must reset after a minute")
	}
}

func TestLimiter_AllowedAloneDoesNotConsumeAttempts(t *testing.T) {
	l := NewLimiter(2, time.Minute)
	for i := 0; i < 50; i++ {
		if !l.Allowed("k") {
			t.Fatal("successful logins must never be blocked")
		}
	}
}

func TestLimiter_ResetClearsFailures(t *testing.T) {
	l := NewLimiter(2, time.Minute)
	l.Fail("k")
	l.Fail("k")
	if l.Allowed("k") {
		t.Fatal("expected a block")
	}
	l.Reset("k")
	if !l.Allowed("k") {
		t.Fatal("a successful login clears earlier failures")
	}
}

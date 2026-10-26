package httpapi_test

import (
	"context"
	"testing"

	"matritsa/internal/auth"
	"matritsa/internal/httpapi"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
	"matritsa/internal/seed"
)

// askedByStaff brings client2's application to NEEDS_INFO with a question from the analyst.
func askedByStaff(t *testing.T, e *testEnv) gen.Application {
	t.Helper()
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	card := e.staffCard(app.Id.String())
	code, raw := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), map[string]any{
		"action": "request_info", "comment": "Пришлите договор аренды помещения", "version": card.Application.Version})
	if code != 200 {
		t.Fatalf("request_info: %d %s", code, raw)
	}
	return e.get("client2", app.Id.String())
}

func TestNeedsInfoFromStaff_ClientCannotChangeTheScoredTerms(t *testing.T) {
	e := newEnv(t)
	app := askedByStaff(t, e)
	for name, body := range map[string]map[string]any{
		"amount":     {"amount": 4_000_000 * rub},
		"term":       {"termMonths": 12},
		"product":    {"productId": string(products.Equipment)},
		"collateral": {"fields": map[string]string{"collateral": "full"}},
	} {
		code, raw := e.do("PATCH", "/api/v1/applications/"+app.Id.String(), e.token("client2"), body)
		if code != 409 {
			t.Errorf("%s: the application was scored on its terms, a change must be refused: %d %s", name, code, raw)
		}
	}
	got := e.get("client2", app.Id.String())
	if got.Amount != 3_000_000*rub || got.TermMonths != 24 || got.ProductId != string(products.WorkingCapital) || got.Version != app.Version {
		t.Fatalf("nothing may change: %+v", got)
	}
}

func TestUpdateDraft_BumpsTheVersion(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, false)
	code, raw := e.do("PATCH", "/api/v1/applications/"+app.Id.String(), e.token("client1"), map[string]any{"amount": 1_200_000 * rub})
	if code != 200 {
		t.Fatalf("patch: %d %s", code, raw)
	}
	var got gen.Application
	mustJSON(t, raw, &got)
	if got.Version <= app.Version {
		t.Fatalf("a change of the terms must move the version (optimistic lock): %d -> %d", app.Version, got.Version)
	}
}

func TestNeedsInfoFromSystem_ClientChange_IsInTheJournal(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true)
	if _, err := e.s.Pool.Exec(context.Background(), "DELETE FROM documents WHERE application_id=$1 AND kind='financials'", app.Id.String()); err != nil {
		t.Fatal(err)
	}
	e.submitDirect(app)
	if got := e.get("client1", app.Id.String()); got.Status != gen.NEEDSINFO {
		t.Fatalf("want NEEDS_INFO, got %s", got.Status)
	}
	if code, raw := e.do("PATCH", "/api/v1/applications/"+app.Id.String(), e.token("client1"), map[string]any{"amount": 900_000 * rub}); code != 200 {
		t.Fatalf("a system request lets the client fix the application: %d %s", code, raw)
	}
	card := e.staffCard(app.Id.String())
	found := false
	for _, ev := range card.Events {
		found = found || ev.Event == "ClientUpdated"
	}
	if !found {
		t.Fatalf("a change after submission must be in the journal: %+v", card.Events)
	}
}

func TestLogin_ForwardedForFromAnUntrustedPeer_IsIgnored(t *testing.T) {
	e := newEnv(t, func(d *httpapi.Deps) { d.TrustedProxies = nil })
	var last int
	for i := 0; i < 6; i++ {
		last, _ = e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client2", "password": "wrong"},
			"X-Forwarded-For", "203.0.113."+string(rune('1'+i)))
	}
	if last != 429 {
		t.Fatalf("a caller that is not the proxy must not pick its own address: 6th attempt got %d", last)
	}
}

func decide(e *testEnv, login, id string, body map[string]any) (int, []byte) {
	return e.do("POST", "/api/v1/staff/applications/"+id+"/decision", e.token(login), body)
}

func TestStaffCounter_StaysWithinTheRequestAndTheProductRate(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	card := e.staffCard(app.Id.String())
	prod, _ := products.Get(products.WorkingCapital)
	for name, offer := range map[string]map[string]any{
		"more than requested": {"amount": 3_500_000 * rub, "termMonths": 24, "rateBp": 2450},
		"below the base rate": {"amount": 800_000 * rub, "termMonths": 24, "rateBp": prod.BaseRateBP - 50},
	} {
		code, raw := decide(e, "staff1", app.Id.String(), map[string]any{"action": "counter", "version": card.Application.Version, "offer": offer})
		if code != 422 {
			t.Errorf("%s: want 422, got %d %s", name, code, raw)
		}
	}
}

// secondAnalyst adds another analyst to the bank team.
func secondAnalyst(t *testing.T, e *testEnv) string {
	t.Helper()
	hash, _ := auth.HashPassword(seed.DemoPassword)
	if _, err := e.s.Pool.Exec(context.Background(), `INSERT INTO users (login, password_hash, role, full_name) VALUES ('staff2', $1, 'staff', 'Олег Ветров')`, hash); err != nil {
		t.Fatal(err)
	}
	return "staff2"
}

func TestClaim_OnlyWhileTheApplicationWaitsForADecision(t *testing.T) {
	e := newEnv(t)
	app := e.run("client1", e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true))
	if app.Status != gen.OFFERMADE {
		t.Fatalf("want an automatic offer, got %s", app.Status)
	}
	if code, raw := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/claim", e.token("staff1"), nil); code != 409 {
		t.Fatalf("nothing to take: want 409, got %d %s", code, raw)
	}
}

func TestDecision_ByAnotherAnalystThanTheAssignee_409(t *testing.T) {
	e := newEnv(t)
	other := secondAnalyst(t, e)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	if code, raw := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/claim", e.token("staff1"), nil); code != 200 {
		t.Fatalf("claim: %d %s", code, raw)
	}
	card := e.staffCard(app.Id.String())
	body := map[string]any{"action": "reject", "comment": "Не подходит", "version": card.Application.Version}
	if code, raw := decide(e, other, app.Id.String(), body); code != 409 {
		t.Fatalf("the application is with a colleague: want 409, got %d %s", code, raw)
	}
	if code, raw := decide(e, "staff1", app.Id.String(), body); code != 200 {
		t.Fatalf("the assignee decides: %d %s", code, raw)
	}
}

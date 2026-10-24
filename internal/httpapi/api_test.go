package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
)

func TestLogin_Success_ReturnsTokens(t *testing.T) {
	e := newEnv(t)
	code, raw := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "staff1", "password": "demo1234"})
	var tp gen.TokenPair
	mustJSON(t, raw, &tp)
	if code != 200 || tp.AccessToken == "" || tp.RefreshToken == "" || tp.Role != gen.TokenPairRoleStaff || tp.FullName == "" {
		t.Fatalf("%d %+v", code, tp)
	}
}

func TestLogin_WrongPassword_401(t *testing.T) {
	e := newEnv(t)
	for _, login := range []string{"client1", "nobody"} {
		code, raw := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": login, "password": "wrong"})
		if code != 401 {
			t.Fatalf("%s: want 401, got %d %s", login, code, raw)
		}
	}
}

func TestLogin_RateLimited_429(t *testing.T) {
	e := newEnv(t)
	var last int
	for i := 0; i < 6; i++ {
		last, _ = e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client1", "password": "wrong"})
	}
	if last != 429 {
		t.Fatalf("6th attempt must be rate limited, got %d", last)
	}
}

func TestLogin_SuccessfulLoginsDoNotConsumeAttempts(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 8; i++ {
		code, raw := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client1", "password": "demo1234"})
		if code != 200 {
			t.Fatalf("login %d: %d %s", i+1, code, raw)
		}
	}
}

func TestLogin_RateLimitOnlyAfterFailures_AndBlocksEvenRightPassword(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client2", "password": "wrong"})
	}
	if code, _ := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client2", "password": "demo1234"}); code != 429 {
		t.Fatalf("after 5 failures even the right password must wait: got %d", code)
	}
}

func TestLogin_FailuresFromOneAddressDoNotLockOthersOut(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client2", "password": "wrong"}, "X-Forwarded-For", "203.0.113.7")
	}
	if code, _ := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client2", "password": "demo1234"}, "X-Forwarded-For", "203.0.113.7"); code != 429 {
		t.Fatalf("the guessing address must be blocked: got %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client2", "password": "demo1234"}, "X-Forwarded-For", "198.51.100.9"); code != 200 {
		t.Fatalf("somebody else with the right password must get in: got %d", code)
	}
}

func TestProtected_NoToken_401(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/api/v1/me", "/api/v1/applications", "/api/v1/feed", "/api/v1/staff/queue"} {
		if code, _ := e.do("GET", p, "", nil); code != 401 {
			t.Errorf("%s without token: %d", p, code)
		}
	}
}

func TestProducts_Public(t *testing.T) {
	e := newEnv(t)
	code, raw := e.do("GET", "/api/v1/products", "", nil)
	var ps []gen.Product
	mustJSON(t, raw, &ps)
	if code != 200 || len(ps) != 4 {
		t.Fatalf("%d %d", code, len(ps))
	}
}

func TestRoleSeparation(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.do("GET", "/api/v1/staff/queue", e.token("client1"), nil); code != 403 {
		t.Errorf("client on staff route: %d", code)
	}
	if code, _ := e.do("GET", "/api/v1/applications", e.token("staff1"), nil); code != 403 {
		t.Errorf("staff on client route: %d", code)
	}
}

func TestRefreshAndMe(t *testing.T) {
	e := newEnv(t)
	_, raw := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "client1", "password": "demo1234"})
	var tp gen.TokenPair
	mustJSON(t, raw, &tp)
	code, raw := e.do("POST", "/api/v1/auth/refresh", "", map[string]string{"refreshToken": tp.RefreshToken})
	if code != 200 {
		t.Fatalf("refresh: %d %s", code, raw)
	}
	if code, _ := e.do("POST", "/api/v1/auth/refresh", "", map[string]string{"refreshToken": tp.AccessToken}); code != 401 {
		t.Errorf("access token must not refresh: %d", code)
	}
	code, raw = e.do("GET", "/api/v1/me", e.token("client1"), nil)
	var me gen.Me
	mustJSON(t, raw, &me)
	if code != 200 || me.Company == nil || me.Company.Inn != "7701234501" {
		t.Fatalf("me: %d %+v", code, me)
	}
}

func TestCreateDraft_Then_Patch_Persists(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, false)
	if app.Status != gen.DRAFT || app.Version < 1 || app.Company.Inn != "7701234501" {
		t.Fatalf("draft: %+v", app)
	}
	code, raw := e.do("PATCH", "/api/v1/applications/"+app.Id.String(), e.token("client1"), map[string]any{
		"amount": 1_500_000 * rub, "purpose": "Новая цель", "fields": map[string]string{"contact": "Иван"},
	})
	if code != 200 {
		t.Fatalf("patch: %d %s", code, raw)
	}
	got := e.get("client1", app.Id.String())
	if got.Amount != 1_500_000*rub || got.Purpose != "Новая цель" || got.Fields["contact"] != "Иван" || got.TermMonths != 12 {
		t.Fatalf("not persisted: %+v", got)
	}
}

func TestAmountOutOfRange_422(t *testing.T) {
	e := newEnv(t)
	code, raw := e.do("POST", "/api/v1/applications", e.token("client1"), map[string]any{
		"productId": "working_capital", "amount": 10, "termMonths": 12,
	})
	var er gen.Error
	mustJSON(t, raw, &er)
	if code != 422 || er.Fields == nil || (*er.Fields)[0].Field != "amount" {
		t.Fatalf("want 422 on amount, got %d %s", code, raw)
	}
	if code, _ := e.do("POST", "/api/v1/applications", e.token("client1"), map[string]any{"productId": "mortgage"}); code != 422 {
		t.Errorf("unknown product: %d", code)
	}
}

func TestSubmit_MissingFields_422_WithFieldList(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, false) // no docs
	code, raw := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/submit", e.token("client1"), nil, "Idempotency-Key", "key-12345678")
	var er gen.Error
	mustJSON(t, raw, &er)
	if code != 422 || er.Fields == nil || len(*er.Fields) < 2 {
		t.Fatalf("want 422 listing missing documents, got %d %s", code, raw)
	}
	if !strings.Contains((*er.Fields)[0].Message, "Загрузите документ") {
		t.Errorf("message must say what to do: %v", *er.Fields)
	}
}

func TestSubmit_RequiresIdempotencyKey(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true)
	if code, _ := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/submit", e.token("client1"), nil); code != 400 {
		t.Fatalf("missing Idempotency-Key must be 400, got %d", code)
	}
}

func TestSubmit_Twice_SameKey_SingleTransition(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true)
	c1, _ := e.submit("client1", app, "same-key-123")
	c2, _ := e.submit("client1", app, "same-key-123")
	if c1 != 200 || c2 != 200 {
		t.Fatalf("both calls with the same key must return 200, got %d %d", c1, c2)
	}
	evs, _ := e.s.ListEvents(context.Background(), app.Id.String())
	submits := 0
	for _, ev := range evs {
		if ev.Event == "Submit" {
			submits++
		}
	}
	if submits != 1 {
		t.Fatalf("want exactly one Submit transition, got %d", submits)
	}
	if c3, _ := e.submit("client1", app, "another-key-9"); c3 != 409 {
		t.Fatalf("different key on a submitted application must be 409, got %d", c3)
	}
}

func TestGetApplication_OtherUsers_404(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, false)
	for _, call := range [][2]string{{"GET", ""}, {"PATCH", ""}, {"POST", "/cancel"}, {"GET", "/timeline"}} {
		code, _ := e.do(call[0], "/api/v1/applications/"+app.Id.String()+call[1], e.token("client2"), map[string]any{"purpose": "x"})
		if code != 404 {
			t.Errorf("%s %s by another client: want 404, got %d", call[0], call[1], code)
		}
	}
	if code, _ := e.do("GET", "/api/v1/applications/not-a-uuid", e.token("client1"), nil); code != 400 && code != 404 {
		t.Errorf("garbage id: %d", code)
	}
}

func TestUpload_TooLarge_413(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, false)
	big := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 5*1024*1024+10)...)
	if code, _ := e.upload(e.token("client1"), app.Id.String(), "bank_statement", "big.pdf", big); code != 413 {
		t.Fatalf("want 413, got %d", code)
	}
}

func TestUpload_BadType_415(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, false)
	if code, _ := e.upload(e.token("client1"), app.Id.String(), "bank_statement", "evil.pdf", []byte("MZ\x90\x00 not a pdf at all")); code != 415 {
		t.Fatalf("want 415 for non-pdf content, got %d", code)
	}
	if code, _ := e.upload(e.token("client1"), app.Id.String(), "unknown_kind", "a.pdf", pdfBytes); code != 422 {
		t.Fatalf("unknown document kind: want 422, got %d", code)
	}
}

func TestLookup(t *testing.T) {
	e := newEnv(t)
	tok := e.token("client1")
	if code, _ := e.do("GET", "/api/v1/company/lookup?inn=123", tok, nil); code != 422 {
		t.Errorf("short INN: %d", code)
	}
	if code, _ := e.do("GET", "/api/v1/company/lookup?inn=12345abcde", tok, nil); code != 422 {
		t.Errorf("non-digit INN: %d", code)
	}
	if code, _ := e.do("GET", "/api/v1/company/lookup?inn=5000000000", tok, nil); code != 404 {
		t.Errorf("unknown INN: %d", code)
	}
	code, raw := e.do("GET", "/api/v1/company/lookup?inn=7701234501", tok, nil)
	var cl gen.CompanyLookup
	mustJSON(t, raw, &cl)
	if code != 200 || cl.Name != "ООО «Тёплый хлеб»" || cl.Kind != gen.CompanyLookupKindOoo {
		t.Fatalf("%d %+v", code, cl)
	}
	e.src.down.Store(true)
	if code, _ := e.do("GET", "/api/v1/company/lookup?inn=7701234501", tok, nil); code != 503 {
		t.Errorf("registry down: want 503, got %d", code)
	}
}

func TestAssistant_Recommend(t *testing.T) {
	e := newEnv(t)
	code, raw := e.do("POST", "/api/v1/assistant/recommend", e.token("client1"), map[string]string{"text": "Хочу купить станок за 2 млн"})
	var r gen.Recommendation
	mustJSON(t, raw, &r)
	if code != 200 || r.ProductId != "equipment" || r.Source != gen.RecommendationSourceRules || r.Amount == nil || *r.Amount != 2_000_000*rub {
		t.Fatalf("%d %+v", code, r)
	}
	if code, _ := e.do("POST", "/api/v1/assistant/recommend", e.token("client1"), map[string]string{"text": "  "}); code != 422 {
		t.Errorf("empty text: %d", code)
	}
}

func TestFlow_AutoOffer_Accept_Disbursed(t *testing.T) {
	e := newEnv(t)
	app := e.run("client1", e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true))
	if app.Status != gen.OFFERMADE || app.Offer == nil || app.Offer.Amount != 1_000_000*rub || app.Offer.RateBp != 1650 {
		t.Fatalf("want auto offer: %+v", app)
	}
	if !contains(app.AllowedActions, gen.Accept) || app.StatusMessage == "" {
		t.Fatalf("client must be able to accept and see a message: %+v", app.AllowedActions)
	}
	code, raw := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/offer/accept", e.token("client1"), nil)
	var done gen.Application
	mustJSON(t, raw, &done)
	if code != 200 || done.Status != gen.DISBURSED {
		t.Fatalf("accept: %d %s", code, raw)
	}
	code, raw = e.do("GET", "/api/v1/applications/"+app.Id.String()+"/timeline", e.token("client1"), nil)
	var tl []gen.TimelineEntry
	mustJSON(t, raw, &tl)
	if code != 200 || len(tl) < 6 || tl[len(tl)-1].ToStatus != "DISBURSED" || tl[0].Message == "" {
		t.Fatalf("timeline: %d %+v", code, tl)
	}
	if code, _ := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/offer/accept", e.token("client1"), nil); code != 409 {
		t.Errorf("second accept must be 409, got %d", code)
	}
}

func TestFlow_Rejected_ShowsReasons(t *testing.T) {
	e := newEnv(t)
	app := e.run("client4", e.draft("client4", products.WorkingCapital, 500_000*rub, 12, true))
	if app.Status != gen.REJECTED || app.Rejection == nil || len(app.Rejection.Reasons) == 0 {
		t.Fatalf("want rejection with reasons: %+v", app)
	}
	if len(app.AllowedActions) != 0 {
		t.Errorf("no actions after rejection: %v", app.AllowedActions)
	}
}

func TestFlow_Counter_Offer(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	if app.Status != gen.ANALYSTREVIEW {
		t.Fatalf("want ANALYST_REVIEW, got %s", app.Status)
	}
	// queue: review item first, with grade
	code, raw := e.do("GET", "/api/v1/staff/queue", e.token("staff1"), nil)
	var q gen.Queue
	mustJSON(t, raw, &q)
	if code != 200 || len(q.Items) == 0 || q.Items[0].Id.String() != app.Id.String() || q.Items[0].Grade == nil || q.Counts["ANALYST_REVIEW"] != 1 {
		t.Fatalf("queue: %d %+v", code, q)
	}
	card := e.staffCard(app.Id.String())
	if card.Scoring == nil || len(card.Scoring.Factors) != 6 || card.Recommended == nil || card.Enrichment == nil || card.Enrichment.Bki == nil {
		t.Fatalf("staff card incomplete: %+v", card)
	}
	if !containsAction(card.AllowedActions, gen.StaffApplicationAllowedActionsReject) {
		t.Fatalf("actions: %v", card.AllowedActions)
	}
	sc := card.Scoring
	if sc.Pd == nil || sc.Lgd == nil || sc.Ead == nil || sc.ExpectedLoss == nil || sc.CumulativePd == nil || sc.RiskPremiumBp == nil {
		t.Fatalf("the analyst must see PD, LGD, EAD and the expected loss: %+v", sc)
	}
	if *sc.Lgd != 0.6 || *sc.Ead != 3_000_000*rub || *sc.ExpectedLoss <= 0 {
		t.Fatalf("EL inputs wrong: LGD=%v EAD=%d EL=%d", *sc.Lgd, *sc.Ead, *sc.ExpectedLoss)
	}
	want := float64(*sc.Ead) * float64(*sc.CumulativePd) * float64(*sc.Lgd)
	if diff := float64(*sc.ExpectedLoss) - want; diff > want*0.01 || diff < -want*0.01 {
		t.Fatalf("EL must equal EAD x cumulative PD x LGD: %d vs %.0f", *sc.ExpectedLoss, want)
	}
	// approve with a counter-offer
	dec := map[string]any{"action": "counter", "comment": "Предлагаем меньшую сумму", "version": card.Application.Version,
		"offer": map[string]any{"amount": 800_000 * rub, "termMonths": 24, "rateBp": 2450}}
	code, raw = e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), dec)
	if code != 200 {
		t.Fatalf("decision: %d %s", code, raw)
	}
	got := e.get("client2", app.Id.String())
	if got.Status != gen.OFFERMADE || got.Offer == nil || got.Offer.Amount != 800_000*rub || got.Offer.Source != gen.Analyst {
		t.Fatalf("client must see the counter-offer: %+v", got)
	}
	if got.Offer.Changes == nil || len(*got.Offer.Changes) == 0 {
		t.Fatalf("counter-offer must explain what changed: %+v", got.Offer)
	}
	// client got an in-app notification
	code, raw = e.do("GET", "/api/v1/notifications", e.token("client2"), nil)
	var ns []gen.Notification
	mustJSON(t, raw, &ns)
	if code != 200 || len(ns) == 0 || ns[0].Read {
		t.Fatalf("notifications: %d %+v", code, ns)
	}
	if code, _ := e.do("POST", "/api/v1/notifications/"+ns[0].Id.String()+"/read", e.token("client2"), nil); code != 204 {
		t.Errorf("mark read: %d", code)
	}
	_, raw = e.do("GET", "/api/v1/notifications", e.token("client2"), nil)
	mustJSON(t, raw, &ns)
	if !ns[0].Read {
		t.Error("notification must be read")
	}
}

func TestStaffDecision_Reject_RequiresComment_422(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	card := e.staffCard(app.Id.String())
	for _, action := range []string{"reject", "request_info"} {
		code, _ := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), map[string]any{"action": action, "version": card.Application.Version})
		if code != 422 {
			t.Errorf("%s without comment: want 422, got %d", action, code)
		}
	}
	code, _ := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), map[string]any{
		"action": "counter", "version": card.Application.Version, "offer": map[string]any{"amount": 10, "termMonths": 24, "rateBp": 2000}})
	if code != 422 {
		t.Errorf("counter-offer outside product limits: want 422, got %d", code)
	}
}

func TestStaffDecision_StaleVersion_409(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	card := e.staffCard(app.Id.String())
	body := map[string]any{"action": "reject", "comment": "Не подходит", "version": card.Application.Version}
	if code, raw := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), body); code != 200 {
		t.Fatalf("first decision: %d %s", code, raw)
	}
	if code, _ := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), body); code != 409 {
		t.Fatalf("repeat with stale version: want 409, got %d", code)
	}
	got := e.get("client2", app.Id.String())
	if got.Status != gen.REJECTED || got.Rejection == nil || got.Rejection.Comment == nil || *got.Rejection.Comment != "Не подходит" {
		t.Fatalf("client must see the staff comment: %+v", got.Rejection)
	}
}

func TestStaffRequestInfo_ClientResponds_ReturnsToReview(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	card := e.staffCard(app.Id.String())
	code, raw := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), map[string]any{
		"action": "request_info", "comment": "Пришлите договор аренды помещения", "version": card.Application.Version})
	if code != 200 {
		t.Fatalf("request_info: %d %s", code, raw)
	}
	got := e.get("client2", app.Id.String())
	if got.Status != gen.NEEDSINFO || got.InfoRequest == nil || got.InfoRequest.Text != "Пришлите договор аренды помещения" || got.InfoRequest.RequestedBy != gen.InfoRequestRequestedByStaff {
		t.Fatalf("client must see the question: %+v", got)
	}
	if code, _ := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/respond", e.token("client2"), map[string]any{"text": " "}); code != 422 {
		t.Errorf("empty answer: %d", code)
	}
	code, raw = e.do("POST", "/api/v1/applications/"+app.Id.String()+"/respond", e.token("client2"), map[string]any{"text": "Договор аренды во вложении, помещение арендуем с 2021 года"})
	if code != 200 {
		t.Fatalf("respond: %d %s", code, raw)
	}
	if got := e.get("client2", app.Id.String()); got.Status != gen.ANALYSTREVIEW {
		t.Fatalf("an answer to staff must return to the analyst, got %s", got.Status)
	}
	if card := e.staffCard(app.Id.String()); len(card.Events) == 0 {
		t.Error("staff must see the history")
	}
}

func TestClientRespond_SystemRequest_ResumesProcessing(t *testing.T) {
	e := newEnv(t)
	app := e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true)
	// remove one required doc to force NEEDS_INFO at intake
	if _, err := e.s.Pool.Exec(context.Background(), "DELETE FROM documents WHERE application_id=$1 AND kind='financials'", app.Id.String()); err != nil {
		t.Fatal(err)
	}
	// bypass the API completeness check by submitting through the pipeline directly
	e.submitDirect(app)
	got := e.get("client1", app.Id.String())
	if got.Status != gen.NEEDSINFO || got.InfoRequest == nil || got.InfoRequest.Items == nil {
		t.Fatalf("want NEEDS_INFO with items: %+v", got)
	}
	// responding without the document is refused and lists what is missing
	code, raw := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/respond", e.token("client1"), map[string]any{"text": "Готово"})
	if code != 422 {
		t.Fatalf("respond without the document: want 422, got %d %s", code, raw)
	}
	if c, r := e.upload(e.token("client1"), app.Id.String(), "financials", "f.pdf", pdfBytes); c != 201 {
		t.Fatalf("upload: %d %s", c, r)
	}
	if code, raw := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/respond", e.token("client1"), map[string]any{"text": "Загрузил"}); code != 200 {
		t.Fatalf("respond: %d %s", code, raw)
	}
	_ = e.pipe.Process(context.Background(), app.Id.String())
	if got := e.get("client1", app.Id.String()); got.Status != gen.OFFERMADE {
		t.Fatalf("processing must resume to an offer, got %s", got.Status)
	}
}

func TestCancelAndDecline(t *testing.T) {
	e := newEnv(t)
	d := e.draft("client1", products.Express, 100_000*rub, 6, false)
	code, raw := e.do("POST", "/api/v1/applications/"+d.Id.String()+"/cancel", e.token("client1"), nil)
	var got gen.Application
	mustJSON(t, raw, &got)
	if code != 200 || got.Status != gen.CANCELLED {
		t.Fatalf("cancel draft: %d %s", code, raw)
	}
	app := e.run("client1", e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true))
	code, raw = e.do("POST", "/api/v1/applications/"+app.Id.String()+"/offer/decline", e.token("client1"), nil)
	mustJSON(t, raw, &got)
	if code != 200 || got.Status != gen.DECLINEDBYCLIENT {
		t.Fatalf("decline: %d %s", code, raw)
	}
	if code, _ := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/cancel", e.token("client1"), nil); code != 409 {
		t.Errorf("cancel after decline: want 409, got %d", code)
	}
}

func TestStaffExplanation(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	code, raw := e.do("GET", "/api/v1/staff/applications/"+app.Id.String()+"/explanation", e.token("staff1"), nil)
	var ex gen.Explanation
	mustJSON(t, raw, &ex)
	if code != 200 || ex.Source != gen.ExplanationSourceRules || !strings.Contains(ex.Text, "Класс риска") {
		t.Fatalf("%d %+v", code, ex)
	}
	code, raw = e.do("GET", "/api/v1/staff/applications/"+app.Id.String()+"/explanation?audience=client", e.token("staff1"), nil)
	mustJSON(t, raw, &ex)
	if code != 200 || strings.Contains(ex.Text, "Класс риска") {
		t.Fatalf("client audience must be plain: %d %s", code, ex.Text)
	}
}

func TestFeed_ReturnsCardsForClient_AndReactsToApplication(t *testing.T) {
	e := newEnv(t)
	code, raw := e.do("GET", "/api/v1/feed", e.token("client1"), nil)
	var before []gen.FeedCard
	mustJSON(t, raw, &before)
	if code != 200 || len(before) == 0 {
		t.Fatalf("feed: %d %s", code, raw)
	}
	e.run("client1", e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true))
	_, raw = e.do("GET", "/api/v1/feed", e.token("client1"), nil)
	var after []gen.FeedCard
	mustJSON(t, raw, &after)
	if after[0].Kind != gen.FeedCardKindStatus || after[0].ApplicationId == nil {
		t.Fatalf("an offer must lead the feed: %+v", after[0])
	}
	_, raw = e.do("GET", "/api/v1/feed", e.token("client3"), nil)
	var other []gen.FeedCard
	mustJSON(t, raw, &other)
	if len(other) > 0 && other[0].Title == before[0].Title && other[len(other)-1].Title == before[len(before)-1].Title {
		t.Error("another company must get a different feed")
	}
}

func TestPipeline_ManualCheck_ShownToStaff(t *testing.T) {
	e := newEnv(t)
	e.src.down.Store(true)
	app := e.run("client1", e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true))
	if app.Status != gen.ANALYSTREVIEW {
		t.Fatalf("want ANALYST_REVIEW after sources stay down, got %s", app.Status)
	}
	card := e.staffCard(app.Id.String())
	if card.ManualCheck == nil || !*card.ManualCheck || card.Scoring != nil {
		t.Fatalf("manual-check flag expected without scoring: %+v", card)
	}
	// the analyst can still approve: the offer uses the base rate
	code, raw := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), map[string]any{
		"action": "approve", "comment": "Проверил вручную", "version": card.Application.Version})
	if code != 200 {
		t.Fatalf("approve: %d %s", code, raw)
	}
	if got := e.get("client1", app.Id.String()); got.Status != gen.OFFERMADE || got.Offer == nil || got.Offer.RateBp != 1650 {
		t.Fatalf("manual approval offer: %+v", got.Offer)
	}
}

func TestSSE_ClientGetsStatusChangeAfterStaffDecision(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	card := e.staffCard(app.Id.String())

	req, _ := http.NewRequest("GET", e.srv.URL+"/api/v1/events?access_token="+e.token("client2"), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("sse: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	waitFor := func(substr string) bool {
		deadline := time.After(3 * time.Second)
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					return false
				}
				if strings.Contains(l, substr) {
					return true
				}
			case <-deadline:
				return false
			}
		}
	}
	if !waitFor(": connected") {
		t.Fatal("stream must announce the connection")
	}
	go e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/decision", e.token("staff1"), map[string]any{
		"action": "reject", "comment": "Отказ", "version": card.Application.Version})
	if !waitFor(`"status":"REJECTED"`) {
		t.Fatal("client did not receive the status change over SSE")
	}
}

func TestSSE_RequiresToken(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.do("GET", "/api/v1/events", "", nil); code != 401 {
		t.Fatalf("want 401, got %d", code)
	}
	if code, _ := e.do("GET", "/api/v1/events?access_token=garbage", "", nil); code != 401 {
		t.Fatalf("want 401, got %d", code)
	}
}

func TestHealth(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/healthz", "/readyz"} {
		if code, _ := e.do("GET", p, "", nil); code != 200 {
			t.Errorf("%s: %d", p, code)
		}
	}
}

func contains(xs []gen.ApplicationAllowedActions, x gen.ApplicationAllowedActions) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsAction(xs []gen.StaffApplicationAllowedActions, x gen.StaffApplicationAllowedActions) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

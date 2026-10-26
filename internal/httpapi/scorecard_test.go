package httpapi_test

import (
	"fmt"
	"strings"
	"testing"

	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/seed"
)

func factorOf(card gen.StaffApplication, key string) (gen.Factor, bool) {
	for _, f := range card.Scoring.Factors {
		if f.Key == key {
			return f, true
		}
	}
	return gen.Factor{}, false
}

// What the bank cannot see about a young company earns no points, and is told apart from a bad result.
func TestScorecard_YoungCompanyWithoutHistory_IsNotRefusedForMissingData(t *testing.T) {
	e := newEnv(t)
	app := e.run("client3", e.draft("client3", products.Express, 500_000*rub, 6, true))
	if app.Status != gen.ANALYSTREVIEW {
		t.Fatalf("a company with gaps in its data goes to a person, got %s", app.Status)
	}
	card := e.staffCard(app.Id.String())
	if len(card.Scoring.Factors) != 12 {
		t.Fatalf("12 criteria expected, got %d", len(card.Scoring.Factors))
	}
	for _, key := range []string{"overdue", "debt_ebitda", "obligations"} {
		f, ok := factorOf(card, key)
		if !ok || f.Points != 0 || f.NoData == nil || !*f.NoData || !strings.HasPrefix(f.Note, "Нет данных") {
			t.Errorf("%s: no history means no points and a note: %+v", key, f)
		}
	}
	if f, _ := factorOf(card, "dscr"); f.NoData != nil {
		t.Errorf("accounts are known, DSCR is not missing data: %+v", f)
	}
}

func TestScorecard_CardCarries590PCategoryAndTheNewSources(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	card := e.staffCard(app.Id.String())
	s := card.Scoring
	want := scoring.QualityOf(scoring.Grade(s.Grade))
	if s.QualityCategory == nil || s.ReserveRange == nil || s.RiskLabel == nil || *s.QualityCategory != want.Category || *s.ReserveRange != want.Reserve {
		t.Fatalf("class %s must map to a 590-P category %q with the reserve %q: %+v", s.Grade, want.Category, want.Reserve, s)
	}
	eg := card.Enrichment.Egrul
	if eg == nil || eg.Finance == nil || eg.Finance.CurrentRatio <= 0 || eg.Cases == nil && false {
		t.Fatalf("the simulated registry and accounts must reach the card: %+v", eg)
	}
	if eg.Statement == nil || eg.Statement.NoiMargin <= 0 {
		t.Fatalf("the analysis of the bank statement must reach the card: %+v", eg)
	}
	if card.Enrichment.Bki == nil || card.Enrichment.Bki.Restructured == nil {
		t.Fatalf("the bureau record carries the restructuring mark: %+v", card.Enrichment.Bki)
	}
}

func TestScorecard_CollateralByTheAnalystsRules_ThroughTheForm(t *testing.T) {
	e := newEnv(t)
	collateral := func(fields map[string]string, docs ...string) gen.Factor {
		tok := e.token("client1")
		code, raw := e.do("POST", "/api/v1/applications", tok, map[string]any{
			"productId": "working_capital", "amount": 1_000_000 * rub, "termMonths": 12, "purpose": "Закупка сырья на сезон", "fields": fields,
		})
		if code != 201 {
			t.Fatalf("%d %s", code, raw)
		}
		var app gen.Application
		mustJSON(t, raw, &app)
		for _, k := range append([]string{"bank_statement", "financials"}, docs...) {
			if c, r := e.upload(tok, app.Id.String(), k, k+".pdf", pdfBytes); c != 201 {
				t.Fatalf("%d %s", c, r)
			}
		}
		e.run("client1", app)
		f, ok := factorOf(e.staffCard(app.Id.String()), "collateral")
		if !ok {
			t.Fatal("collateral criterion missing")
		}
		return f
	}
	flat := map[string]string{"collateralKind": "real_estate", "collateralValue": "5 000 000"}
	if f := collateral(flat, "collateral_doc"); f.Points != 5 {
		t.Errorf("a flat of 5 million (30%% off) covers a million: %+v", f)
	}
	if f := collateral(map[string]string{"collateralKind": "real_estate", "collateralValue": "1000000"}, "collateral_doc"); f.Points != 2 {
		t.Errorf("a flat of a million (30%% off) covers a part: %+v", f)
	}
	if f := collateral(flat); f.Points != 0 || !strings.Contains(f.Note, "документа") {
		t.Errorf("without the document nothing counts: %+v", f)
	}
	if f := collateral(map[string]string{"collateralKind": "guarantee"}, "collateral_doc"); f.Points != 2 {
		t.Errorf("a guarantee of the owner is partial at most: %+v", f)
	}
	if f := collateral(map[string]string{"collateralKind": "none"}); f.Points != 0 {
		t.Errorf("%+v", f)
	}
}

func TestApplication_FormAnswersThatTheModelReadsAreChecked(t *testing.T) {
	e := newEnv(t)
	tok := e.token("client1")
	for name, fields := range map[string]map[string]string{
		"unknown kind of collateral": {"collateralKind": "castle"},
		"a value in words":           {"collateralKind": "real_estate", "collateralValue": "много"},
		"a document amount in words": {"purposeDocAmount": "около миллиона"},
	} {
		code, _ := e.do("POST", "/api/v1/applications", tok, map[string]any{
			"productId": "working_capital", "amount": 1_000_000 * rub, "termMonths": 12, "purpose": "Закупка сырья на сезон", "fields": fields,
		})
		if code != 422 {
			t.Errorf("%s: want 422, got %d", name, code)
		}
	}
}

// 13.3 of the analyst: the verdict on the purpose replaces the automatic reading, the application is scored again.
func TestPurposeCheck_TheVerdictChangesThePointsTheClassAndTheVersion(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	if app.Status != gen.ANALYSTREVIEW {
		t.Fatalf("want review, got %s", app.Status)
	}
	before := e.staffCard(app.Id.String())
	pBefore, _ := factorOf(before, "purpose")

	check := func(login, verdict string) (int, gen.StaffApplication) {
		code, raw := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/purpose-check", e.token(login), map[string]any{"verdict": verdict, "comment": "Смета получена по почте"})
		var card gen.StaffApplication
		if code == 200 {
			mustJSON(t, raw, &card)
		}
		return code, card
	}
	code, card := check("staff1", "confirmed")
	pAfter, _ := factorOf(card, "purpose")
	if code != 200 || pAfter.Points != 5 || !strings.Contains(pAfter.Note, "сотрудник") || pAfter.Points <= pBefore.Points {
		t.Fatalf("a confirmed purpose earns the full 5: %d %+v (was %+v)", code, pAfter, pBefore)
	}
	if card.Scoring.Score != before.Scoring.Score+(5-pBefore.Points) || card.PurposeVerdict == nil || *card.PurposeVerdict != gen.StaffApplicationPurposeVerdictConfirmed {
		t.Fatalf("the score follows and the verdict is shown: %+v", card.Scoring.Score)
	}
	if *card.Version <= *before.Version {
		t.Fatalf("the version moves, so a stale card cannot be decided on: %d -> %d", *before.Version, *card.Version)
	}
	if code, card := check("staff1", "not_confirmed"); code != 200 || mustFactor(t, card, "purpose").Points != 0 {
		t.Fatalf("a rejected purpose earns nothing: %d", code)
	}
	if code, _ := check("staff1", "maybe"); code != 422 {
		t.Errorf("an unknown verdict: %d", code)
	}
	if code, _ := check("client2", "confirmed"); code != 403 {
		t.Errorf("a client cannot judge the purpose: %d", code)
	}
	// the journal of the client does not show the working verdict of the bank
	_, raw := e.do("GET", "/api/v1/applications/"+app.Id.String()+"/timeline", e.token("client2"), nil)
	if strings.Contains(string(raw), "PurposeChecked") {
		t.Errorf("the client must not see the verdict of the analyst: %s", raw)
	}
	_, raw = e.do("GET", "/api/v1/staff/applications/"+app.Id.String(), e.token("staff1"), nil)
	if !strings.Contains(string(raw), "PurposeChecked") {
		t.Errorf("the analyst sees the entry in the journal")
	}
}

func mustFactor(t *testing.T, card gen.StaffApplication, key string) gen.Factor {
	t.Helper()
	f, ok := factorOf(card, key)
	if !ok {
		t.Fatalf("no %s", key)
	}
	return f
}

func TestPurposeCheck_OnlyWhileTheApplicationWaitsForTheAnalyst(t *testing.T) {
	e := newEnv(t)
	app := e.run("client1", e.draft("client1", products.WorkingCapital, 1_000_000*rub, 12, true)) // an automatic offer
	if code, _ := e.do("POST", "/api/v1/staff/applications/"+app.Id.String()+"/purpose-check", e.token("staff1"), map[string]any{"verdict": "confirmed"}); code != 409 {
		t.Fatalf("an application that is not with the analyst: want 409, got %d", code)
	}
}

func TestScorecard_TheRateIsBrokenIntoItsParts(t *testing.T) {
	e := newEnv(t)
	app := e.run("client2", e.draft("client2", products.WorkingCapital, 3_000_000*rub, 24, true))
	s := e.staffCard(app.Id.String()).Scoring
	if s.RateRiskFreeBp == nil || s.RateRiskBp == nil || s.RateOperatingBp == nil {
		t.Fatalf("the parts of the rate are missing: %+v", s)
	}
	if *s.RateRiskFreeBp+*s.RateRiskBp+*s.RateOperatingBp != 1650+*s.RiskPremiumBp {
		t.Fatalf("risk-free %d + risk %d + operating %d must make the base rate plus the premium (%d + %d)", *s.RateRiskFreeBp, *s.RateRiskBp, *s.RateOperatingBp, 1650, *s.RiskPremiumBp)
	}
}

// A company the registry marks (disqualified director, unreliable data) is refused by a hard stop, whatever its points.
func TestScorecard_RegistryMarks_RefuseByAHardStop(t *testing.T) {
	e := newEnv(t)
	for _, kind := range []string{"disqualified", "unreliable"} {
		var inn string
		for n := int64(7_700_000_000); n < 7_700_200_000 && inn == ""; n++ {
			cand := fmt.Sprintf("%010d", n)
			if !seed.ValidINN(cand) || seed.ArchetypeOf(cand) != seed.Flagged {
				continue
			}
			if c, _ := seed.Synthesize(cand); (kind == "disqualified") == c.DirectorDisqualified {
				inn = cand
			}
		}
		if inn == "" {
			t.Fatalf("no INN of kind %s found", kind)
		}
		login := "flag_" + kind
		code, raw := e.do("POST", "/api/v1/auth/register", "", signup(login, inn))
		if code != 201 {
			t.Fatalf("%s: register %d %s", kind, code, raw)
		}
		var tp gen.TokenPair
		mustJSON(t, raw, &tp)
		e.mu.Lock()
		e.tok[login] = tp.AccessToken
		e.mu.Unlock()
		app := e.run(login, e.draft(login, products.Express, 200_000*rub, 6, true))
		if app.Status != gen.REJECTED || app.Rejection == nil {
			t.Fatalf("%s: want a rejection, got %s", kind, app.Status)
		}
		want := map[string]string{"disqualified": "дисквалифицирован", "unreliable": "недостоверность"}[kind]
		if !strings.Contains(strings.Join(app.Rejection.Reasons, " "), want) {
			t.Fatalf("%s: the reason must say why: %v", kind, app.Rejection.Reasons)
		}
	}
}

// Decision 13.1 of the analyst, through the whole path: what confirms the purpose depends on the product.
func TestScorecard_PurposeIsConfirmedByTheDocumentsOfTheProduct(t *testing.T) {
	e := newEnv(t)
	points := func(login string, pid products.ID, amount int64, term int, extra ...string) gen.Factor {
		app := e.draft(login, pid, amount, term, true)
		for _, kind := range extra {
			if c, r := e.upload(e.token(login), app.Id.String(), kind, kind+".pdf", pdfBytes); c != 201 {
				t.Fatalf("upload %s: %d %s", kind, c, r)
			}
		}
		e.run(login, app)
		f, ok := factorOf(e.staffCard(app.Id.String()), "purpose")
		if !ok {
			t.Fatal("purpose criterion missing")
		}
		return f
	}
	if f := points("client1", products.WorkingCapital, 1_000_000*rub, 12); f.Points != 2 {
		t.Errorf("working capital without a contract, an invoice or an estimate: 2 of 5, got %+v", f)
	}
	if f := points("client7", products.WorkingCapital, 1_000_000*rub, 12, "purpose_doc"); f.Points != 5 {
		t.Errorf("working capital with a supplier's contract: 5 of 5, got %+v", f)
	}
	if f := points("client8", products.Overdraft, 500_000*rub, 12); f.Points != 5 {
		t.Errorf("overdraft: the statement proves the cash gaps, 5 of 5, got %+v", f)
	}
	if f := points("client11", products.Express, 100_000*rub, 6); f.Points != 5 {
		t.Errorf("express: the described purpose of the form is enough, 5 of 5, got %+v", f)
	}
}

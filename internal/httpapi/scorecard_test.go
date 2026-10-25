package httpapi_test

import (
	"fmt"
	"strings"
	"testing"

	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
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
	if s.QualityCategory == nil || s.ReserveRange == nil || s.RiskLabel == nil || !strings.Contains(*s.QualityCategory, "II–III") || *s.ReserveRange != "1–50 %" {
		t.Fatalf("class %s must map to a 590-P category: %+v", s.Grade, s)
	}
	eg := card.Enrichment.Egrul
	if eg == nil || eg.Finance == nil || eg.Finance.CurrentRatio <= 0 || eg.Litigation == nil {
		t.Fatalf("the simulated registry and accounts must reach the card: %+v", eg)
	}
	if eg.Statement == nil || eg.Statement.NoiMargin <= 0 {
		t.Fatalf("the analysis of the bank statement must reach the card: %+v", eg)
	}
	if card.Enrichment.Bki == nil || card.Enrichment.Bki.Restructured == nil {
		t.Fatalf("the bureau record carries the restructuring mark: %+v", card.Enrichment.Bki)
	}
}

func TestScorecard_CollateralFromTheWizardChangesTheScore_NotTheDecisionRules(t *testing.T) {
	e := newEnv(t)
	score := func(collateral string) int {
		tok := e.token("client1")
		code, raw := e.do("POST", "/api/v1/applications", tok, map[string]any{
			"productId": "working_capital", "amount": 1_000_000 * rub, "termMonths": 12, "purpose": "Закупка сырья на сезон",
			"fields": map[string]string{"collateral": collateral},
		})
		if code != 201 {
			t.Fatalf("%d %s", code, raw)
		}
		var app gen.Application
		mustJSON(t, raw, &app)
		for _, k := range []string{"bank_statement", "financials"} {
			if c, r := e.upload(tok, app.Id.String(), k, k+".pdf", pdfBytes); c != 201 {
				t.Fatalf("%d %s", c, r)
			}
		}
		e.run("client1", app)
		card := e.staffCard(app.Id.String())
		f, ok := factorOf(card, "collateral")
		if !ok {
			t.Fatal("collateral criterion missing")
		}
		return f.Points
	}
	if none, full, partial := score("none"), score("full"), score("partial"); none != 0 || full != 5 || partial != 2 {
		t.Fatalf("collateral points: none %d, partial %d, full %d", none, partial, full)
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

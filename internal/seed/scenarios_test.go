package seed_test

import (
	"testing"
	"time"

	"matritsa/internal/decision"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/seed"
)

const rub = int64(100)

func evaluate(t *testing.T, c seed.Company, pid products.ID, amount int64, term int) (scoring.Result, decision.Decision) {
	t.Helper()
	p, _ := products.Get(pid)
	age := int(time.Since(c.RegisteredAt).Hours() / 24 / 30)
	res := scoring.Score(scoring.Input{
		CompanyAgeMonths: age, MonthlyRevenue: c.Revenue, MonthlyObligations: c.MonthlyObligations,
		BKI: c.BKI, EGRUL: c.EGRUL(), Amount: amount, TermMonths: term, RateBP: p.BaseRateBP, OKVEDGroup: c.OKVEDGroup, LGD: p.LGD,
	}, scoring.DefaultConfig())
	return res, decision.Decide(res, p, amount, term)
}

func TestDemoCompanies_ProduceIntendedOutcomes(t *testing.T) {
	cases := []struct {
		inn     string
		product products.ID
		amount  int64
		term    int
		want    decision.Outcome
	}{
		{"7701234501", products.WorkingCapital, 1_000_000 * rub, 12, decision.AutoOffer},
		{"771234500002", products.WorkingCapital, 3_000_000 * rub, 24, decision.Review},
		{"7701234503", products.Express, 500_000 * rub, 6, decision.Review},
		{"7701234504", products.WorkingCapital, 500_000 * rub, 12, decision.Reject},
		{"771234500005", products.Equipment, 1_500_000 * rub, 36, decision.Review},
		{"7701234506", products.WorkingCapital, 1_000_000 * rub, 12, decision.Reject},
	}
	for _, tc := range cases {
		c, ok := seed.ByINN(tc.inn)
		if !ok {
			t.Fatalf("company %s missing", tc.inn)
		}
		res, d := evaluate(t, c, tc.product, tc.amount, tc.term)
		t.Logf("%-40s grade=%s score=%d load=%.2f affordable=%d outcome=%s", c.Name, res.Grade, res.Score, res.DebtLoad, res.MaxAffordableAmount/rub, d.Outcome)
		if d.Outcome != tc.want {
			t.Errorf("%s: outcome %s, want %s (grade %s, score %d)", c.Name, d.Outcome, tc.want, res.Grade, res.Score)
		}
	}
}

func TestDemoCompany2_CounterOfferIsSmallerThanRequested(t *testing.T) {
	c, _ := seed.ByINN("771234500002")
	_, d := evaluate(t, c, products.WorkingCapital, 3_000_000*rub, 24)
	if d.Recommended == nil || d.Recommended.Amount >= 3_000_000*rub {
		t.Fatalf("expected a reduced counter-offer, got %+v", d.Recommended)
	}
}

func TestDemoCompanies_UniqueLoginsAndINNs(t *testing.T) {
	logins, inns := map[string]bool{}, map[string]bool{}
	for _, c := range seed.Companies() {
		if logins[c.Login] || inns[c.INN] {
			t.Fatalf("duplicate %s/%s", c.Login, c.INN)
		}
		logins[c.Login], inns[c.INN] = true, true
		if l := len(c.INN); (c.Kind == "ooo" && l != 10) || (c.Kind == "ip" && l != 12) {
			t.Errorf("%s: INN length %d does not match kind %s", c.Name, l, c.Kind)
		}
	}
}

// The complaint that started the rework: one borrower must not get a different risk class
// just because the request is bigger or longer.
func TestDemoCompanies_ClassDoesNotChangeWithTheRequest(t *testing.T) {
	for _, c := range seed.Companies() {
		small, _ := evaluate(t, c, products.Equipment, 2_000_000*rub, 30)
		big, _ := evaluate(t, c, products.Equipment, 20_000_000*rub, 40)
		if small.Grade != big.Grade || small.Score != big.Score {
			t.Errorf("%s: class %s/%d on 2M/30 vs %s/%d on 20M/40", c.Name, small.Grade, small.Score, big.Grade, big.Score)
		}
		if !(big.ExpectedLoss > small.ExpectedLoss) {
			t.Errorf("%s: the bigger and longer loan must carry more expected loss (%d vs %d)", c.Name, small.ExpectedLoss, big.ExpectedLoss)
		}
	}
}

func TestDemoCompanies_BigRequestIsHandledByAffordabilityNotByClass(t *testing.T) {
	c, _ := seed.ByINN("771234500005") // Kuznetsova, class B
	res, d := evaluate(t, c, products.Equipment, 20_000_000*rub, 40)
	if res.DebtLoad < 1 {
		t.Fatalf("20M is far beyond what this business can pay: load %.0f%%", res.DebtLoad*100)
	}
	if d.Outcome != decision.Review || d.Recommended == nil || d.Recommended.Amount > res.MaxAffordableAmount {
		t.Fatalf("expected a counter-offer within the affordable limit, got %+v", d)
	}
}

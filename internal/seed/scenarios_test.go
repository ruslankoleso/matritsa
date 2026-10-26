package seed_test

import (
	"fmt"
	"testing"
	"time"

	"matritsa/internal/decision"
	"matritsa/internal/products"
	"matritsa/internal/scoring"
	"matritsa/internal/seed"
)

const rub = int64(100)

// evaluate runs the same steps as the pipeline: score, decide, and score a counter-offer on its own.
func evaluate(t *testing.T, c seed.Company, pid products.ID, amount int64, term int) (scoring.Result, decision.Decision) {
	t.Helper()
	p, _ := products.Get(pid)
	age := int(time.Since(c.RegisteredAt).Hours() / 24 / 30)
	// the demo applications attach the required documents only; which of them confirm the purpose is in the catalog
	proof := false
	for _, d := range p.Documents {
		proof = proof || (d.Required && d.ConfirmsPurpose)
	}
	in := scoring.Input{
		CompanyAgeMonths: age, MonthlyRevenue: c.Revenue, MonthlyObligations: c.MonthlyObligations,
		BKI: c.BKI, EGRUL: c.EGRUL(), PurposeText: "Закупка сырья и материалов перед сезоном", PurposeDocs: proof, PurposeFromForm: p.PurposeFromForm,
		Amount: amount, TermMonths: term, RateBP: p.BaseRateBP, OKVEDGroup: c.OKVEDGroup, LGD: p.LGD,
	}
	cfg := scoring.DefaultConfig()
	res := scoring.Score(in, cfg)
	rescore := func(a int64) scoring.Result {
		other := in
		other.Amount = a
		return scoring.Score(other, cfg)
	}
	return res, decision.DecideRescoring(res, p, amount, term, rescore)
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

// Every demo company with the grade and the points the scorecard gives it: the stories must read right.
func TestDemoCompanies_GradesMatchTheirStories(t *testing.T) {
	want := map[string]scoring.Grade{
		"client1": "A", "client2": "C", "client3": "C", "client4": "D", "client5": "B", "client6": "D",
		"client7": "A", "client8": "A", "client9": "C", "client10": "D", "client11": "A",
	}
	for _, c := range seed.Companies() {
		res, d := evaluate(t, c, products.WorkingCapital, 1_000_000*rub, 12)
		t.Logf("%-8s %-38s %s %3d points, missing %2d, outcome %s", c.Login, c.Name, res.Grade, res.Score, res.MissingPoints, d.Outcome)
		if res.Grade != want[c.Login] {
			t.Errorf("%s: grade %s (%d points), want %s", c.Login, res.Grade, res.Score, want[c.Login])
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

// The grade belongs to the loan: for the same borrower a bigger and longer loan covers worse, never better,
// and carries more expected loss.
func TestDemoCompanies_BiggerLoanNeverScoresBetter(t *testing.T) {
	for _, c := range seed.Companies() {
		small, _ := evaluate(t, c, products.Equipment, 2_000_000*rub, 30)
		big, _ := evaluate(t, c, products.Equipment, 20_000_000*rub, 40)
		if big.Score > small.Score {
			t.Errorf("%s: %d points on 20M/40 vs %d on 2M/30", c.Name, big.Score, small.Score)
		}
		if !(big.ExpectedLoss > small.ExpectedLoss) {
			t.Errorf("%s: the bigger and longer loan must carry more expected loss (%d vs %d)", c.Name, small.ExpectedLoss, big.ExpectedLoss)
		}
	}
}

func TestDemoCompanies_BigRequestIsAnsweredWithACounterOffer(t *testing.T) {
	c, _ := seed.ByINN("771234500005") // Kuznetsova
	small, _ := evaluate(t, c, products.Equipment, 2_000_000*rub, 30)
	res, d := evaluate(t, c, products.Equipment, 20_000_000*rub, 40)
	t.Logf("2M/30: %s %d; 20M/40: %s %d, load %.0f%%, offer %+v", small.Grade, small.Score, res.Grade, res.Score, res.DebtLoad*100, d.Recommended)
	if res.DebtLoad < 1 {
		t.Fatalf("20M is far beyond what this business can pay: load %.0f%%", res.DebtLoad*100)
	}
	if d.Outcome != decision.Review || d.Recommended == nil || d.Recommended.Amount > res.MaxAffordableAmount {
		t.Fatalf("expected a counter-offer within the affordable limit, got %+v", d)
	}
}

// Companies that appear at sign-up must behave the way their hint says, for any INN of that kind.
func TestSyntheticCompanies_ProduceTheOutcomeOfTheirKind(t *testing.T) {
	want := map[seed.Archetype]decision.Outcome{
		seed.Reliable: decision.AutoOffer, seed.Borderline: decision.Review, seed.Young: decision.Review,
		seed.Indebted: decision.Review, seed.Liquidated: decision.Reject, seed.Overdue: decision.Reject, seed.Flagged: decision.Reject,
	}
	seen := map[seed.Archetype]int{}
	for n := int64(7_900_000_000); n < 7_900_020_000; n++ {
		inn := fmt.Sprintf("%010d", n)
		c, ok := seed.Synthesize(inn)
		if !ok {
			continue
		}
		a := seed.ArchetypeOf(inn)
		seen[a]++
		_, d := evaluate(t, c, products.WorkingCapital, 1_000_000*rub, 12)
		if d.Outcome != want[a] {
			t.Fatalf("%s (%s, %s): outcome %s, want %s", inn, a, c.Name, d.Outcome, want[a])
		}
	}
	for a := range want {
		if seen[a] < 20 {
			t.Errorf("archetype %s is too rare in the sample: %d", a, seen[a])
		}
	}
}

// A good company with a thin repayment record is not approved automatically, however clean the class is.
func TestDemoCompany1_WithoutRepaymentExperience_IsNotApprovedAutomatically(t *testing.T) {
	c, _ := seed.ByINN("7701234501")
	if _, d := evaluate(t, c, products.WorkingCapital, 1_000_000*rub, 12); d.Outcome != decision.AutoOffer {
		t.Fatalf("the demo company with a real history must stay automatic: %s", d.Outcome)
	}
	thin := *c.BKI
	thin.TotalRepaid = 100_000 * rub
	thin.ClosedLoans = 1
	c.BKI = &thin
	res, d := evaluate(t, c, products.WorkingCapital, 1_000_000*rub, 12)
	if res.Grade != "A" || d.Outcome != decision.Review {
		t.Fatalf("class %s must stay A and the outcome must be a review, got %s", res.Grade, d.Outcome)
	}
}

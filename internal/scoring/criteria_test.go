package scoring

import (
	"strings"
	"testing"

	"matritsa/internal/integrations"
)

// The tiers of the analyst's scorecard, one criterion at a time.
func TestCriteria_Tiers(t *testing.T) {
	cfg := DefaultConfig()
	type tc struct {
		name string
		key  string
		set  func(*Input)
		want int
	}
	withBKI := func(b integrations.BKIReport) func(*Input) {
		b.HasHistory = true
		return func(in *Input) { in.BKI = &b }
	}
	// revenue 2 000 000 a month and a 30% cash flow margin: the cash flow is 600 000 a month
	// obl sets the current payments (v in thousand rubles) and no new loan, so that the tiers are read off the current cover
	obl := func(v int64) func(*Input) {
		return func(in *Input) { in.MonthlyObligations, in.Amount = v*1000*rub, 0 }
	}
	debt := func(v int64) func(*Input) {
		return withBKI(integrations.BKIReport{TotalDebt: v * 1000 * rub, Score: 700}) // v in thousand rubles
	}
	fin := func(f integrations.Finance) func(*Input) { return func(in *Input) { in.EGRUL.Finance = &f } }
	stm := func(st integrations.Statement) func(*Input) { return func(in *Input) { in.EGRUL.Statement = &st } }
	cases := []tc{
		// 1. DSCR 15 / 10 / 5 / 0 by cash flow / current payments
		{"dscr 1.5 and above", "dscr", obl(400), 15},
		{"dscr 1.25 to 1.5", "dscr", obl(450), 10},
		{"dscr 1.0 to 1.25", "dscr", obl(530), 5},
		{"dscr below 1", "dscr", obl(700), 0},
		{"dscr without payments", "dscr", obl(0), 15},
		// 2. debt / EBITDA 10 / 5 / 0 (EBITDA a year = 7 200 000)
		{"debt below 1.5", "debt_ebitda", debt(10_000), 10},
		{"debt 1.5 to 3", "debt_ebitda", debt(20_000), 5},
		{"debt above 3", "debt_ebitda", debt(23_000), 0},
		{"no debt", "debt_ebitda", debt(0), 10},
		// 3. profitability and stability 10 / 5 / 0
		{"stable and profitable", "profitability", fin(integrations.Finance{GrossMargin: 0.35, CurrentRatio: 1.5}), 10},
		{"stable, margin below the industry", "profitability", fin(integrations.Finance{GrossMargin: 0.10, CurrentRatio: 1.5}), 5},
		{"loss-making", "profitability", fin(integrations.Finance{GrossMargin: -0.02, CurrentRatio: 1.5}), 0},
		{"falling revenue", "profitability", func(in *Input) {
			in.MonthlyRevenue = []int64{3000 * rub, 3000 * rub, 3000 * rub, 3000 * rub, 3000 * rub, 3000 * rub, 2500 * rub, 2500 * rub, 2500 * rub, 2500 * rub, 2500 * rub, 2500 * rub}
		}, 0},
		{"unstable revenue", "profitability", func(in *Input) {
			in.MonthlyRevenue = []int64{1000 * rub, 4000 * rub, 900 * rub, 4100 * rub, 1000 * rub, 4000 * rub, 900 * rub, 4100 * rub, 1000 * rub, 4000 * rub, 900 * rub, 4100 * rub}
		}, 0},
		// 4. current ratio 5 / 3 / 0
		{"liquidity comfortable", "liquidity", fin(integrations.Finance{CurrentRatio: 1.5}), 5},
		{"liquidity borderline", "liquidity", fin(integrations.Finance{CurrentRatio: 1.2}), 3},
		{"liquidity low", "liquidity", fin(integrations.Finance{CurrentRatio: 0.9}), 0},
		// 5. overdue in 36 months 15 / 10 / 5 / 0
		{"no overdue", "overdue", withBKI(integrations.BKIReport{Score: 700}), 15},
		{"technical overdue", "overdue", withBKI(integrations.BKIReport{Score: 700, MaxOverdueDays: 5}), 10},
		{"overdue up to 30 days", "overdue", withBKI(integrations.BKIReport{Score: 700, MaxOverdueDays: 30}), 5},
		{"overdue above 30 days", "overdue", withBKI(integrations.BKIReport{Score: 700, MaxOverdueDays: 31}), 0},
		// 6. current obligations 10 / 5 / 0
		{"no obligations", "obligations", func(in *Input) {
			in.MonthlyObligations = 0
			in.BKI = &integrations.BKIReport{HasHistory: true, Score: 700}
		}, 10},
		{"served on time", "obligations", withBKI(integrations.BKIReport{Score: 700, ActiveLoans: 1}), 5},
		{"restructured", "obligations", withBKI(integrations.BKIReport{Score: 700, ActiveLoans: 1, Restructured: true}), 0},
		// 7. age of the business: more than 5 years 10, 3 to 5 years 7, 1 to 3 years 4, below a year 0
		{"age above 5 years", "company_age", func(in *Input) { in.CompanyAgeMonths = 61 }, 10},
		{"age 5 years", "company_age", func(in *Input) { in.CompanyAgeMonths = 60 }, 7},
		{"age 3 years", "company_age", func(in *Input) { in.CompanyAgeMonths = 36 }, 7},
		{"age 1 to 3 years", "company_age", func(in *Input) { in.CompanyAgeMonths = 24 }, 4},
		{"age below a year", "company_age", func(in *Input) { in.CompanyAgeMonths = 11 }, 0},
		// 8. industry 5 / 3 / 0
		{"low risk industry", "okved_risk", func(in *Input) { in.OKVEDGroup = 1 }, 5},
		{"medium risk industry", "okved_risk", func(in *Input) { in.OKVEDGroup = 2 }, 3},
		{"high risk industry", "okved_risk", func(in *Input) { in.OKVEDGroup = 3 }, 0},
		// 9. anomalies in the statement 5 / 2 / 0
		{"no anomalies", "statement", stm(integrations.Statement{NOIMargin: 0.3, Anomaly: 0}), 5},
		{"minor anomalies", "statement", stm(integrations.Statement{NOIMargin: 0.3, Anomaly: 1}), 2},
		{"major anomalies", "statement", stm(integrations.Statement{NOIMargin: 0.3, Anomaly: 2}), 0},
		// 10. reputation and court cases 5 / 2 / 0
		{"clean reputation", "reputation", func(in *Input) { in.EGRUL.Litigation = 0 }, 5},
		{"minor disputes", "reputation", func(in *Input) { in.EGRUL.Litigation = 1 }, 2},
		{"major suits", "reputation", func(in *Input) { in.EGRUL.Litigation = 2 }, 0},
		// 11. collateral 5 / 2 / 0
		{"full collateral", "collateral", func(in *Input) { in.Collateral = CollateralFull }, 5},
		{"partial collateral", "collateral", func(in *Input) { in.Collateral = CollateralPartial }, 2},
		{"no collateral", "collateral", func(in *Input) { in.Collateral = CollateralNone }, 0},
		{"collateral not stated", "collateral", func(in *Input) { in.Collateral = "" }, 0},
		// 12. purpose and amount 5 / 2 / 0
		{"purpose backed by a document", "purpose", func(in *Input) { in.PurposeDocs = true }, 5},
		{"purpose only stated", "purpose", func(in *Input) {
			in.PurposeDocs = false
			in.PurposeText = "Закупка сырья перед сезоном"
		}, 2},
		{"purpose not given", "purpose", func(in *Input) { in.PurposeDocs = false; in.PurposeText = "" }, 0},
	}
	for _, c := range cases {
		in := baseInput()
		in.BKI = &integrations.BKIReport{HasHistory: true, Score: 780}
		c.set(&in)
		if got := factor(t, Score(in, cfg), c.key).Points; got != c.want {
			t.Errorf("%s: %s got %d points, want %d", c.name, c.key, got, c.want)
		}
	}
}

func TestCriteria_MaximumsFollowTheScorecard(t *testing.T) {
	want := map[string]int{"dscr": 15, "debt_ebitda": 10, "profitability": 10, "liquidity": 5, "overdue": 15, "obligations": 10, "company_age": 10, "okved_risk": 5, "statement": 5, "reputation": 5, "collateral": 5, "purpose": 5}
	res := Score(baseInput(), DefaultConfig())
	for _, f := range res.Factors {
		if f.Max != want[f.Key] {
			t.Errorf("%s: max %d, want %d", f.Key, f.Max, want[f.Key])
		}
	}
}

func TestGrades_HardStopIsAlwaysGradeD(t *testing.T) {
	in := baseInput()
	in.BKI = &integrations.BKIReport{HasHistory: true, Score: 800, MaxOverdueDays: 120}
	res := Score(in, DefaultConfig())
	if res.Grade != GradeD || len(res.HardStops) == 0 {
		t.Fatalf("a hard stop ends scoring with grade D whatever the points: %s %v", res.Grade, res.HardStops)
	}
}

func TestQuality_MapsGradesTo590P(t *testing.T) {
	cases := map[Grade][2]string{
		GradeA: {"I–II", "0–20 %"},
		GradeB: {"II–III", "1–50 %"},
		GradeC: {"III–IV", "21–100 %"},
		GradeD: {"IV–V", "51–100 %"},
	}
	for g, want := range cases {
		q := QualityOf(g)
		if !strings.HasPrefix(q.Category, want[0]) || q.Reserve != want[1] {
			t.Errorf("%s: %+v, want %v", g, q, want)
		}
	}
}

// A criterion without data is marked, and the points it could have given are counted, so that a decision
// can tell "the borrower is bad" from "we know too little".
func TestMissingPoints_CountWhatCouldNotBeJudged(t *testing.T) {
	cfg := DefaultConfig()
	if res := Score(baseInput(), cfg); res.MissingPoints != 0 {
		t.Fatalf("a complete input misses nothing: %d", res.MissingPoints)
	}
	in := baseInput()
	in.EGRUL.Finance, in.EGRUL.Statement = nil, nil
	in.BKI = &integrations.BKIReport{HasHistory: false}
	res := Score(in, cfg)
	// statement: dscr 15, debt/ebitda 10, anomalies 5; accounts: profitability 10, liquidity 5; bureau: overdue 15, obligations 10
	want := 15 + 10 + 5 + 10 + 5 + 15 + 10
	if res.MissingPoints != want {
		t.Fatalf("missing points %d, want %d", res.MissingPoints, want)
	}
	for _, f := range res.Factors {
		if f.NoData != (f.Points == 0 && strings.HasPrefix(f.Note, "Нет данных")) {
			t.Errorf("%s: NoData=%v note %q", f.Key, f.NoData, f.Note)
		}
	}
	if res.MissingPoints > 100 {
		t.Fatal("cannot miss more than the whole scorecard")
	}
}

func TestMissingPoints_CollateralNoneIsAFactNotAGap(t *testing.T) {
	in := baseInput()
	in.Collateral = CollateralNone
	for _, f := range Score(in, DefaultConfig()).Factors {
		if f.Key == "collateral" && f.NoData {
			t.Fatal("a stated absence of collateral is a fact, not missing data")
		}
	}
}

// DSCR counts the payment of the new loan: the same company covers a bigger loan worse.
func TestDSCR_CountsThePaymentOfTheNewLoan(t *testing.T) {
	cfg := DefaultConfig()
	mk := func(amount int64) Result {
		in := baseInput() // cash flow 600 000 a month, current payments 100 000
		in.MonthlyObligations = 400_000 * rub
		in.Amount, in.TermMonths = amount*rub, 12
		return Score(in, cfg)
	}
	none, small, big := mk(0), mk(1_000_000), mk(5_000_000)
	pts := func(r Result) int { return factor(t, r, "dscr").Points }
	if pts(none) != 15 || pts(small) != 5 || pts(big) != 0 {
		t.Fatalf("1.50 without the loan, 1.22 with 1 million, 0.70 with 5 million: got %d / %d / %d points", pts(none), pts(small), pts(big))
	}
	if !(none.Score > small.Score && small.Score > big.Score) {
		t.Fatalf("a bigger loan must not score better: %d / %d / %d", none.Score, small.Score, big.Score)
	}
	if got := factor(t, small, "dscr").Note; !strings.Contains(got, "платежом по заявке") {
		t.Fatalf("the note says that the new payment is counted: %q", got)
	}
}

// A sole trader has no published accounts, but the bank statement tells what is left of the receipts.
func TestCashFlow_ComesFromTheStatement(t *testing.T) {
	cfg := DefaultConfig()
	in := baseInput() // receipts 2 000 000 a month
	in.EGRUL.Statement = &integrations.Statement{NOIMargin: 0.20}
	if got, want := Score(in, cfg).MonthlyCashFlow, int64(float64(2_000_000*rub)*0.20); got != want {
		t.Fatalf("cash flow %d, want %d (receipts minus write-offs on the statement)", got, want)
	}
	in.EGRUL.Finance = nil // no accounts: the cash flow does not change
	if got := Score(in, cfg).MonthlyCashFlow; got != int64(float64(2_000_000*rub)*0.20) {
		t.Fatalf("the accounts are not needed for the cash flow: %d", got)
	}
}

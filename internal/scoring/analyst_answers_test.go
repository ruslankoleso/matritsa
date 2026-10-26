package scoring

import (
	"strings"
	"testing"

	"matritsa/internal/integrations"
)

// Answers of the analyst on the marked list of questions (26 October): thresholds and rules of the criteria.

func alternating(share float64) []int64 { // 12 months around 1 000 000, the spread (coefficient of variation) equals share
	r := make([]int64, 12)
	for i := range r {
		d := 1 + share
		if i%2 == 1 {
			d = 1 - share
		}
		r[i] = int64(float64(1_000_000*rub) * d)
	}
	return r
}

func points(t *testing.T, in Input, key string) Factor {
	t.Helper()
	return factor(t, Score(in, DefaultConfig()), key)
}

// 4.1: a technical overdue is up to 3 days.
func TestOverdue_TechnicalIsUpTo3Days(t *testing.T) {
	for days, want := range map[int]int{3: 10, 4: 5, 30: 5, 31: 0} {
		in := baseInput()
		in.BKI = &integrations.BKIReport{HasHistory: true, Score: 780, MaxOverdueDays: days}
		if got := points(t, in, "overdue").Points; got != want {
			t.Errorf("%d days: %d points, want %d", days, got, want)
		}
	}
}

// 4.2: two overdues or more in the last 12 months lower the tier by one; overdues older than 36 months are ignored.
func TestOverdue_CountInTheLastYearLowersTheTier_AndOldOnesAreIgnored(t *testing.T) {
	bki := func(o ...integrations.Overdue) *integrations.BKIReport {
		b := &integrations.BKIReport{HasHistory: true, Score: 780, Overdues: o}
		for _, x := range o {
			if x.MonthsAgo < 36 && x.Days > b.MaxOverdueDays {
				b.MaxOverdueDays = x.Days
			}
		}
		return b
	}
	cases := []struct {
		name string
		b    *integrations.BKIReport
		want int
	}{
		{"one technical overdue", bki(integrations.Overdue{Days: 2, MonthsAgo: 4}), 10},
		{"two technical in the last 12 months: a tier lower", bki(integrations.Overdue{Days: 2, MonthsAgo: 4}, integrations.Overdue{Days: 3, MonthsAgo: 9}), 5},
		{"two technical, one of them a year and a half ago: no count effect", bki(integrations.Overdue{Days: 2, MonthsAgo: 4}, integrations.Overdue{Days: 3, MonthsAgo: 18}), 10},
		{"two of 20 days in the last 12 months: 5 goes to 0", bki(integrations.Overdue{Days: 20, MonthsAgo: 2}, integrations.Overdue{Days: 10, MonthsAgo: 11}), 0},
		{"a 20 days overdue older than 36 months is not counted at all", bki(integrations.Overdue{Days: 20, MonthsAgo: 40}), 15},
		{"old and a fresh technical one", bki(integrations.Overdue{Days: 80, MonthsAgo: 41}, integrations.Overdue{Days: 2, MonthsAgo: 3}), 10},
	}
	for _, c := range cases {
		in := baseInput()
		in.BKI = c.b
		if got := points(t, in, "overdue").Points; got != c.want {
			t.Errorf("%s: %d points, want %d", c.name, got, c.want)
		}
	}
}

func TestOverdueHistory_BuildsTheRecordTheRulesRead(t *testing.T) {
	h := integrations.OverdueHistory(20, 2)
	if len(h) != 2 || h[0].Days != 20 {
		t.Fatalf("%+v", h)
	}
	n := 0
	for _, o := range h {
		if o.MonthsAgo < 12 {
			n++
		}
	}
	if n != 2 || len(integrations.OverdueHistory(0, 0)) != 0 {
		t.Fatalf("two of the last year, none for a clean history: %+v", h)
	}
}

// 8.2: stable up to 10%, moderate up to 25%, above 25% strong fluctuations, 0 points.
func TestProfitability_StableUpTo10_ModerateUpTo25(t *testing.T) {
	for share, want := range map[float64]int{0.08: 10, 0.12: 5, 0.24: 5, 0.27: 0} {
		in := baseInput()
		in.MonthlyRevenue = alternating(share)
		f := points(t, in, "profitability")
		if f.Points != want {
			t.Errorf("spread %.0f%%: %d points, want %d (%s)", share*100, f.Points, want, f.Note)
		}
	}
}

// 3.2 and 3.3: the debt is the bureau debt, leasing and the requested amount; 3.1: above 4 earns 0 and stops the machine refusing.
func TestDebtEBITDA_CountsLeasingAndTheNewLoan(t *testing.T) {
	cfg := DefaultConfig()
	mk := func(debt, leasing, amount int64) Result { // the cash flow is 600 000 a month: EBITDA 7 200 000 a year
		in := baseInput()
		in.BKI = &integrations.BKIReport{HasHistory: true, Score: 780, TotalDebt: debt * 1000 * rub, LeasingDebt: leasing * 1000 * rub}
		in.Amount = amount * 1000 * rub
		return Score(in, cfg)
	}
	if f := factor(t, mk(0, 0, 1000), "debt_ebitda"); f.Points != 10 {
		t.Errorf("no debt but the requested million: 0,14 → 10, got %+v", f)
	}
	if f := factor(t, mk(8000, 0, 1000), "debt_ebitda"); f.Points != 10 {
		t.Errorf("9 million over 7,2: 1,25 → 10, got %+v", f)
	}
	if f := factor(t, mk(8000, 2500, 1000), "debt_ebitda"); f.Points != 5 || !strings.Contains(f.Note, "лизинг") {
		t.Errorf("with leasing 11,5 over 7,2: 1,6 → 5, and the note names leasing, got %+v", f)
	}
	if f := factor(t, mk(20000, 0, 1000), "debt_ebitda"); f.Points != 5 {
		t.Errorf("21 million: 2,9 → 5, got %+v", f)
	}
	if f := factor(t, mk(23000, 0, 1000), "debt_ebitda"); f.Points != 0 {
		t.Errorf("24 million: 3,3 → 0, got %+v", f)
	}
	if r := mk(23000, 0, 1000); r.HighLeverage {
		t.Errorf("3,3 is not above 4")
	}
	if r := mk(29000, 0, 1000); !r.HighLeverage || factor(t, r, "debt_ebitda").Points != 0 {
		t.Errorf("30 million over 7,2 is 4,2: 0 points and the flag, got %+v", r.HighLeverage)
	}
	// a bigger request weighs more: the same borrower is not the same loan
	if a, b := factor(t, mk(8000, 0, 1000), "debt_ebitda"), factor(t, mk(8000, 0, 20000), "debt_ebitda"); !(b.Points < a.Points) {
		t.Errorf("a bigger loan must not look better: %d vs %d", a.Points, b.Points)
	}
}

// 6.1: a class C loan is considered only with collateral.
func TestCollateralCounted_FlagsWhetherCollateralWasAccepted(t *testing.T) {
	in := baseInput()
	if Score(in, DefaultConfig()).CollateralCounted {
		t.Fatal("no collateral stated")
	}
	in.CollateralKind, in.CollateralValue, in.CollateralDoc = CollateralRealEstate, 5_000_000*rub, true
	if !Score(in, DefaultConfig()).CollateralCounted {
		t.Fatal("a pledge with a document counts")
	}
}

// 9.1 and 9.2: no balance sheet: the balance on the account against the monthly expenses; retail: 1,2.
func TestLiquidity_FromTheStatementWithoutABalanceSheet(t *testing.T) {
	cases := []struct {
		cover float64
		want  int
	}{{1.5, 5}, {1.2, 3}, {0.8, 0}}
	for _, c := range cases {
		in := baseInput()
		in.EGRUL.Finance = nil
		in.EGRUL.Statement.BalanceCover = c.cover
		f := points(t, in, "liquidity")
		if f.Points != c.want || f.NoData || !strings.Contains(f.Note, "выпис") {
			t.Errorf("cover %.1f months: %+v, want %d points from the statement", c.cover, f, c.want)
		}
	}
	in := baseInput()
	in.EGRUL.Finance, in.EGRUL.Statement = nil, nil
	if f := points(t, in, "liquidity"); !f.NoData {
		t.Errorf("neither a balance sheet nor a statement: %+v", f)
	}
}

func TestLiquidity_RetailComfortableFrom1_2(t *testing.T) {
	in := baseInput()
	in.EGRUL.Finance = &integrations.Finance{GrossMargin: 0.3, CurrentRatio: 1.25}
	in.OKVED = "47.11"
	if f := points(t, in, "liquidity"); f.Points != 5 || !strings.Contains(f.Note, "торговл") {
		t.Errorf("retail at 1,25: comfortable, %+v", f)
	}
	in.OKVED = "10.71"
	if f := points(t, in, "liquidity"); f.Points != 3 {
		t.Errorf("another industry at 1,25: borderline, %+v", f)
	}
	in.OKVED, in.EGRUL.Finance.CurrentRatio = "46.90", 1.1
	if f := points(t, in, "liquidity"); f.Points != 3 {
		t.Errorf("retail at 1,1: borderline, %+v", f)
	}
}

// 11: a large suit is above 5% of the annual revenue; only cases where the company or its head is the defendant,
// of the last 3 years.
func TestReputation_SuitsByTheAnalystsRules(t *testing.T) {
	annual := int64(12_000_000 * rub) // baseInput earns 2 000 000 a month: 24 million a year
	annual = 24_000_000 * rub
	_ = annual
	cases := []struct {
		name  string
		cases []integrations.CourtCase
		want  int
	}{
		{"no suits", nil, 5},
		{"a small one against the company: 4% of the year", []integrations.CourtCase{{Defendant: true, Amount: 960_000 * rub, YearsAgo: 1}}, 2},
		{"a large one: 6% of the year", []integrations.CourtCase{{Defendant: true, Amount: 1_440_000 * rub, YearsAgo: 1}}, 0},
		{"exactly 5% is not above", []integrations.CourtCase{{Defendant: true, Amount: 1_200_000 * rub, YearsAgo: 1}}, 2},
		{"a large suit where the company is the plaintiff", []integrations.CourtCase{{Defendant: false, Amount: 5_000_000 * rub, YearsAgo: 1}}, 5},
		{"a large suit older than 3 years", []integrations.CourtCase{{Defendant: true, Amount: 5_000_000 * rub, YearsAgo: 3.5}}, 5},
		{"a small recent and a large old one", []integrations.CourtCase{{Defendant: true, Amount: 500_000 * rub, YearsAgo: 1}, {Defendant: true, Amount: 5_000_000 * rub, YearsAgo: 4}}, 2},
	}
	for _, c := range cases {
		in := baseInput()
		in.EGRUL.Cases = c.cases
		if got := points(t, in, "reputation").Points; got != c.want {
			t.Errorf("%s: %d points, want %d", c.name, got, c.want)
		}
	}
}

func TestCasesOfLevel_BuildsSuitsOfTheRightWeight(t *testing.T) {
	annual := int64(24_000_000 * rub)
	if got := integrations.CasesOfLevel(0, annual); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	minor, major := integrations.CasesOfLevel(1, annual), integrations.CasesOfLevel(2, annual)
	if len(minor) != 1 || !minor[0].Defendant || float64(minor[0].Amount)/float64(annual) > 0.05 {
		t.Errorf("a minor suit is within 5%%: %+v", minor)
	}
	if len(major) != 1 || !major[0].Defendant || float64(major[0].Amount)/float64(annual) <= 0.05 {
		t.Errorf("a major suit is above 5%%: %+v", major)
	}
}

// 12: collateral. Full: the pledge value after the discount covers the loan; a guarantee counts as partial only;
// without a document nothing counts.
func TestCollateral_ByTheAnalystsRules(t *testing.T) {
	cfg := DefaultConfig()
	mk := func(kind string, value, amount int64, doc bool) Factor {
		in := baseInput()
		in.CollateralKind, in.CollateralValue, in.CollateralDoc = kind, value*rub, doc
		in.Amount = amount * rub
		return factor(t, Score(in, cfg), "collateral")
	}
	if f := mk(CollateralRealEstate, 10_000_000, 5_000_000, true); f.Points != 5 {
		t.Errorf("a flat of 10 million with a 30%% discount covers 5 million: %+v", f)
	}
	if f := mk(CollateralRealEstate, 6_000_000, 5_000_000, true); f.Points != 2 {
		t.Errorf("6 million less 30%% is 4,2: covers part: %+v", f)
	}
	if f := mk(CollateralTransport, 10_000_000, 5_000_000, true); f.Points != 5 {
		t.Errorf("a vehicle of 10 million with a 50%% discount covers exactly: %+v", f)
	}
	if f := mk(CollateralGoods, 8_000_000, 5_000_000, true); f.Points != 2 {
		t.Errorf("goods 8 million less 50%% is 4: part: %+v", f)
	}
	if f := mk(CollateralGuarantee, 0, 5_000_000, true); f.Points != 2 || !strings.Contains(f.Note, "оручительство") {
		t.Errorf("a guarantee of the owner is partial collateral at most: %+v", f)
	}
	if f := mk(CollateralRealEstate, 50_000_000, 5_000_000, false); f.Points != 0 || !strings.Contains(f.Note, "документ") || f.NoData {
		t.Errorf("without a document the collateral is not counted, but that is not a gap in the data: %+v", f)
	}
	if f := mk(CollateralNone, 0, 5_000_000, false); f.Points != 0 || f.NoData {
		t.Errorf("%+v", f)
	}
	if f := mk(CollateralEquipment, 0, 5_000_000, true); f.Points != 0 {
		t.Errorf("a pledge without a value counts for nothing: %+v", f)
	}
}

func TestCollateral_BiggerLoanNeedsMoreCover(t *testing.T) {
	cfg := DefaultConfig()
	in := baseInput()
	in.CollateralKind, in.CollateralValue, in.CollateralDoc = CollateralEquipment, 10_000_000*rub, true
	in.Amount = 6_000_000 * rub // 10 million less 35% is 6,5 million
	a := factor(t, Score(in, cfg), "collateral")
	in.Amount = 8_000_000 * rub
	b := factor(t, Score(in, cfg), "collateral")
	if a.Points != 5 || b.Points != 2 {
		t.Fatalf("the same pledge covers 6 million and not 8: %d vs %d", a.Points, b.Points)
	}
}

// 13.2 and 13.3: the amount in the confirming document within ±10% of the request; the analyst's verdict decides.
func TestPurpose_DocumentAmountWithinTenPercent(t *testing.T) {
	cfg := DefaultConfig()
	mk := func(docAmount int64) Factor {
		in := baseInput()
		in.PurposeDocs = true
		in.Amount = 1_000_000 * rub
		in.PurposeDocAmount = docAmount * rub
		return factor(t, Score(in, cfg), "purpose")
	}
	for amount, want := range map[int64]int{0: 5, 1_000_000: 5, 1_100_000: 5, 900_000: 5, 1_100_001: 2, 800_000: 2} {
		if got := mk(amount); got.Points != want {
			t.Errorf("document of %d against the request of 1 000 000: %d points, want %d (%s)", amount, got.Points, want, got.Note)
		}
	}
	if f := mk(800_000); !strings.Contains(f.Note, "10") {
		t.Errorf("the note names the tolerance: %q", f.Note)
	}
}

func TestPurpose_TheAnalystsVerdictDecides(t *testing.T) {
	cfg := DefaultConfig()
	for verdict, want := range map[string]int{PurposeConfirmed: 5, PurposeClarify: 2, PurposeRejected: 0} {
		for _, docs := range []bool{false, true} {
			in := baseInput()
			in.PurposeDocs, in.PurposeVerdict = docs, verdict
			f := factor(t, Score(in, cfg), "purpose")
			if f.Points != want || f.NoData || !strings.Contains(f.Note, "сотрудник") {
				t.Errorf("verdict %s (documents %v): %+v, want %d points and the note that the employee decided", verdict, docs, f, want)
			}
		}
	}
	in := baseInput()
	in.PurposeText, in.PurposeVerdict = "", PurposeConfirmed
	if f := factor(t, Score(in, cfg), "purpose"); f.Points != 5 {
		t.Errorf("the employee's word stands even where the form is empty: %+v", f)
	}
}

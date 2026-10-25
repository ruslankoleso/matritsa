package scoring

import (
	"math"
	"strings"
	"testing"

	"matritsa/internal/integrations"
)

const rub = int64(100) // kopecks per ruble

func flatRevenue(monthly int64) []int64 {
	r := make([]int64, 12)
	for i := range r {
		r[i] = monthly
	}
	return r
}

func goodFinance() *integrations.Finance {
	return &integrations.Finance{GrossMargin: 0.38, CurrentRatio: 1.8}
}

// goodStatement is the bank statement: the net flow (receipts minus operating write-offs) is 30% of revenue.
func goodStatement() *integrations.Statement {
	return &integrations.Statement{NOIMargin: 0.30}
}

func activeCompany() integrations.EGRULRecord {
	return integrations.EGRULRecord{Status: "active", Finance: goodFinance(), Statement: goodStatement()}
}

func goodBKI() *integrations.BKIReport {
	return &integrations.BKIReport{HasHistory: true, Score: 780, MaxOverdueDays: 0}
}

func baseInput() Input {
	return Input{
		CompanyAgeMonths:   72,
		PurposeText:        "Закупка сырья перед сезоном",
		MonthlyRevenue:     flatRevenue(2_000_000 * rub),
		MonthlyObligations: 100_000 * rub,
		BKI:                goodBKI(),
		EGRUL:              activeCompany(),
		Amount:             1_000_000 * rub,
		TermMonths:         12,
		RateBP:             1650,
		OKVEDGroup:         1,
		LGD:                0.6,
	}
}

func TestScore_ReliableBakery_GradeA(t *testing.T) {
	res := Score(baseInput(), DefaultConfig())
	if res.Grade != GradeA {
		t.Fatalf("want grade A, got %s (score %d)", res.Grade, res.Score)
	}
	if len(res.HardStops) != 0 {
		t.Fatalf("unexpected hard stops: %v", res.HardStops)
	}
}

// No data, no points: what the bank cannot see about a company does not earn it anything.
func TestScore_NoData_MeansNoPoints_WithANoteSayingSo(t *testing.T) {
	cfg := DefaultConfig()
	noHistory := baseInput()
	noHistory.BKI = &integrations.BKIReport{HasHistory: false}
	res := Score(noHistory, cfg)
	for _, key := range []string{"overdue", "debt_ebitda", "obligations"} {
		if f := factor(t, res, key); f.Points != 0 || !strings.HasPrefix(f.Note, "Нет данных") {
			t.Errorf("%s without a bureau history: want 0 points and a note, got %d %q", key, f.Points, f.Note)
		}
	}
	if res.BKIScore != 0 {
		t.Fatalf("a missing history has no bureau score: %d", res.BKIScore)
	}

	// no accounts (a sole trader does not publish them): the statement still gives the cash flow
	noAccounts := baseInput()
	noAccounts.EGRUL.Finance = nil
	res = Score(noAccounts, cfg)
	for _, key := range []string{"profitability", "liquidity"} {
		if f := factor(t, res, key); f.Points != 0 || !strings.HasPrefix(f.Note, "Нет данных") {
			t.Errorf("%s without the accounts: want 0 points and a note, got %d %q", key, f.Points, f.Note)
		}
	}
	for _, key := range []string{"dscr", "debt_ebitda", "statement"} {
		if f := factor(t, res, key); f.NoData {
			t.Errorf("%s comes from the statement and needs no accounts: %+v", key, f)
		}
	}
	// no statement: no cash flow to cover payments with
	noStatement := baseInput()
	noStatement.EGRUL.Statement = nil
	res = Score(noStatement, cfg)
	for _, key := range []string{"dscr", "debt_ebitda", "statement"} {
		if f := factor(t, res, key); f.Points != 0 || !strings.HasPrefix(f.Note, "Нет данных") {
			t.Errorf("%s without the statement: want 0 points and a note, got %d %q", key, f.Points, f.Note)
		}
	}
	if res.MonthlyCashFlow <= 0 {
		t.Error("the affordability limit still needs a cash flow: without the statement it is estimated by the industry")
	}

	noRevenue := baseInput()
	noRevenue.MonthlyRevenue = nil
	res = Score(noRevenue, cfg)
	for _, key := range []string{"dscr", "profitability"} {
		if f := factor(t, res, key); f.Points != 0 {
			t.Errorf("%s without revenue: got %d points", key, f.Points)
		}
	}

	nothing := baseInput()
	nothing.PurposeText, nothing.OKVEDGroup = "", 0
	res = Score(nothing, cfg)
	for _, key := range []string{"purpose", "okved_risk"} {
		if f := factor(t, res, key); f.Points != 0 || !strings.HasPrefix(f.Note, "Нет данных") {
			t.Errorf("%s with nothing given: got %d %q", key, f.Points, f.Note)
		}
	}
}

func TestScore_NoData_IsNotAFactOfGoodStanding(t *testing.T) {
	good := Score(baseInput(), DefaultConfig())
	young := baseInput()
	young.BKI = &integrations.BKIReport{HasHistory: false}
	young.MonthlyObligations = 0
	if Score(young, DefaultConfig()).Score >= good.Score {
		t.Fatal("a company with no history must not score as well as one with a clean history")
	}
}

func TestScore_HardStop_Liquidated(t *testing.T) {
	in := baseInput()
	in.EGRUL.Status = "liquidated"
	res := Score(in, DefaultConfig())
	if len(res.HardStops) == 0 {
		t.Fatal("liquidated company must have a hard stop")
	}
}

func TestScore_HardStop_Bankrupt(t *testing.T) {
	in := baseInput()
	in.EGRUL.Bankrupt = true
	if len(Score(in, DefaultConfig()).HardStops) == 0 {
		t.Fatal("bankrupt company must have a hard stop")
	}
}

// The analyst's stop list: a disqualified director and false data in the registry end the scoring too.
func TestScore_HardStop_DisqualifiedDirector_AndUnreliableRegistryData(t *testing.T) {
	cases := map[string]func(*Input){
		"disqualified director": func(in *Input) { in.EGRUL.DirectorDisqualified = true },
		"unreliable data":       func(in *Input) { in.EGRUL.UnreliableData = true },
	}
	for name, set := range cases {
		in := baseInput()
		set(&in)
		res := Score(in, DefaultConfig())
		if len(res.HardStops) != 1 || res.Grade != GradeD {
			t.Errorf("%s: want one hard stop and grade D, got %v %s", name, res.HardStops, res.Grade)
		}
		if f := factor(t, res, "reputation"); f.Points != 0 {
			t.Errorf("%s: the reputation earns nothing, got %d", name, f.Points)
		}
	}
	in := baseInput()
	in.EGRUL.DirectorDisqualified, in.EGRUL.UnreliableData = true, true
	if got := Score(in, DefaultConfig()).HardStops; len(got) != 2 {
		t.Errorf("both reasons are named: %v", got)
	}
}

func TestScore_HardStop_Overdue91Days(t *testing.T) {
	in := baseInput()
	in.BKI = &integrations.BKIReport{HasHistory: true, Score: 700, MaxOverdueDays: 91}
	res := Score(in, DefaultConfig())
	if len(res.HardStops) == 0 {
		t.Fatal("overdue > 90 days must be a hard stop")
	}
	in.BKI.MaxOverdueDays = 90
	if len(Score(in, DefaultConfig()).HardStops) != 0 {
		t.Fatal("overdue of exactly 90 days is not a hard stop")
	}
}

func TestScore_ZeroRevenue_NoPanic_GradeD(t *testing.T) {
	for name, rev := range map[string][]int64{"nil": nil, "zeros": make([]int64, 12)} {
		in := baseInput()
		in.MonthlyRevenue = rev
		res := Score(in, DefaultConfig())
		if res.Grade != GradeD {
			t.Errorf("%s: want grade D for no revenue, got %s (score %d)", name, res.Grade, res.Score)
		}
		if res.MaxAffordableAmount != 0 {
			t.Errorf("%s: want zero affordable amount, got %d", name, res.MaxAffordableAmount)
		}
	}
}

func TestFactorsSumToScore(t *testing.T) {
	res := Score(baseInput(), DefaultConfig())
	sum, max := 0, 0
	for _, f := range res.Factors {
		sum += f.Points
		max += f.Max
	}
	if sum != res.Score {
		t.Fatalf("factors sum %d != score %d", sum, res.Score)
	}
	if max != 100 {
		t.Fatalf("max points must total 100, got %d", max)
	}
}

func TestScore_GradeBoundaries(t *testing.T) {
	cases := map[int]Grade{100: GradeA, 80: GradeA, 79: GradeB, 50: GradeB, 49: GradeC, 30: GradeC, 29: GradeD, 0: GradeD}
	for score, want := range cases {
		if got := GradeFor(score, DefaultConfig()); got != want {
			t.Errorf("GradeFor(%d)=%s want %s", score, got, want)
		}
	}
}

func TestScore_IndebtedCompany_LowerThanReliable(t *testing.T) {
	in := baseInput()
	in.MonthlyObligations = 1_200_000 * rub
	in.BKI = &integrations.BKIReport{HasHistory: true, Score: 600, MaxOverdueDays: 20}
	got := Score(in, DefaultConfig())
	reliable := Score(baseInput(), DefaultConfig())
	if got.Grade == GradeA || got.Score >= reliable.Score {
		t.Fatalf("an indebted company must score below a reliable one and not reach A: %s/%d vs %s/%d", got.Grade, got.Score, reliable.Grade, reliable.Score)
	}
	if factor(t, got, "dscr").Points != 0 {
		t.Fatalf("cash flow that does not cover the payments earns no DSCR points, got %d", factor(t, got, "dscr").Points)
	}
}

func TestMonthlyPayment_Annuity(t *testing.T) {
	// 1 000 000 RUB, 12% annual (1200 bp), 12 months -> 88 848.79 RUB
	got := MonthlyPayment(1_000_000*rub, 1200, 12)
	want := int64(8_884_879)
	if got < want-100 || got > want+100 {
		t.Fatalf("annuity payment = %d kopecks, want %d ±100", got, want)
	}
}

func TestMonthlyPayment_ZeroRateAndTerm(t *testing.T) {
	if got := MonthlyPayment(1_200_000*rub, 0, 12); got != 100_000*rub {
		t.Fatalf("zero rate: got %d", got)
	}
	if got := MonthlyPayment(1_000*rub, 1200, 0); got != 0 {
		t.Fatalf("zero term must not panic and return 0, got %d", got)
	}
}

func TestMaxAffordable_KeepsLoadAtMost030(t *testing.T) {
	in := baseInput()
	res := Score(in, DefaultConfig())
	max := res.MaxAmountByLoad
	if max <= 0 {
		t.Fatal("expected positive affordable amount")
	}
	avg := int64(2_000_000) * rub
	pay := MonthlyPayment(max, in.RateBP, in.TermMonths)
	load := float64(in.MonthlyObligations+pay) / float64(avg)
	if load > 0.301 {
		t.Fatalf("load at max affordable = %.3f, want <= 0.30", load)
	}
	// and a bit more should break the threshold
	pay2 := MonthlyPayment(max+max/20, in.RateBP, in.TermMonths)
	if float64(in.MonthlyObligations+pay2)/float64(avg) <= 0.30 {
		t.Fatal("max affordable is not tight")
	}
}

func factor(t *testing.T, r Result, key string) Factor {
	t.Helper()
	for _, f := range r.Factors {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("factor %q not found in %+v", key, r.Factors)
	return Factor{}
}

func TestScore_TwelveCriteria_NoRequestSizeFactors(t *testing.T) {
	res := Score(baseInput(), DefaultConfig())
	want := []string{"dscr", "debt_ebitda", "profitability", "liquidity", "overdue", "obligations", "company_age", "okved_risk", "statement", "reputation", "collateral", "purpose"}
	if len(res.Factors) != len(want) {
		t.Fatalf("want %d criteria, got %d", len(want), len(res.Factors))
	}
	for i, f := range res.Factors {
		if f.Key != want[i] {
			t.Errorf("criterion %d: want %s, got %s", i+1, want[i], f.Key)
		}
		if f.Key == "debt_load" || f.Key == "amount_to_revenue" {
			t.Fatalf("factor %s depends on the size of the request and must not be part of the score", f.Key)
		}
	}
}

// The grade belongs to the loan: a bigger and longer request covers worse and never scores better.
func TestScore_BiggerAndLongerRequest_NeverScoresBetter_AndCanFallToAnotherClass(t *testing.T) {
	cfg := DefaultConfig()
	small := baseInput()
	small.Amount, small.TermMonths = 2_000_000*rub, 30
	big := baseInput()
	big.Amount, big.TermMonths = 20_000_000*rub, 40
	a, b := Score(small, cfg), Score(big, cfg)
	if b.Score > a.Score {
		t.Fatalf("a bigger loan scored better: %d vs %d", b.Score, a.Score)
	}
	if !(b.Score < a.Score) || a.Grade == b.Grade {
		t.Fatalf("20 million must lose the DSCR points and the class (A for 2 million): %s/%d vs %s/%d", a.Grade, a.Score, b.Grade, b.Score)
	}
	// only DSCR moves: nothing else in the score depends on the size
	for i, f := range a.Factors {
		if f.Key != "dscr" && f.Points != b.Factors[i].Points {
			t.Errorf("%s changed with the size of the request: %d vs %d", f.Key, f.Points, b.Factors[i].Points)
		}
	}
}

func TestExpectedLoss_Formula(t *testing.T) {
	in := baseInput() // grade A: PD 1% a year, LGD 60%, 1 000 000 RUB for 12 months
	res := Score(in, DefaultConfig())
	if res.PD != 0.01 || res.LGD != 0.6 || res.EAD != 1_000_000*rub {
		t.Fatalf("inputs of the formula wrong: PD=%v LGD=%v EAD=%d", res.PD, res.LGD, res.EAD)
	}
	if res.CumulativePD < 0.0099 || res.CumulativePD > 0.0101 {
		t.Fatalf("cumulative PD over one year must equal the annual PD, got %v", res.CumulativePD)
	}
	want := int64(6_000) * rub // 1% x 60% x 1 000 000
	if res.ExpectedLoss < want-rub || res.ExpectedLoss > want+rub {
		t.Fatalf("EL = PD x LGD x EAD: want %d, got %d", want, res.ExpectedLoss)
	}
	if res.ELRate < 0.0059 || res.ELRate > 0.0061 {
		t.Fatalf("EL rate = cumulative PD x LGD, got %v", res.ELRate)
	}
}

func TestExpectedLoss_LongerTermAndBiggerAmountCostMore(t *testing.T) {
	short := baseInput()
	long := baseInput()
	long.TermMonths = 40
	bigger := baseInput()
	bigger.Amount = 5_000_000 * rub
	a, b, c := Score(short, DefaultConfig()), Score(long, DefaultConfig()), Score(bigger, DefaultConfig())
	if !(b.CumulativePD > a.CumulativePD && b.ExpectedLoss > a.ExpectedLoss) {
		t.Fatalf("a longer term must raise the cumulative PD and the EL: %v/%d vs %v/%d", a.CumulativePD, a.ExpectedLoss, b.CumulativePD, b.ExpectedLoss)
	}
	if !(c.ExpectedLoss > 4*a.ExpectedLoss) {
		t.Fatalf("EL must scale with the exposure: %d vs %d", a.ExpectedLoss, c.ExpectedLoss)
	}
}

func TestExpectedLoss_WorseClassMeansHigherPD(t *testing.T) {
	good := Score(baseInput(), DefaultConfig())
	bad := baseInput()
	bad.BKI = &integrations.BKIReport{HasHistory: true, Score: 500, MaxOverdueDays: 60}
	bad.MonthlyObligations = 700_000 * rub
	worse := Score(bad, DefaultConfig())
	if !(worse.PD > good.PD) {
		t.Fatalf("a weaker borrower must have a higher PD: %v vs %v (grades %s, %s)", good.PD, worse.PD, good.Grade, worse.Grade)
	}
}

func TestPremium(t *testing.T) {
	cfg := DefaultConfig()
	want := map[Grade]int{GradeA: 0, GradeB: 180, GradeC: 540} // (PD_g − PD_A) × LGD 0.6, in basis points
	for g, bp := range want {
		got, ok := Premium(g, 0.6, cfg)
		if !ok || got != bp {
			t.Errorf("grade %s: got %d,%v want %d", g, got, ok, bp)
		}
	}
	if _, ok := Premium(GradeD, 0.6, cfg); ok {
		t.Error("grade D gets no offer, so no premium")
	}
}

func TestPremium_SecuredProductIsCheaperThanUnsecured(t *testing.T) {
	cfg := DefaultConfig()
	secured, _ := Premium(GradeC, 0.4, cfg)
	unsecured, _ := Premium(GradeC, 0.75, cfg)
	if !(secured < unsecured) {
		t.Fatalf("a lower LGD must mean a lower premium: %d vs %d", secured, unsecured)
	}
}

func TestResult_PremiumFollowsGradeAndProduct(t *testing.T) {
	res := Score(baseInput(), DefaultConfig())
	if res.RiskPremiumBP != 0 {
		t.Fatalf("grade A pays no premium on top of the base rate, got %d", res.RiskPremiumBP)
	}
}

func TestScore_RepaymentExperience_IsMeasuredAgainstTheRequestedAmount(t *testing.T) {
	in := baseInput()
	in.BKI = &integrations.BKIReport{HasHistory: true, Score: 780, ClosedLoans: 3, TotalRepaid: 3_000_000 * 100}
	in.Amount = 2_000_000 * 100
	res := Score(in, DefaultConfig())
	if res.RepaidTotal != 3_000_000*100 || res.RepaidCoverage != 1.5 {
		t.Fatalf("repaid %d coverage %v", res.RepaidTotal, res.RepaidCoverage)
	}
	in.Amount = 6_000_000 * 100
	if c := Score(in, DefaultConfig()).RepaidCoverage; c != 0.5 {
		t.Fatalf("the same history covers a bigger request less: %v", c)
	}
}

func TestScore_RepaymentExperience_DoesNotChangeTheGrade(t *testing.T) {
	a := baseInput()
	a.BKI = &integrations.BKIReport{HasHistory: true, Score: 780, TotalRepaid: 0}
	b := a
	b.BKI = &integrations.BKIReport{HasHistory: true, Score: 780, TotalRepaid: 50_000_000 * 100}
	if Score(a, DefaultConfig()).Grade != Score(b, DefaultConfig()).Grade {
		t.Fatal("the class describes the borrower; the repaid amount is checked next to the request, not inside the class")
	}
}

func TestScore_RepaymentExperience_SafeWithoutBureauOrAmount(t *testing.T) {
	in := baseInput()
	in.BKI = nil
	if c := Score(in, DefaultConfig()).RepaidCoverage; c != 0 {
		t.Fatalf("no bureau: %v", c)
	}
	in.BKI = &integrations.BKIReport{HasHistory: true, TotalRepaid: 100}
	in.Amount = 0
	if c := Score(in, DefaultConfig()).RepaidCoverage; c != 0 {
		t.Fatalf("no amount must not divide by zero: %v", c)
	}
}

func TestMetrics_CarryTheRepaymentExperience(t *testing.T) {
	in := baseInput()
	in.BKI = &integrations.BKIReport{HasHistory: true, TotalRepaid: 1_000_000 * 100}
	in.Amount = 2_000_000 * 100
	m := Score(in, DefaultConfig()).Metrics()
	if m.RepaidTotal != 1_000_000*100 || m.RepaidCoverage != 0.5 {
		t.Fatalf("%+v", m)
	}
}

// ---- two coverage measures: the share of revenue that goes to payments (load) and the cash flow cover (DSCR) ----

func TestDSCR_IsCashFlowOverAllPayments(t *testing.T) {
	cfg := DefaultConfig()
	in := baseInput() // revenue 2 000 000, group 1, obligations 100 000
	res := Score(in, cfg)
	wantCash := int64(float64(2_000_000*rub) * in.EGRUL.Statement.NOIMargin) // receipts minus write-offs on the statement
	if res.MonthlyCashFlow != wantCash {
		t.Fatalf("cash flow %d, want %d", res.MonthlyCashFlow, wantCash)
	}
	want := float64(wantCash) / float64(in.MonthlyObligations+res.Payment)
	if math.Abs(res.DSCR-want) > 1e-9 {
		t.Fatalf("DSCR %v, want %v", res.DSCR, want)
	}
}

func TestDSCR_WithoutTheStatement_MarginDependsOnTheIndustryGroup(t *testing.T) {
	cfg := DefaultConfig()
	low, high := baseInput(), baseInput()
	low.EGRUL.Statement, high.EGRUL.Statement = nil, nil
	low.OKVEDGroup, high.OKVEDGroup = 3, 1
	if Score(low, cfg).MonthlyCashFlow >= Score(high, cfg).MonthlyCashFlow {
		t.Fatal("a riskier industry keeps less cash from the same revenue")
	}
}

func TestMaxAmountByDSCR_KeepsCoverAtTheMinimum(t *testing.T) {
	cfg := DefaultConfig()
	in := baseInput()
	res := Score(in, cfg)
	if res.MaxAmountByDSCR <= 0 {
		t.Fatal("expected a positive limit")
	}
	pay := MonthlyPayment(res.MaxAmountByDSCR, in.RateBP, in.TermMonths)
	if got := float64(res.MonthlyCashFlow) / float64(in.MonthlyObligations+pay); got < cfg.MinDSCR-0.001 {
		t.Fatalf("cover at the limit is %.3f, below %.2f", got, cfg.MinDSCR)
	}
	over := MonthlyPayment(res.MaxAmountByDSCR+res.MaxAmountByDSCR/20, in.RateBP, in.TermMonths)
	if float64(res.MonthlyCashFlow)/float64(in.MonthlyObligations+over) >= cfg.MinDSCR {
		t.Fatal("the limit is not tight")
	}
}

func TestMaxAffordable_IsTheStricterOfTheTwoLimits(t *testing.T) {
	res := Score(baseInput(), DefaultConfig())
	want := res.MaxAmountByLoad
	if res.MaxAmountByDSCR < want {
		want = res.MaxAmountByDSCR
	}
	if res.MaxAffordableAmount != want {
		t.Fatalf("affordable %d, by load %d, by DSCR %d", res.MaxAffordableAmount, res.MaxAmountByLoad, res.MaxAmountByDSCR)
	}
}

func TestBothMeasuresMatter_EachCanBeTheBindingOne(t *testing.T) {
	cfg := DefaultConfig()
	// a thin margin makes the cash flow the limit although the load looks fine
	thin := baseInput()
	thin.EGRUL.Statement = nil
	thin.OKVEDGroup = 3
	thin.MonthlyObligations = 400_000 * rub
	r := Score(thin, cfg)
	if !(r.MaxAmountByDSCR < r.MaxAmountByLoad) {
		t.Fatalf("expected DSCR to bind: load %d, dscr %d", r.MaxAmountByLoad, r.MaxAmountByDSCR)
	}
	// no existing debt and a good margin: the load share is the stricter one
	lean := baseInput()
	lean.EGRUL.Statement = nil
	lean.MonthlyObligations = 0
	lean.OKVEDGroup = 1
	lean.MonthlyRevenue = flatRevenue(1_000_000 * rub)
	cfg.OperatingMargin = map[int]float64{1: 0.9, 2: 0.9, 3: 0.9}
	r = Score(lean, cfg)
	if !(r.MaxAmountByLoad < r.MaxAmountByDSCR) {
		t.Fatalf("expected the load to bind: load %d, dscr %d", r.MaxAmountByLoad, r.MaxAmountByDSCR)
	}
}

func TestDSCR_SafeWithoutRevenue(t *testing.T) {
	in := baseInput()
	in.MonthlyRevenue = nil
	r := Score(in, DefaultConfig())
	if r.DSCR != 0 || r.MonthlyCashFlow != 0 || r.MaxAmountByDSCR != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestMetrics_CarryTheCoverMeasures(t *testing.T) {
	r := Score(baseInput(), DefaultConfig())
	m := r.Metrics()
	if m.DSCR != r.DSCR || m.MonthlyCashFlow != r.MonthlyCashFlow || m.MaxAmountByLoad != r.MaxAmountByLoad || m.MaxAmountByDSCR != r.MaxAmountByDSCR {
		t.Fatalf("%+v", m)
	}
}

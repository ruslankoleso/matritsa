package scoring

import (
	"math"
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

func activeCompany() integrations.EGRULRecord {
	return integrations.EGRULRecord{Status: "active"}
}

func goodBKI() *integrations.BKIReport {
	return &integrations.BKIReport{HasHistory: true, Score: 780, MaxOverdueDays: 0}
}

func baseInput() Input {
	return Input{
		CompanyAgeMonths:   60,
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

func TestScore_NoHistory_GetsEightBKIPoints(t *testing.T) {
	in := baseInput()
	in.BKI = &integrations.BKIReport{HasHistory: false}
	res := Score(in, DefaultConfig())
	f := factor(t, res, "bki_score")
	if f.Points != 10 {
		t.Fatalf("want 10 points for no history, got %d", f.Points)
	}
	od := factor(t, res, "overdue")
	if od.Points != 15 {
		t.Fatalf("no history must not be punished on overdue, got %d", od.Points)
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

func TestScore_ZeroRevenue_NoPanic_GradeE(t *testing.T) {
	for name, rev := range map[string][]int64{"nil": nil, "zeros": make([]int64, 12)} {
		in := baseInput()
		in.MonthlyRevenue = rev
		res := Score(in, DefaultConfig())
		if res.Grade != GradeE {
			t.Errorf("%s: want grade E for no revenue, got %s (score %d)", name, res.Grade, res.Score)
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
	cases := map[int]Grade{100: GradeA, 80: GradeA, 79: GradeB, 65: GradeB, 64: GradeC, 50: GradeC, 49: GradeD, 35: GradeD, 34: GradeE, 0: GradeE}
	for score, want := range cases {
		if got := GradeFor(score); got != want {
			t.Errorf("GradeFor(%d)=%s want %s", score, got, want)
		}
	}
}

func TestScore_IndebtedCompany_LowerThanReliable(t *testing.T) {
	in := baseInput()
	in.MonthlyObligations = 1_200_000 * rub
	in.BKI = &integrations.BKIReport{HasHistory: true, Score: 600, MaxOverdueDays: 20}
	got := Score(in, DefaultConfig())
	if got.Grade == GradeA || got.Grade == GradeB {
		t.Fatalf("indebted company should be C or worse, got %s", got.Grade)
	}
	if factor(t, got, "existing_debt").Points != 0 {
		t.Fatalf("existing debt points should be zero, got %d", factor(t, got, "existing_debt").Points)
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

func TestMaxAffordable_KeepsLoadAtMost035(t *testing.T) {
	in := baseInput()
	res := Score(in, DefaultConfig())
	max := res.MaxAmountByLoad
	if max <= 0 {
		t.Fatal("expected positive affordable amount")
	}
	avg := int64(2_000_000) * rub
	pay := MonthlyPayment(max, in.RateBP, in.TermMonths)
	load := float64(in.MonthlyObligations+pay) / float64(avg)
	if load > 0.351 {
		t.Fatalf("load at max affordable = %.3f, want <= 0.35", load)
	}
	// and a bit more should break the threshold
	pay2 := MonthlyPayment(max+max/20, in.RateBP, in.TermMonths)
	if float64(in.MonthlyObligations+pay2)/float64(avg) <= 0.35 {
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

func TestScore_SixBorrowerFactors_NoRequestFactors(t *testing.T) {
	res := Score(baseInput(), DefaultConfig())
	if len(res.Factors) != 6 {
		t.Fatalf("want 6 factors, got %d", len(res.Factors))
	}
	for _, f := range res.Factors {
		if f.Key == "debt_load" || f.Key == "amount_to_revenue" {
			t.Fatalf("factor %s depends on the request and must not be part of the borrower score", f.Key)
		}
	}
}

func TestScore_GradeDoesNotDependOnRequestSize(t *testing.T) {
	small := baseInput()
	small.Amount, small.TermMonths = 2_000_000*rub, 30
	big := baseInput()
	big.Amount, big.TermMonths = 20_000_000*rub, 40
	a, b := Score(small, DefaultConfig()), Score(big, DefaultConfig())
	if a.Score != b.Score || a.Grade != b.Grade {
		t.Fatalf("the same borrower must get the same class: %s/%d vs %s/%d", a.Grade, a.Score, b.Grade, b.Score)
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
	want := map[Grade]int{GradeA: 0, GradeB: 120, GradeC: 300, GradeD: 660} // (PD_g − PD_A) × LGD 0.6, in basis points
	for g, bp := range want {
		got, ok := Premium(g, 0.6, cfg)
		if !ok || got != bp {
			t.Errorf("grade %s: got %d,%v want %d", g, got, ok, bp)
		}
	}
	if _, ok := Premium(GradeE, 0.6, cfg); ok {
		t.Error("grade E gets no offer, so no premium")
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
	wantCash := int64(float64(2_000_000*rub) * cfg.OperatingMargin[1])
	if res.MonthlyCashFlow != wantCash {
		t.Fatalf("cash flow %d, want %d", res.MonthlyCashFlow, wantCash)
	}
	want := float64(wantCash) / float64(in.MonthlyObligations+res.Payment)
	if math.Abs(res.DSCR-want) > 1e-9 {
		t.Fatalf("DSCR %v, want %v", res.DSCR, want)
	}
}

func TestDSCR_MarginDependsOnTheIndustryGroup(t *testing.T) {
	cfg := DefaultConfig()
	low, high := baseInput(), baseInput()
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
	thin.OKVEDGroup = 3
	thin.MonthlyObligations = 400_000 * rub
	r := Score(thin, cfg)
	if !(r.MaxAmountByDSCR < r.MaxAmountByLoad) {
		t.Fatalf("expected DSCR to bind: load %d, dscr %d", r.MaxAmountByLoad, r.MaxAmountByDSCR)
	}
	// no existing debt and a good margin: the load share is the stricter one
	lean := baseInput()
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

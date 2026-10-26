package decision

import (
	"math"
	"strings"
	"testing"

	"matritsa/internal/products"
	"matritsa/internal/scoring"
)

func prod(t *testing.T, id products.ID) products.Product {
	t.Helper()
	p, ok := products.Get(id)
	if !ok {
		t.Fatalf("product %s missing", id)
	}
	return p
}

func result(g scoring.Grade, affordable int64, stops ...string) scoring.Result {
	return scoring.Result{Grade: g, MaxAffordableAmount: affordable, HardStops: stops, RepaidCoverage: 2, BKIScore: 780, CollateralCounted: true}
}

func TestDecide_GradeA_Small_AutoOffer(t *testing.T) {
	d := Decide(result(scoring.GradeA, 5_000_000*rub), prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != AutoOffer || d.Offer == nil {
		t.Fatalf("want auto offer, got %+v", d)
	}
	if d.Offer.Amount != 1_000_000*rub || d.Offer.TermMonths != 12 || d.Offer.RateBP != 1650 {
		t.Fatalf("offer must keep requested terms with base rate: %+v", d.Offer)
	}
	if d.Offer.MonthlyPayment <= 0 {
		t.Fatal("offer needs a monthly payment")
	}
}

func TestDecide_GradeA_AboveAutoLimit_Review(t *testing.T) {
	d := Decide(result(scoring.GradeA, 9_000_000*rub), prod(t, products.WorkingCapital), 5_000_000*rub, 12)
	if d.Outcome != Review {
		t.Fatalf("want review above auto limit, got %s", d.Outcome)
	}
	if d.Recommended == nil || d.Recommended.Amount != 5_000_000*rub {
		t.Fatalf("recommended should keep the requested amount for grade A: %+v", d.Recommended)
	}
}

func TestDecide_GradeC_ReviewWithReducedCounter(t *testing.T) {
	d := Decide(result(scoring.GradeC, 800_000*rub), prod(t, products.WorkingCapital), 2_000_000*rub, 12)
	if d.Outcome != Review || d.Recommended == nil {
		t.Fatalf("want review with recommendation, got %+v", d)
	}
	if d.Recommended.Amount >= 2_000_000*rub || d.Recommended.Amount > 800_000*rub {
		t.Fatalf("counter amount must be reduced to what is affordable: %d", d.Recommended.Amount)
	}
	if d.Recommended.Amount < 300_000*rub {
		t.Fatalf("counter must stay within product minimum: %d", d.Recommended.Amount)
	}
}

func TestDecide_GradeD_Reject_NamesWhereThePointsWereLost(t *testing.T) {
	r := result(scoring.GradeD, 100_000*rub)
	r.Score = 22
	r.Factors = []scoring.Factor{{Title: "Просрочки за 36 месяцев", Max: 15}, {Title: "Возраст бизнеса", Max: 10, Points: 4}, {Title: "Обеспечение", Max: 5}}
	d := Decide(r, prod(t, products.Express), 100_000*rub, 6)
	if d.Outcome != Reject || len(d.Reasons) == 0 || d.Offer != nil {
		t.Fatalf("grade D must reject with reasons: %+v", d)
	}
	text := d.Reasons[0]
	if !strings.Contains(text, "22 баллов") || !strings.Contains(text, "Просрочки за 36 месяцев; Обеспечение") || strings.Contains(text, "Возраст") {
		t.Fatalf("the reason says the score and the criteria without points: %q", text)
	}
}

func TestDecide_GradeA_LowBureauScore_GoesToReview(t *testing.T) {
	r := result(scoring.GradeA, 5_000_000*rub)
	r.BKIScore = 600
	d := Decide(r, prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != Review {
		t.Fatalf("a bureau score below the threshold must not give an automatic offer: %s", d.Outcome)
	}
	if !strings.Contains(strings.Join(d.Reasons, " "), "Балл бюро кредитных историй 600") {
		t.Fatalf("the reason must name the bureau score: %v", d.Reasons)
	}
	r.BKIScore = scoring.DefaultConfig().MinAutoBKIScore
	if d := Decide(r, prod(t, products.WorkingCapital), 1_000_000*rub, 12); d.Outcome != AutoOffer {
		t.Fatalf("exactly the threshold must pass: %s", d.Outcome)
	}
}

func TestDecide_HardStop_RejectsEvenForGradeA(t *testing.T) {
	d := Decide(result(scoring.GradeA, 5_000_000*rub, "Компания ликвидирована"), prod(t, products.Express), 100_000*rub, 6)
	if d.Outcome != Reject {
		t.Fatalf("hard stop must reject: %+v", d)
	}
	if d.Reasons[0] != "Компания ликвидирована" {
		t.Fatalf("hard stop reason must come first: %v", d.Reasons)
	}
}

func TestDecide_AffordableBelowMin_Reject(t *testing.T) {
	d := Decide(result(scoring.GradeC, 100_000*rub), prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != Reject {
		t.Fatalf("affordable below product minimum must reject: %+v", d)
	}
}

func TestDecide_RateIncludesRiskPremium(t *testing.T) {
	d := Decide(result(scoring.GradeB, 5_000_000*rub), prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != Review || d.Recommended == nil {
		t.Fatalf("grade B goes to review: %+v", d)
	}
	if d.Recommended.RateBP != 1650+180 {
		t.Fatalf("rate = base + premium, got %d", d.Recommended.RateBP)
	}
}

// A good class is not enough for an automatic offer: the borrower must also have repaid loans
// of a size that makes the requested amount familiar to the bank.
func withCoverage(r scoring.Result, c float64) scoring.Result {
	r.RepaidCoverage = c
	return r
}

func TestDecide_GradeA_WithoutRepaymentExperience_GoesToReview(t *testing.T) {
	d := Decide(withCoverage(result(scoring.GradeA, 5_000_000*rub), 0.3), prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != Review {
		t.Fatalf("a small repaid history must not give an automatic offer: %s", d.Outcome)
	}
	if d.Recommended == nil || d.Recommended.Amount != 1_000_000*rub {
		t.Fatalf("the recommendation keeps the requested amount, it is only a hint to the analyst: %+v", d.Recommended)
	}
	found := false
	for _, r := range d.Reasons {
		found = found || strings.Contains(r, "опыт погашения")
	}
	if !found {
		t.Fatalf("the reason must name the repayment experience: %v", d.Reasons)
	}
}

func TestDecide_GradeA_NoHistoryAtAll_GoesToReview(t *testing.T) {
	if d := Decide(withCoverage(result(scoring.GradeA, 5_000_000*rub), 0), prod(t, products.WorkingCapital), 1_000_000*rub, 12); d.Outcome != Review {
		t.Fatalf("no repaid loans: %s", d.Outcome)
	}
}

func TestDecide_CoverageAtTheThresholdIsEnough(t *testing.T) {
	min := scoring.DefaultConfig().MinRepaidCoverage
	if d := Decide(withCoverage(result(scoring.GradeA, 5_000_000*rub), min), prod(t, products.WorkingCapital), 1_000_000*rub, 12); d.Outcome != AutoOffer {
		t.Fatalf("exactly the threshold must pass: %s", d.Outcome)
	}
}

func TestDecide_LowCoverage_IsNotedForTheAnalystInEveryReview(t *testing.T) {
	low := Decide(withCoverage(result(scoring.GradeC, 800_000*rub), 0.2), prod(t, products.WorkingCapital), 2_000_000*rub, 12)
	ok := Decide(withCoverage(result(scoring.GradeC, 800_000*rub), 1.2), prod(t, products.WorkingCapital), 2_000_000*rub, 12)
	has := func(d Decision) bool {
		for _, r := range d.Reasons {
			if strings.Contains(r, "опыт погашения") {
				return true
			}
		}
		return false
	}
	if !has(low) || has(ok) {
		t.Fatalf("the hint appears only when the experience is short: low=%v ok=%v", low.Reasons, ok.Reasons)
	}
}

func TestDecide_TooBigRequest_NamesTheLimitThatBinds(t *testing.T) {
	has := func(d Decision, part string) bool {
		for _, r := range d.Reasons {
			if strings.Contains(r, part) {
				return true
			}
		}
		return false
	}
	byCash := result(scoring.GradeC, 800_000*rub)
	byCash.MaxAmountByLoad, byCash.MaxAmountByDSCR = 2_000_000*rub, 800_000*rub
	if d := Decide(byCash, prod(t, products.WorkingCapital), 2_000_000*rub, 12); !has(d, "денежного потока") {
		t.Fatalf("the cash flow is the limit: %v", d.Reasons)
	}
	byLoad := result(scoring.GradeC, 800_000*rub)
	byLoad.MaxAmountByLoad, byLoad.MaxAmountByDSCR = 800_000*rub, 2_000_000*rub
	if d := Decide(byLoad, prod(t, products.WorkingCapital), 2_000_000*rub, 12); !has(d, "долговую нагрузку") {
		t.Fatalf("the load share is the limit: %v", d.Reasons)
	}
}

// Decisions 7.1 and 7.2 of the analyst: a gap in the data is never a reason for the machine to decide, in either
// direction. The application goes to a person with the list of gaps and with what the system would have done.
func withGap(r scoring.Result) scoring.Result {
	r.Factors = append(r.Factors, scoring.Factor{Title: "Просрочки за 36 месяцев", Max: 15, NoData: true})
	r.MissingPoints = 15
	return r
}

func TestDecide_AGapInTheData_StopsAnAutomaticOffer(t *testing.T) {
	r := withGap(result(scoring.GradeA, 5_000_000*rub))
	d := Decide(r, prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != Review || d.Offer != nil || d.Recommended == nil || d.Recommended.Amount != 1_000_000*rub {
		t.Fatalf("an automatic offer becomes a recommendation for the analyst: %+v", d)
	}
	joined := strings.Join(d.Reasons, " ")
	if !strings.Contains(joined, "Просрочки за 36 месяцев") || !strings.Contains(joined, "одобрила бы автоматически") {
		t.Fatalf("the reasons name the gap and what the system would have done: %v", d.Reasons)
	}
}

func TestDecide_AGapInTheData_StopsAnAutomaticRefusal_WhateverTheClass(t *testing.T) {
	// even a class D that stays D with every missing point added is not refused by the machine
	r := withGap(result(scoring.GradeD, 100_000*rub))
	r.Score = 5
	d := Decide(r, prod(t, products.Express), 100_000*rub, 6)
	if d.Outcome != Review || d.Offer != nil {
		t.Fatalf("missing data must not produce an automatic refusal: %+v", d)
	}
	joined := strings.Join(d.Reasons, " ")
	if !strings.Contains(joined, "Просрочки за 36 месяцев") || !strings.Contains(joined, "отказала бы") {
		t.Fatalf("the reasons name the gap and the refusal the system would have given: %v", d.Reasons)
	}
	// the same for a refusal by affordability
	low := withGap(result(scoring.GradeC, 100_000*rub))
	if d := Decide(low, prod(t, products.WorkingCapital), 1_000_000*rub, 12); d.Outcome != Review {
		t.Fatalf("a refusal by affordability with a gap also goes to a person: %+v", d)
	}
}

func TestDecide_AGapInTheData_DoesNotSoftenAStopFactor(t *testing.T) {
	r := withGap(result(scoring.GradeD, 0, "Компания ликвидирована"))
	if d := Decide(r, prod(t, products.Express), 100_000*rub, 6); d.Outcome != Reject {
		t.Fatalf("a stop factor is a condition of admission, not a gap in the data: %+v", d)
	}
}

func TestDecide_ClassD_WithoutAnyGap_IsRefused(t *testing.T) {
	r := result(scoring.GradeD, 5_000_000*rub)
	r.Score = 12
	if d := Decide(r, prod(t, products.Express), 100_000*rub, 6); d.Outcome != Reject {
		t.Fatalf("%+v", d)
	}
}

func TestDecide_NoGap_NothingChanges(t *testing.T) {
	d := Decide(result(scoring.GradeA, 5_000_000*rub), prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != AutoOffer {
		t.Fatalf("%+v", d)
	}
}

// Decision 6.1 of the analyst: a class C loan, the high risk, is considered only with collateral.
func TestDecide_ClassC_WithoutCollateral_IsRefused_WithItIsReviewed(t *testing.T) {
	r := result(scoring.GradeC, 5_000_000*rub)
	r.CollateralCounted = false
	d := Decide(r, prod(t, products.WorkingCapital), 1_000_000*rub, 12)
	if d.Outcome != Reject || !strings.Contains(strings.Join(d.Reasons, " "), "обеспечени") {
		t.Fatalf("class C without collateral: %+v", d)
	}
	r.CollateralCounted = true
	if d := Decide(r, prod(t, products.WorkingCapital), 1_000_000*rub, 12); d.Outcome != Review {
		t.Fatalf("class C with collateral goes to the analyst: %+v", d)
	}
	b := result(scoring.GradeB, 5_000_000*rub)
	b.CollateralCounted = false
	if d := Decide(b, prod(t, products.WorkingCapital), 1_000_000*rub, 12); d.Outcome != Review {
		t.Fatalf("class B needs no collateral: %+v", d)
	}
}

func TestDecide_ClassC_TheRecommendedAmountScoredAsC_NeedsCollateralToo(t *testing.T) {
	requested := result(scoring.GradeB, 800_000*rub)
	requested.MaxAmountByLoad, requested.MaxAmountByDSCR = 2_000_000*rub, 800_000*rub
	rescore := func(int64) scoring.Result {
		r := result(scoring.GradeC, 800_000*rub)
		r.CollateralCounted = false
		return r
	}
	if d := DecideRescoring(requested, prod(t, products.WorkingCapital), 4_000_000*rub, 12, rescore); d.Outcome != Reject {
		t.Fatalf("the smaller loan is class C and has no collateral: %+v", d)
	}
}

// Decision 3.1 of the analyst: Debt/EBITDA above the stop earns nothing and the machine does not refuse.
func TestDecide_HighLeverage_StopsAnAutomaticRefusal_ButNotAnAutomaticOffer(t *testing.T) {
	r := result(scoring.GradeD, 100_000*rub)
	r.Score, r.HighLeverage, r.DebtEBITDA = 20, true, 4.6
	d := Decide(r, prod(t, products.Express), 100_000*rub, 6)
	if d.Outcome != Review || d.Offer != nil {
		t.Fatalf("a refusal on top of a leverage above 4 goes to a person: %+v", d)
	}
	joined := strings.Join(d.Reasons, " ")
	if !strings.Contains(joined, "Долг / EBITDA 4,6") || !strings.Contains(joined, "отказала бы") {
		t.Fatalf("the reasons name the leverage and the refusal the system would have given: %v", d.Reasons)
	}
	a := result(scoring.GradeA, 5_000_000*rub)
	a.HighLeverage, a.DebtEBITDA = true, 4.2
	if d := Decide(a, prod(t, products.WorkingCapital), 1_000_000*rub, 12); d.Outcome != AutoOffer {
		t.Fatalf("the leverage alone does not take an automatic offer away: %+v", d)
	}
	stop := result(scoring.GradeD, 0, "Компания ликвидирована")
	stop.HighLeverage = true
	if d := Decide(stop, prod(t, products.Express), 100_000*rub, 6); d.Outcome != Reject {
		t.Fatalf("a stop factor still refuses: %+v", d)
	}
}

// Decision 6.4 of the analyst: rate = risk-free rate + risk premium (PD x LGD) + operating costs. The base rate of
// each product is made of the three for class A, and every other class pays its own PD x LGD.
func TestPricing_RateIsRiskFreePlusPDxLGDPlusOperatingCost(t *testing.T) {
	cfg := scoring.DefaultConfig()
	for _, p := range products.Catalog() {
		for _, g := range []scoring.Grade{scoring.GradeA, scoring.GradeB, scoring.GradeC} {
			offer, ok := MakeOffer(p, g, 1_000_000*rub, 12, "auto")
			if !ok {
				t.Fatalf("%s %s: no offer", p.ID, g)
			}
			want := cfg.RiskFreeBP + int(math.Round(cfg.PD[g]*p.LGD*10000)) + p.OperatingCostBP
			if offer.RateBP != want {
				t.Errorf("%s class %s: rate %d, risk-free %d + PD x LGD + operating cost %d = %d", p.ID, g, offer.RateBP, cfg.RiskFreeBP, p.OperatingCostBP, want)
			}
		}
	}
}

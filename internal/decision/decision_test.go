package decision

import (
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
	return scoring.Result{Grade: g, MaxAffordableAmount: affordable, HardStops: stops, RepaidCoverage: 2, BKIScore: 780}
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

// A class D that comes from missing data is not a reason to refuse by itself: a person looks at it.
func TestDecide_GradeD_FromMissingData_GoesToAPerson(t *testing.T) {
	r := result(scoring.GradeD, 5_000_000*rub)
	r.Score, r.MissingPoints = 24, 25 // 24 + 25 would be class C
	r.Factors = []scoring.Factor{{Title: "Просрочки за 36 месяцев", Max: 15, NoData: true}, {Title: "Обеспечение", Max: 5}}
	d := Decide(r, prod(t, products.Express), 100_000*rub, 6)
	if d.Outcome != Review || d.Offer != nil {
		t.Fatalf("missing data must not produce an automatic refusal: %+v", d)
	}
	if !strings.Contains(strings.Join(d.Reasons, " "), "Просрочки за 36 месяцев") {
		t.Fatalf("the reason names the criteria without data: %v", d.Reasons)
	}
}

func TestDecide_GradeD_WithEnoughData_StillRejects(t *testing.T) {
	r := result(scoring.GradeD, 5_000_000*rub)
	r.Score, r.MissingPoints = 12, 10 // even with the missing points it stays below class C
	if d := Decide(r, prod(t, products.Express), 100_000*rub, 6); d.Outcome != Reject {
		t.Fatalf("%+v", d)
	}
}

// The class belongs to the loan: a request too big for the business can fall to class D, while a smaller amount
// offered instead is scored again and gets its own class and rate.
func TestDecide_TooBigRequestScoredD_StillGetsACounterOfferScoredOnItsOwn(t *testing.T) {
	requested := result(scoring.GradeD, 800_000*rub)
	requested.Score, requested.MissingPoints = 28, 0
	requested.MaxAmountByLoad, requested.MaxAmountByDSCR = 2_000_000*rub, 800_000*rub
	asked := []int64{}
	rescore := func(amount int64) scoring.Result {
		asked = append(asked, amount)
		r := result(scoring.GradeB, 800_000*rub)
		r.Score = 62
		return r
	}
	d := DecideRescoring(requested, prod(t, products.WorkingCapital), 4_000_000*rub, 12, rescore)
	if d.Outcome != Review || d.Recommended == nil || d.Recommended.Amount != 800_000*rub {
		t.Fatalf("a counter-offer instead of a refusal: %+v", d)
	}
	if len(asked) != 1 || asked[0] != 800_000*rub {
		t.Fatalf("the recommended amount is scored on its own: %v", asked)
	}
	if d.Recommended.RateBP != 1650+180 {
		t.Fatalf("the rate follows the class of the recommended amount (B), got %d", d.Recommended.RateBP)
	}
	if !strings.Contains(strings.Join(d.Reasons, " "), "оценена отдельно: класс B, 62 баллов") {
		t.Fatalf("the reasons say that the amount was scored separately: %v", d.Reasons)
	}
}

func TestDecide_ClassD_EvenForTheSmallerAmount_IsARefusal(t *testing.T) {
	requested := result(scoring.GradeD, 800_000*rub)
	requested.Score = 20
	rescore := func(int64) scoring.Result {
		r := result(scoring.GradeD, 800_000*rub)
		r.Score = 24
		return r
	}
	if d := DecideRescoring(requested, prod(t, products.WorkingCapital), 4_000_000*rub, 12, rescore); d.Outcome != Reject {
		t.Fatalf("%+v", d)
	}
	if d := Decide(requested, prod(t, products.WorkingCapital), 4_000_000*rub, 12); d.Outcome != Reject {
		t.Fatalf("without rescoring the class of the request stands: %+v", d)
	}
}

func TestDecide_NoRescoreWhenTheRequestFits(t *testing.T) {
	called := false
	rescore := func(int64) scoring.Result { called = true; return result(scoring.GradeA, 0) }
	DecideRescoring(result(scoring.GradeB, 5_000_000*rub), prod(t, products.WorkingCapital), 1_000_000*rub, 12, rescore)
	if called {
		t.Fatal("an affordable request is scored once")
	}
}

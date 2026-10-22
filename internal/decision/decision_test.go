package decision

import (
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
	return scoring.Result{Grade: g, MaxAffordableAmount: affordable, HardStops: stops}
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

func TestDecide_GradeE_Reject(t *testing.T) {
	d := Decide(result(scoring.GradeE, 100_000*rub), prod(t, products.Express), 100_000*rub, 6)
	if d.Outcome != Reject || len(d.Reasons) == 0 || d.Offer != nil {
		t.Fatalf("grade E must reject with reasons: %+v", d)
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
	if d.Recommended.RateBP != 1650+120 {
		t.Fatalf("rate = base + premium, got %d", d.Recommended.RateBP)
	}
}

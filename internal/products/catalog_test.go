package products

import "testing"

func TestCatalog_FourProducts(t *testing.T) {
	c := Catalog()
	if len(c) != 4 {
		t.Fatalf("want 4 products, got %d", len(c))
	}
	wc, ok := Get(WorkingCapital)
	if !ok {
		t.Fatal("working_capital missing")
	}
	if wc.MinAmount != 300_000*rub || wc.MaxAmount != 10_000_000*rub || wc.BaseRateBP != 1650 || wc.AutoApproveLimit != 3_000_000*rub {
		t.Fatalf("working_capital values wrong: %+v", wc)
	}
	od, _ := Get(Overdraft)
	if od.MinTerm != 12 || od.MaxTerm != 12 || od.BaseRateBP != 1800 {
		t.Fatalf("overdraft values wrong: %+v", od)
	}
	for _, p := range c {
		if p.Title == "" || p.Description == "" || p.Hint == "" || len(p.Documents) == 0 {
			t.Errorf("product %s lacks human text or documents", p.ID)
		}
	}
}

func TestGet_Unknown(t *testing.T) {
	if _, ok := Get("nope"); ok {
		t.Fatal("unknown product must not be found")
	}
}

func TestLGD_SecuredProductLosesLessThanUnsecured(t *testing.T) {
	lgd := map[ID]float64{WorkingCapital: 0.60, Equipment: 0.40, Overdraft: 0.65, Express: 0.75}
	for id, want := range lgd {
		p, _ := Get(id)
		if p.LGD != want {
			t.Errorf("%s: LGD %v, want %v", id, p.LGD, want)
		}
	}
	eq, _ := Get(Equipment)
	ex, _ := Get(Express)
	if !(eq.LGD < ex.LGD) {
		t.Fatal("equipment is backed by the purchased asset, express is unsecured")
	}
}

func TestValidateRequest_OutOfRange_ReturnsFieldErrors(t *testing.T) {
	p, _ := Get(WorkingCapital)
	errs := ValidateRequest(p, 10, 12)
	if !hasField(errs, "amount") {
		t.Errorf("10 kopecks must fail on amount: %v", errs)
	}
	errs = ValidateRequest(p, 1_000_000_000_000, 12)
	if !hasField(errs, "amount") {
		t.Errorf("huge amount must fail on amount: %v", errs)
	}
	errs = ValidateRequest(p, 1_000_000*rub, 0)
	if !hasField(errs, "termMonths") {
		t.Errorf("term 0 must fail on termMonths: %v", errs)
	}
	if errs := ValidateRequest(p, 1_000_000*rub, 12); len(errs) != 0 {
		t.Errorf("valid request must pass, got %v", errs)
	}
}

func hasField(errs []FieldError, f string) bool {
	for _, e := range errs {
		if e.Field == f && e.Message != "" {
			return true
		}
	}
	return false
}

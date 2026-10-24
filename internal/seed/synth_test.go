package seed

import (
	"reflect"
	"testing"
)

func TestValidINN(t *testing.T) {
	for _, ok := range []string{"5000000000", "7700000016", "500100732259"} {
		if !ValidINN(ok) {
			t.Errorf("%s must be valid", ok)
		}
	}
	for _, bad := range []string{"", "123", "5000000001", "7700000017", "500100732250", "50000000a0", "50000000000"} {
		if ValidINN(bad) {
			t.Errorf("%q must be invalid", bad)
		}
	}
}

func TestSynthesize_RejectsMalformedINN(t *testing.T) {
	if _, ok := Synthesize("5000000001"); ok {
		t.Fatal("a wrong check digit must not produce a company")
	}
}

func TestSynthesize_IsDeterministic(t *testing.T) {
	a, _ := Synthesize("7700000016")
	b, _ := Synthesize("7700000016")
	if !reflect.DeepEqual(a.Revenue, b.Revenue) || a.Name != b.Name || !reflect.DeepEqual(a.BKI, b.BKI) {
		t.Fatalf("the same INN must always give the same company:\n%+v\n%+v", a, b)
	}
}

func TestSynthesize_KindFollowsINNLength(t *testing.T) {
	org, _ := Synthesize("7700000016")
	ip, _ := Synthesize("500100732259")
	if org.Kind != "ooo" || ip.Kind != "ip" {
		t.Fatalf("kinds: %s %s", org.Kind, ip.Kind)
	}
}

func TestExampleINNs_CoverEveryArchetype_AndAreSyntheticCompaniesOfThatKind(t *testing.T) {
	ex := ExampleINNs()
	if len(ex) != 6 {
		t.Fatalf("want one example per archetype, got %d", len(ex))
	}
	for _, e := range ex {
		c, ok := Synthesize(e.INN)
		if !ok || ArchetypeOf(e.INN) != e.Archetype {
			t.Fatalf("example %s is not what it claims: %+v", e.INN, e)
		}
		if _, demo := ByINN(e.INN); demo {
			t.Fatalf("example %s clashes with a demo company", e.INN)
		}
		switch e.Archetype {
		case Liquidated:
			if c.Status != "liquidated" {
				t.Errorf("%s must be liquidated", e.INN)
			}
		case Overdue:
			if c.BKI == nil || c.BKI.MaxOverdueDays <= 90 {
				t.Errorf("%s must have an overdue above 90 days", e.INN)
			}
		case Young:
			if c.BKI == nil || c.BKI.HasHistory || len(c.Revenue) > 6 {
				t.Errorf("%s must be young without history", e.INN)
			}
		}
	}
}

func TestSynthesize_EverySyntheticCompanyHasUsableData(t *testing.T) {
	n := 0
	for k := int64(7_800_000_000); n < 300; k++ {
		inn := ""
		inn = padINN(k)
		if !ValidINN(inn) {
			continue
		}
		n++
		c, ok := Synthesize(inn)
		if !ok || c.Name == "" || len(c.Revenue) == 0 || c.BKI == nil || c.OKVEDGroup < 1 || c.OKVEDGroup > 3 {
			t.Fatalf("%s gives an unusable company: %+v", inn, c)
		}
		for _, v := range c.Revenue {
			if v <= 0 {
				t.Fatalf("%s: revenue must be positive: %v", inn, c.Revenue)
			}
		}
	}
}

func padINN(n int64) string {
	s := ""
	for i := 0; i < 10; i++ {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

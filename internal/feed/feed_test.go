package feed

import (
	"testing"
	"time"
)

const rub = int64(100)

func bakery() Profile {
	return Profile{CompanyName: "ООО «Тёплый хлеб»", OKVED: "10.71", CompanyAgeMonths: 72, AvgMonthlyRevenue: 2_400_000 * rub, Volatility: 0.04}
}

func findProduct(cards []Card, id string) *Card {
	for i := range cards {
		if cards[i].ProductID == id {
			return &cards[i]
		}
	}
	return nil
}

func TestFeed_Bakery_NoApp_ShowsWorkingCapital(t *testing.T) {
	cards := Build(bakery())
	if findProduct(cards, "working_capital") == nil {
		t.Fatalf("a bakery without applications should be offered working capital: %+v", cards)
	}
	if findProduct(cards, "equipment") == nil {
		t.Fatalf("manufacturing (OKVED 10) should also see equipment: %+v", cards)
	}
}

func TestFeed_NeedsInfo_ShowsClarificationFirst(t *testing.T) {
	p := bakery()
	p.Applications = []AppSummary{{ID: "app-1", Status: "NEEDS_INFO", ProductTitle: "Кредит на оборотные средства"}}
	cards := Build(p)
	if len(cards) == 0 || cards[0].Kind != "action" || cards[0].ApplicationID != "app-1" {
		t.Fatalf("clarification must be the first card: %+v", cards)
	}
}

func TestFeed_OfferMade_ShowsDeadline(t *testing.T) {
	p := bakery()
	until := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	p.Applications = []AppSummary{{ID: "app-2", Status: "OFFER_MADE", ProductTitle: "Овердрафт", OfferExpiresAt: until}}
	cards := Build(p)
	if cards[0].Kind != "status" || cards[0].ApplicationID != "app-2" || !contains(cards[0].Body, "28 октября") {
		t.Fatalf("offer card must show the deadline: %+v", cards[0])
	}
}

func TestFeed_ConstructionCompany_ShowsEquipment(t *testing.T) {
	p := Profile{OKVED: "41.20", CompanyAgeMonths: 60, AvgMonthlyRevenue: 1_500_000 * rub, Volatility: 0.25}
	cards := Build(p)
	if findProduct(cards, "equipment") == nil {
		t.Fatalf("construction should see equipment: %+v", cards)
	}
	if findProduct(cards, "overdraft") == nil {
		t.Fatalf("volatile revenue should suggest overdraft: %+v", cards)
	}
}

func TestFeed_YoungCompany_GetsHistoryTipAndExpress(t *testing.T) {
	p := Profile{OKVED: "62.01", CompanyAgeMonths: 4, AvgMonthlyRevenue: 300_000 * rub, Volatility: 0.3}
	cards := Build(p)
	tip := false
	for _, c := range cards {
		if c.Kind == "tip" && contains(c.Title, "историю") {
			tip = true
		}
	}
	if !tip {
		t.Fatalf("young company should get a payment-history tip: %+v", cards)
	}
	if findProduct(cards, "express") == nil {
		t.Fatalf("young small company should see express credit: %+v", cards)
	}
}

func TestFeed_DifferentProfiles_DifferentCards(t *testing.T) {
	a := Build(bakery())
	b := Build(Profile{OKVED: "62.01", CompanyAgeMonths: 4, AvgMonthlyRevenue: 300_000 * rub, Volatility: 0.3})
	if sameTitles(a, b) {
		t.Fatal("different profiles must get different feeds")
	}
}

func TestFeed_MaxSixCards_SortedByPriority(t *testing.T) {
	p := bakery()
	p.Applications = []AppSummary{
		{ID: "1", Status: "NEEDS_INFO"}, {ID: "2", Status: "OFFER_MADE"}, {ID: "3", Status: "ANALYST_REVIEW"},
		{ID: "4", Status: "DISBURSED"}, {ID: "5", Status: "REJECTED"}, {ID: "6", Status: "SCORING"}, {ID: "7", Status: "ENRICHMENT"},
	}
	cards := Build(p)
	if len(cards) > 6 {
		t.Fatalf("at most 6 cards, got %d", len(cards))
	}
	for i := 1; i < len(cards); i++ {
		if cards[i].Priority < cards[i-1].Priority {
			t.Fatalf("cards must be sorted by priority: %+v", cards)
		}
	}
}

func TestFeed_ZeroRevenue_NoPanic(t *testing.T) {
	if cards := Build(Profile{}); len(cards) == 0 {
		t.Fatal("even an empty profile gets generic cards")
	}
}

func TestFeed_UniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Build(bakery()) {
		if seen[c.ID] {
			t.Fatalf("duplicate card id %s", c.ID)
		}
		seen[c.ID] = true
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func sameTitles(a, b []Card) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Title != b[i].Title {
			return false
		}
	}
	return true
}

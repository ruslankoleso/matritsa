package seed

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strings"

	"matritsa/internal/integrations"
)

// ValidINN reports whether inn is a well-formed Russian taxpayer number: 10 digits (organisation)
// or 12 digits (individual entrepreneur) with correct check digits.
func ValidINN(inn string) bool {
	for _, c := range inn {
		if c < '0' || c > '9' {
			return false
		}
	}
	d := make([]int, len(inn))
	for i, c := range inn {
		d[i] = int(c - '0')
	}
	check := func(coef []int) int {
		sum := 0
		for i, k := range coef {
			sum += d[i] * k
		}
		return sum % 11 % 10
	}
	switch len(inn) {
	case 10:
		return check([]int{2, 4, 10, 3, 5, 9, 4, 6, 8}) == d[9]
	case 12:
		return check([]int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}) == d[10] &&
			check([]int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}) == d[11]
	}
	return false
}

// Archetype is the kind of synthetic company an INN turns into.
type Archetype string

const (
	Reliable   Archetype = "reliable"   // long-standing, clean history: an automatic offer
	Borderline Archetype = "borderline" // average: the analyst decides
	Young      Archetype = "young"      // a few months old, no credit history
	Indebted   Archetype = "indebted"   // heavy existing payments
	Liquidated Archetype = "liquidated" // not an active company: automatic decline
	Overdue    Archetype = "overdue"    // payment overdue by more than 90 days: hard stop
	Flagged    Archetype = "flagged"    // the registry marks the director as disqualified or the data as unreliable: hard stop
)

var archetypeNote = map[Archetype]string{
	Reliable:   "Надёжная компания: предложение придёт автоматически",
	Borderline: "Пограничный случай: решение примет сотрудник",
	Young:      "Молодая компания без кредитной истории",
	Indebted:   "Закредитованная компания: сотрудник предложит меньшую сумму",
	Liquidated: "Ликвидированная компания: автоматический отказ",
	Overdue:    "Просрочка более 90 дней: отказ по жёсткому правилу",
	Flagged:    "Дисквалификация руководителя или недостоверные сведения в реестре: отказ по жёсткому правилу",
}

func seedOf(inn string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(inn))
	return h.Sum64()
}

// ArchetypeOf is what a valid INN will look like in the simulated registry and bureau.
// The mix favours companies that can be processed (liquidated and overdue ones are rare).
func ArchetypeOf(inn string) Archetype {
	switch seedOf(inn) % 10 {
	case 0, 1:
		return Reliable
	case 2, 3, 4:
		return Borderline
	case 5, 6:
		return Young
	case 7, 8:
		return Indebted
	}
	switch seedOf(inn) / 10 % 3 {
	case 0:
		return Liquidated
	case 1:
		return Overdue
	}
	return Flagged
}

var (
	orgWords   = []string{"Север", "Прогресс", "Вектор", "Кедр", "Волна", "Маяк", "Заря", "Горизонт", "Альтаир", "Родник", "Простор", "Мастер"}
	orgKinds   = []string{"Торг", "Сервис", "Строй", "Групп", "Снаб", "Трейд", "Тех"}
	surnames   = []string{"Иванов", "Петров", "Сидоров", "Кузнецов", "Попов", "Васильев", "Соколов", "Михайлов", "Новиков", "Фёдоров", "Морозов", "Егоров"}
	firstNames = []string{"Александр", "Дмитрий", "Сергей", "Андрей", "Николай", "Павел", "Максим", "Олег"}
	patronyms  = []string{"Александрович", "Дмитриевич", "Сергеевич", "Андреевич", "Николаевич", "Павлович", "Олегович", "Игоревич"}
	cities     = []string{"г. Казань, ул. Кремлёвская, 12", "г. Самара, ул. Ленинградская, 40", "г. Пермь, ул. Сибирская, 8", "г. Тула, пр-т Ленина, 77", "г. Воронеж, ул. Плехановская, 33", "г. Омск, ул. Маркса, 19"}
	activities = []struct {
		okved, title string
		group        int
	}{
		{"47.11", "Торговля розничная в неспециализированных магазинах", 1},
		{"56.10", "Деятельность ресторанов и кафе", 2},
		{"62.01", "Разработка программного обеспечения", 1},
		{"49.41", "Деятельность автомобильного грузового транспорта", 2},
		{"43.99", "Работы строительные специализированные", 3},
		{"96.02", "Услуги парикмахерских и салонов красоты", 2},
		{"10.71", "Производство хлеба и мучных кондитерских изделий", 1},
	}
)

// Synthesize builds a deterministic synthetic company for any valid INN: the same INN always
// gives the same registry record, bureau report and revenue. Used by the simulated sources and
// by sign-up, so a new account has data to work with. The second result is false for malformed INNs.
func Synthesize(inn string) (Company, bool) {
	if !ValidINN(inn) {
		return Company{}, false
	}
	a := ArchetypeOf(inn)
	r := rand.New(rand.NewPCG(seedOf(inn), 0x9e3779b97f4a7c15))
	between := func(lo, hi int) int { return lo + r.IntN(hi-lo+1) }
	pick := func(list []string) string { return list[r.IntN(len(list))] }

	kind := "ooo"
	owner := fmt.Sprintf("%s %s %s", pick(surnames), pick(firstNames), pick(patronyms))
	name := fmt.Sprintf("ООО «%s%s»", pick(orgWords), strings.ToLower(pick(orgKinds)))
	if len(inn) == 12 {
		kind = "ip"
		name = fmt.Sprintf("ИП %s", owner)
	}
	act := activities[r.IntN(len(activities))]
	if a == Reliable { // a steady business is not in the highest risk group
		for act.group > 2 {
			act = activities[r.IntN(len(activities))]
		}
	}

	c := Company{
		INN: inn, OwnerName: owner, Name: name, Kind: kind, OKVED: act.okved, OKVEDTitle: act.title, OKVEDGroup: act.group,
		Director: owner, Address: pick(cities), Status: "active", Story: archetypeNote[a],
	}
	base := int64(between(400, 900)) * 1000 * rub
	pattern := []float64{1.0, 0.96, 1.04, 1.02, 0.97, 1.05}
	history := func(score, loans, overdue, recent int, debtShare, repaidShare float64) *integrations.BKIReport {
		closed := 0
		if repaidShare > 0 {
			closed = 1 + int(repaidShare)
		}
		return &integrations.BKIReport{HasHistory: true, ActiveLoans: loans, TotalDebt: int64(float64(base) * debtShare), MaxOverdueDays: overdue, Overdues: integrations.OverdueHistory(overdue, recent), Score: score, ClosedLoans: closed, TotalRepaid: int64(float64(base) * repaidShare)}
	}

	// The accounts and the statement of a synthetic company: the operating margin follows the industry as the old estimate did,
	// the rest depends on the kind of company. A fraction of a percent is added per INN so that no two look alike.
	noi := map[int]float64{1: 0.35, 2: 0.30, 3: 0.25}[act.group] + float64(between(-2, 2))/100
	fin := func(gross, current float64, anomaly int) {
		c.Finance = &integrations.Finance{GrossMargin: gross + float64(between(0, 4))/100, CurrentRatio: current}
		c.Statement = integrations.StatementOfLevel(noi, anomaly)
	}

	switch a {
	case Reliable:
		c.RegisteredAt = monthsAgo(between(66, 120))
		fin(0.36, 1.6+float64(between(0, 6))/10, 0)
		base = int64(between(1500, 3500)) * 1000 * rub
		c.MonthlyObligations = base * int64(between(3, 5)) / 100
		c.Revenue = series(base, pattern, 12)
		c.BKI = history(between(750, 820), 1, 0, 0, 0.5, float64(between(3, 6)))
	case Borderline:
		c.RegisteredAt = monthsAgo(between(24, 60))
		fin(0.24, 1.0+float64(between(0, 3))/10, between(0, 1))
		c.LitigationLevel = between(0, 1)
		c.MonthlyObligations = base * int64(between(5, 8)) / 100
		c.Revenue = series(base, []float64{1.0, 0.92, 1.08, 1.04, 0.95}, 12)
		c.BKI = history(between(590, 640), 1, between(18, 30), 1, 0.8, float64(between(1, 2)))
	case Young:
		c.RegisteredAt = monthsAgo(between(3, 8))
		fin(0.40, 1.1+float64(between(0, 3))/10, 0)
		start := base / 3
		c.Revenue = []int64{start, start * 13 / 10, start * 15 / 10, start * 17 / 10}
		c.BKI = &integrations.BKIReport{HasHistory: false}
	case Indebted:
		c.RegisteredAt = monthsAgo(between(36, 96))
		fin(0.22, 1.0+float64(between(0, 2))/10, 0)
		c.LitigationLevel = between(0, 1)
		c.MonthlyObligations = base * int64(between(9, 12)) / 100
		c.Revenue = series(base, []float64{1.0, 0.8, 1.15, 0.9, 1.2, 0.85}, 12)
		c.BKI = history(between(580, 625), 3, between(2, 3), 1, 4.5, 1)
	case Liquidated:
		c.RegisteredAt = monthsAgo(between(60, 120))
		fin(0.25, 1.2, 0)
		c.Status = "liquidated"
		c.Revenue = series(base, []float64{1.0, 0.9, 1.1}, 12)
		c.BKI = history(700, 0, 0, 0, 0, 1)
	case Flagged:
		c.RegisteredAt = monthsAgo(between(36, 96))
		c.MonthlyObligations = base * int64(between(3, 5)) / 100
		c.Revenue = series(base, pattern, 12)
		c.BKI = history(between(700, 780), 1, 0, 0, 0.5, float64(between(2, 4)))
		fin(0.30, 1.4, 0)
		// the two marks are told apart by the INN, so that both kinds appear
		if seedOf(inn)/30%2 == 0 {
			c.DirectorDisqualified = true
		} else {
			c.UnreliableData = true
		}
	case Overdue:
		c.RegisteredAt = monthsAgo(between(36, 96))
		fin(0.15, 0.9, 1)
		c.LitigationLevel = 1
		c.MonthlyObligations = base * int64(between(12, 16)) / 100
		c.Revenue = series(base, []float64{1.0, 0.7, 1.3, 0.9}, 12)
		c.BKI = history(between(470, 520), 4, between(95, 150), 1, 5, 1)
	}
	return c, true
}

// ExampleINNs returns valid INNs, one for each kind of company, for the sign-up form hint.
func ExampleINNs() []Example {
	var out []Example
	seen := map[Archetype]bool{}
	want := []Archetype{Reliable, Borderline, Young, Indebted, Liquidated, Overdue, Flagged}
	// The INNs below are searched once at start-up from a fixed range, so the list is stable.
	for n := int64(7_700_000_000); len(seen) < len(want) && n < 7_700_100_000; n++ {
		inn := fmt.Sprintf("%010d", n)
		if !ValidINN(inn) {
			continue
		}
		if a := ArchetypeOf(inn); !seen[a] {
			seen[a] = true
			c, _ := Synthesize(inn)
			out = append(out, Example{INN: inn, Name: c.Name, Note: archetypeNote[a], Archetype: a})
		}
	}
	order := map[Archetype]int{}
	for i, a := range want {
		order[a] = i
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if order[out[j].Archetype] < order[out[i].Archetype] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Example is a sign-up hint: an INN and what it will turn into.
type Example struct {
	INN       string
	Name      string
	Note      string
	Archetype Archetype
}

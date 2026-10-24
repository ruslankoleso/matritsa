// Package scoring is a transparent points-based risk model: every factor
// returns its own points and a plain-language note, and the sum is the score.
package scoring

import (
	"fmt"
	"math"

	"matritsa/internal/integrations"
)

type Grade string

const (
	GradeA Grade = "A"
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeD Grade = "D"
	GradeE Grade = "E"
)

// GradeFor maps a 0..100 score to a risk grade.
func GradeFor(score int) Grade {
	switch {
	case score >= 80:
		return GradeA
	case score >= 65:
		return GradeB
	case score >= 50:
		return GradeC
	case score >= 35:
		return GradeD
	default:
		return GradeE
	}
}

// Input is everything the model needs; amounts are in kopecks.
// The borrower score uses only the first group; the request group feeds affordability and expected loss.
type Input struct {
	// Borrower.
	CompanyAgeMonths   int
	MonthlyRevenue     []int64 // last months, newest last
	MonthlyObligations int64   // existing monthly loan payments
	BKI                *integrations.BKIReport
	EGRUL              integrations.EGRULRecord
	OKVEDGroup         int // 1 low risk .. 3 high risk
	// Request.
	Amount     int64
	TermMonths int
	RateBP     int     // rate used to estimate the new payment
	LGD        float64 // loss given default of the product (share of the exposure lost if the borrower defaults)
}

type Factor struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Points int    `json:"points"`
	Max    int    `json:"max"`
	Note   string `json:"note"`
}

// Result separates three things: who the borrower is (Score, Grade, PD), what it costs the bank if
// they default (EAD, LGD, EL) and whether this request is affordable (DebtLoad, MaxAffordableAmount).
type Result struct {
	Score   int      `json:"score"`
	Grade   Grade    `json:"grade"`
	Factors []Factor `json:"factors"`

	PD            float64 `json:"pd"`           // annual probability of default for the grade
	CumulativePD  float64 `json:"cumulativePd"` // probability of default within the loan term
	LGD           float64 `json:"lgd"`
	EAD           int64   `json:"ead"`              // exposure at default, kopecks (the requested amount)
	ExpectedLoss  int64   `json:"expectedLoss"`     // EL = cumulative PD x LGD x EAD, kopecks
	ELRate        float64 `json:"expectedLossRate"` // EL / EAD
	RiskPremiumBP int     `json:"riskPremiumBp"`    // rate surcharge over a grade-A borrower, basis points

	// RepaidTotal is the sum of loans the borrower has already repaid in full, RepaidCoverage its share of the requested amount.
	RepaidTotal    int64   `json:"repaidTotal"`
	RepaidCoverage float64 `json:"repaidCoverage"`

	Payment             int64    `json:"payment"`
	DebtLoad            float64  `json:"debtLoad"`
	MaxAffordableAmount int64    `json:"maxAffordableAmount"`
	HardStops           []string `json:"hardStops"`
}

// Score evaluates the input. It never panics on empty or zero revenue.
func Score(in Input, cfg Config) Result {
	avg := averageRevenue(in.MonthlyRevenue)
	factors := []Factor{
		ageFactor(in.CompanyAgeMonths, cfg),
		stabilityFactor(in.MonthlyRevenue, avg, cfg),
		existingDebtFactor(in.MonthlyObligations, avg, cfg),
		bkiFactor(in.BKI, cfg),
		overdueFactor(in.BKI, cfg),
		okvedFactor(in.OKVEDGroup, cfg),
	}
	score := 0
	for _, f := range factors {
		score += f.Points
	}
	grade := GradeFor(score)
	hard := hardStops(in, avg, cfg)
	if avg <= 0 {
		// Without proven revenue nothing can be repaid: grade E regardless of other points.
		grade = GradeE
	}

	res := Result{Score: score, Grade: grade, Factors: factors, HardStops: hard, LGD: in.LGD, EAD: in.Amount}
	res.PD = cfg.PD[grade]
	res.CumulativePD = cumulativePD(res.PD, in.TermMonths)
	res.ELRate = res.CumulativePD * in.LGD
	res.ExpectedLoss = int64(math.Round(float64(in.Amount) * res.ELRate))
	if bp, ok := Premium(grade, in.LGD, cfg); ok {
		res.RiskPremiumBP = bp
	}

	if in.BKI != nil {
		res.RepaidTotal = in.BKI.TotalRepaid
		if in.Amount > 0 {
			res.RepaidCoverage = float64(in.BKI.TotalRepaid) / float64(in.Amount)
		}
	}

	res.Payment = MonthlyPayment(in.Amount, in.RateBP, in.TermMonths)
	if avg > 0 {
		res.DebtLoad = float64(in.MonthlyObligations+res.Payment) / float64(avg)
	}
	res.MaxAffordableAmount = maxAffordable(avg, in, cfg)
	return res
}

// cumulativePD turns an annual PD into the probability of defaulting at some point within the term.
func cumulativePD(annual float64, termMonths int) float64 {
	if termMonths <= 0 {
		return 0
	}
	return 1 - math.Pow(1-annual, float64(termMonths)/12)
}

// Premium is the rate surcharge, in basis points, for a grade on top of the product's base rate.
// The base rate already prices a grade-A borrower, so the premium is the extra annual expected loss
// rate: (PD_grade - PD_A) x LGD. Grade E gets no offer at all.
func Premium(g Grade, lgd float64, cfg Config) (int, bool) {
	if g == GradeE {
		return 0, false
	}
	extra := cfg.PD[g] - cfg.PD[GradeA]
	if extra < 0 {
		extra = 0
	}
	return int(math.Round(extra * lgd * 10000)), true
}

func hardStops(in Input, avg int64, cfg Config) []string {
	var stops []string
	if in.EGRUL.Status != "" && in.EGRUL.Status != "active" {
		stops = append(stops, "Компания не является действующей по данным реестра (ЕГРЮЛ/ЕГРИП)")
	}
	if in.EGRUL.Bankrupt {
		stops = append(stops, "В отношении компании ведётся процедура банкротства")
	}
	if in.BKI != nil && in.BKI.MaxOverdueDays > cfg.HardStopOverdueDays {
		stops = append(stops, fmt.Sprintf("В кредитной истории есть просрочка более %d дней", cfg.HardStopOverdueDays))
	}
	if avg <= 0 {
		stops = append(stops, "Нет подтверждённой выручки, из которой можно погашать кредит")
	}
	return stops
}

func averageRevenue(rev []int64) int64 {
	if len(rev) == 0 {
		return 0
	}
	var sum int64
	for _, v := range rev {
		sum += v
	}
	return sum / int64(len(rev))
}

func ageFactor(months int, cfg Config) Factor {
	f := Factor{Key: "company_age", Title: "Возраст бизнеса", Max: cfg.MaxAge}
	switch {
	case months >= 36:
		f.Points, f.Note = cfg.MaxAge, fmt.Sprintf("Срок работы %s", years(months))
	case months >= 12:
		f.Points, f.Note = cfg.MaxAge*2/3, fmt.Sprintf("Срок работы %s: достаточно для оценки результатов", years(months))
	case months >= 6:
		f.Points, f.Note = cfg.MaxAge/3, fmt.Sprintf("Срок работы %d мес.: история ограничена", months)
	default:
		f.Points, f.Note = 0, fmt.Sprintf("Срок работы %d мес.: недостаточно для оценки устойчивости", months)
	}
	return f
}

func years(months int) string {
	if months < 12 {
		return fmt.Sprintf("%d мес.", months)
	}
	y := months / 12
	switch {
	case y%10 == 1 && y%100 != 11:
		return fmt.Sprintf("%d год", y)
	case y%10 >= 2 && y%10 <= 4 && (y%100 < 10 || y%100 >= 20):
		return fmt.Sprintf("%d года", y)
	default:
		return fmt.Sprintf("%d лет", y)
	}
}

func stabilityFactor(rev []int64, avg int64, cfg Config) Factor {
	f := Factor{Key: "revenue_stability", Title: "Стабильность выручки", Max: cfg.MaxStability}
	if avg <= 0 || len(rev) < 3 {
		f.Note = "Данных о выручке недостаточно для оценки стабильности (нужно не менее 3 месяцев)"
		return f
	}
	var sq float64
	for _, v := range rev {
		d := float64(v - avg)
		sq += d * d
	}
	cv := math.Sqrt(sq/float64(len(rev))) / float64(avg)
	switch {
	case cv < 0.15:
		f.Points, f.Note = cfg.MaxStability, fmt.Sprintf("Выручка стабильна: коэффициент вариации %d%%", pct(cv))
	case cv < 0.30:
		f.Points, f.Note = cfg.MaxStability*13/20, fmt.Sprintf("Умеренные колебания выручки: коэффициент вариации %d%%", pct(cv))
	case cv < 0.50:
		f.Points, f.Note = cfg.MaxStability*6/20, fmt.Sprintf("Значительные колебания выручки: коэффициент вариации %d%%", pct(cv))
	default:
		f.Points, f.Note = 0, fmt.Sprintf("Выручка нестабильна: коэффициент вариации %d%%", pct(cv))
	}
	return f
}

// existingDebtFactor looks only at what the company already pays, before the new loan.
func existingDebtFactor(obligations, avg int64, cfg Config) Factor {
	f := Factor{Key: "existing_debt", Title: "Текущая долговая нагрузка", Max: cfg.MaxExistingDebt}
	if avg <= 0 {
		f.Note = "Нет подтверждённой выручки: нагрузку оценить нельзя"
		return f
	}
	load := float64(obligations) / float64(avg)
	switch {
	case load < 0.10:
		f.Points, f.Note = cfg.MaxExistingDebt, fmt.Sprintf("Действующие платежи составляют %d%% выручки: низкая нагрузка", pct(load))
	case load < 0.20:
		f.Points, f.Note = cfg.MaxExistingDebt*15/20, fmt.Sprintf("Действующие платежи составляют %d%% выручки: умеренная нагрузка", pct(load))
	case load < 0.30:
		f.Points, f.Note = cfg.MaxExistingDebt*9/20, fmt.Sprintf("Действующие платежи составляют %d%% выручки: повышенная нагрузка", pct(load))
	case load < 0.45:
		f.Points, f.Note = cfg.MaxExistingDebt*3/20, fmt.Sprintf("Действующие платежи составляют %d%% выручки: высокая нагрузка", pct(load))
	default:
		f.Points, f.Note = 0, fmt.Sprintf("Действующие платежи составляют %d%% выручки: чрезмерная нагрузка", pct(load))
	}
	return f
}

func pct(v float64) int { return int(math.Round(v * 100)) }

func bkiFactor(b *integrations.BKIReport, cfg Config) Factor {
	f := Factor{Key: "bki_score", Title: "Кредитная история (балл БКИ)", Max: cfg.MaxBKI}
	switch {
	case b == nil || !b.HasHistory:
		f.Points, f.Note = cfg.MaxBKI*10/25, "Кредитная история отсутствует: оценка по данным бюро невозможна"
	case b.Score >= 750:
		f.Points, f.Note = cfg.MaxBKI, fmt.Sprintf("Балл бюро кредитных историй %d: высокий", b.Score)
	case b.Score >= 650:
		f.Points, f.Note = cfg.MaxBKI*18/25, fmt.Sprintf("Балл бюро кредитных историй %d: хороший", b.Score)
	case b.Score >= 550:
		f.Points, f.Note = cfg.MaxBKI*9/25, fmt.Sprintf("Балл бюро кредитных историй %d: средний", b.Score)
	default:
		f.Points, f.Note = 0, fmt.Sprintf("Балл бюро кредитных историй %d: низкий", b.Score)
	}
	return f
}

func overdueFactor(b *integrations.BKIReport, cfg Config) Factor {
	f := Factor{Key: "overdue", Title: "Просрочки по платежам", Max: cfg.MaxOverdue}
	switch {
	case b == nil || !b.HasHistory || b.MaxOverdueDays == 0:
		f.Points, f.Note = cfg.MaxOverdue, "Просрочек по платежам не было"
	case b.MaxOverdueDays <= 30:
		f.Points, f.Note = cfg.MaxOverdue*8/15, fmt.Sprintf("Короткая просрочка: до %d дн.", b.MaxOverdueDays)
	default:
		f.Points, f.Note = 0, fmt.Sprintf("Значительная просрочка: до %d дн.", b.MaxOverdueDays)
	}
	return f
}

func okvedFactor(group int, cfg Config) Factor {
	f := Factor{Key: "okved_risk", Title: "Отрасль", Max: cfg.MaxOKVED}
	switch group {
	case 1:
		f.Points, f.Note = cfg.MaxOKVED, "Отрасль с устойчивым спросом"
	case 2:
		f.Points, f.Note = cfg.MaxOKVED*3/5, "Отрасль со средней зависимостью от рыночной конъюнктуры"
	default:
		f.Points, f.Note = 0, "Отрасль с повышенной зависимостью от рыночной конъюнктуры"
	}
	return f
}

// maxAffordable is the largest amount whose payment keeps the total load at or below cfg.MaxLoad.
func maxAffordable(avg int64, in Input, cfg Config) int64 {
	if avg <= 0 || in.TermMonths <= 0 {
		return 0
	}
	room := cfg.MaxLoad*float64(avg) - float64(in.MonthlyObligations)
	if room <= 0 {
		return 0
	}
	r := float64(in.RateBP) / 10000 / 12
	if r == 0 {
		return int64(room * float64(in.TermMonths))
	}
	factor := (1 - math.Pow(1+r, -float64(in.TermMonths))) / r
	return int64(math.Floor(room * factor))
}

// Metrics is the part of a result that describes the request rather than the borrower; it is stored next to the score.
type Metrics struct {
	PD            float64 `json:"pd"`
	CumulativePD  float64 `json:"cumulativePd"`
	LGD           float64 `json:"lgd"`
	EAD           int64   `json:"ead"`
	ExpectedLoss  int64   `json:"expectedLoss"`
	ELRate        float64 `json:"expectedLossRate"`
	RiskPremiumBP int     `json:"riskPremiumBp"`
	Payment       int64   `json:"payment"`

	RepaidTotal    int64   `json:"repaidTotal"`
	RepaidCoverage float64 `json:"repaidCoverage"`
}

func (r Result) Metrics() Metrics {
	return Metrics{PD: r.PD, CumulativePD: r.CumulativePD, LGD: r.LGD, EAD: r.EAD, ExpectedLoss: r.ExpectedLoss, ELRate: r.ELRate, RiskPremiumBP: r.RiskPremiumBP, Payment: r.Payment, RepaidTotal: r.RepaidTotal, RepaidCoverage: r.RepaidCoverage}
}

// Package backtest checks how well the scorecard separates borrowers who default from those who do not,
// on a synthetic portfolio. The bank's data is not available to the prototype, so the "true" chance of
// default of each company is set by an explicit model below (TrueLogit). The check answers a narrow
// question: if the world works roughly like this, does the scorecard rank the risk right and do its
// classes hold increasing shares of defaults? It is not a calibration of the bank's model.
package backtest

import (
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"matritsa/internal/integrations"
	"matritsa/internal/scoring"
)

const rub = int64(100)

// Borrower is one synthetic company with its request and what happened to the loan.
type Borrower struct {
	In        scoring.Input
	Coverage  float64 // cash flow / all loan payments with the new one
	Volatile  float64
	Hidden    float64 // risk the scorecard cannot see: management, market, luck
	PD        float64 // the true chance of default over the term
	Defaulted bool
}

// TrueLogit is the assumed world: the log-odds of default over the term grow with a thin coverage of
// payments, past overdue, a young business, unstable revenue, anomalies in the statement, lawsuits,
// a weak liquidity and a long term; a part of the risk is hidden from any scorecard.
func TrueLogit(b Borrower, overdueDays, ageMonths int, anomaly, litigation int, currentRatio float64) float64 {
	x := -3.9
	x += 2.4 * math.Max(0, 1.4-b.Coverage)
	x += 0.018 * math.Min(float64(overdueDays), 90)
	if ageMonths < 24 {
		x += 0.9
	}
	x += 2.5 * b.Volatile
	x += 0.55 * float64(anomaly)
	x += 0.45 * float64(litigation)
	if currentRatio > 0 && currentRatio < 1 {
		x += 0.5
	}
	x += 0.012 * float64(b.In.TermMonths)
	return x + b.Hidden
}

// Generate makes n active companies that pass the hard stops: the stops refuse on their own, the
// question is how the points rank the rest.
func Generate(n int, seed uint64) []Borrower {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	out := make([]Borrower, 0, n)
	for len(out) < n {
		age := 6 + int(math.Exp(r.NormFloat64()*0.8+3.6)) // months, median about three years
		revenue := int64(math.Exp(r.NormFloat64()*0.7+math.Log(1_500_000))) * rub
		vol := math.Min(0.45, math.Abs(r.NormFloat64()*0.12))
		months := make([]int64, 12)
		for i := range months {
			months[i] = int64(float64(revenue) * math.Max(0.2, 1+r.NormFloat64()*vol))
		}
		margin := 0.12 + r.Float64()*0.33
		cash := float64(revenue) * margin
		obligations := int64(cash * r.Float64() * 0.6)

		hasHistory := age > 12 && r.Float64() < 0.85
		overdue := 0
		if hasHistory {
			switch u := r.Float64(); {
			case u < 0.6:
			case u < 0.8:
				overdue = 1 + r.IntN(5)
			case u < 0.93:
				overdue = 6 + r.IntN(25)
			default:
				overdue = 31 + r.IntN(60) // up to 90: more is a hard stop
			}
		}
		var bki *integrations.BKIReport
		if hasHistory {
			active := r.IntN(4)
			bki = &integrations.BKIReport{
				HasHistory: true, ActiveLoans: active, TotalDebt: obligations * int64(6+r.IntN(30)),
				ClosedLoans: r.IntN(5), TotalRepaid: int64(r.Float64() * 4 * float64(revenue)),
				MaxOverdueDays: overdue, Score: 520 + r.IntN(300) - overdue*2, Restructured: r.Float64() < 0.04,
			}
		}
		anomaly := weighted(r, 0.75, 0.2)
		litigation := weighted(r, 0.8, 0.15)
		var fin *integrations.Finance
		currentRatio := 0.0
		if r.Float64() < 0.7 {
			currentRatio = 0.6 + r.Float64()*1.6
			fin = &integrations.Finance{GrossMargin: 0.1 + r.Float64()*0.35, CurrentRatio: currentRatio}
		}
		term := []int{6, 12, 18, 24, 36}[r.IntN(5)]
		// The request: a new payment between 5 % and 90 % of the monthly cash flow.
		payShare := 0.05 + r.Float64()*0.85
		amount := principalFor(int64(cash*payShare), 1650, term) / (10_000 * rub) * (10_000 * rub)
		if amount < 100_000*rub {
			continue
		}
		in := scoring.Input{
			CompanyAgeMonths: age, MonthlyRevenue: months, MonthlyObligations: obligations, BKI: bki,
			EGRUL: integrations.EGRULRecord{Status: "active", RegisteredAt: time.Now().AddDate(0, -age, 0), Litigation: litigation,
				Finance: fin, Statement: integrations.StatementOfLevel(margin, anomaly)},
			OKVEDGroup: 1 + r.IntN(3), Collateral: []string{"none", "none", "partial", "full"}[r.IntN(4)],
			PurposeText: "Пополнение оборотных средств", PurposeDocs: r.Float64() < 0.4,
			Amount: amount, TermMonths: term, RateBP: 1650, LGD: 0.6,
		}
		payment := scoring.MonthlyPayment(amount, 1650, term)
		b := Borrower{In: in, Coverage: cash / float64(obligations+payment), Volatile: vol, Hidden: r.NormFloat64() * 0.8}
		b.PD = 1 / (1 + math.Exp(-TrueLogit(b, overdue, age, anomaly, litigation, currentRatio)))
		b.Defaulted = r.Float64() < b.PD
		out = append(out, b)
	}
	return out
}

func weighted(r *rand.Rand, p0, p1 float64) int {
	switch u := r.Float64(); {
	case u < p0:
		return 0
	case u < p0+p1:
		return 1
	}
	return 2
}

// principalFor is the amount whose annuity payment is pay.
func principalFor(pay int64, rateBP, months int) int64 {
	m := float64(rateBP) / 10000 / 12
	return int64(float64(pay) * (1 - math.Pow(1+m, -float64(months))) / m)
}

// GradeRow is the share of defaults among the loans of one class against the PD the model assumes.
type GradeRow struct {
	Grade       scoring.Grade
	Count       int
	Defaults    int
	Rate        float64 // observed share of defaults over the term
	ModelPD     float64 // the class PD of the configuration over the average term of the class
	AverageTerm float64
}

type Report struct {
	N, Defaults int
	Gini, KS    float64
	// OracleGini ranks by the true chance of default: the most any model could reach on this portfolio,
	// since a part of the risk is hidden and defaults are random even when the chance is known.
	OracleGini float64
	Grades     []GradeRow
	Monotonic  bool // the share of defaults grows from A to D
}

// Run scores the portfolio and measures the ranking and the classes.
func Run(n int, seed uint64) Report {
	cfg := scoring.DefaultConfig()
	pop := Generate(n, seed)
	type scored struct {
		score int
		bad   bool
	}
	all := make([]scored, len(pop))
	byGrade := map[scoring.Grade]*GradeRow{}
	termSum := map[scoring.Grade]int{}
	rep := Report{N: len(pop)}
	for i, b := range pop {
		res := scoring.Score(b.In, cfg)
		all[i] = scored{res.Score, b.Defaulted}
		g := byGrade[res.Grade]
		if g == nil {
			g = &GradeRow{Grade: res.Grade}
			byGrade[res.Grade] = g
		}
		g.Count++
		termSum[res.Grade] += b.In.TermMonths
		if b.Defaulted {
			g.Defaults++
			rep.Defaults++
		}
	}
	rep.Gini = 2*auc(all, func(s scored) (int, bool) { return s.score, s.bad }) - 1
	rep.KS = ks(all, func(s scored) (int, bool) { return s.score, s.bad })
	// The oracle: a higher true PD is a worse borrower, so its score is the PD negated (in basis points of a basis point).
	rep.OracleGini = 2*auc(pop, func(b Borrower) (int, bool) { return -int(b.PD * 1e8), b.Defaulted }) - 1
	rep.Monotonic = true
	prev := -1.0
	for _, grade := range []scoring.Grade{scoring.GradeA, scoring.GradeB, scoring.GradeC, scoring.GradeD} {
		g := byGrade[grade]
		if g == nil || g.Count == 0 {
			continue
		}
		g.Rate = float64(g.Defaults) / float64(g.Count)
		g.AverageTerm = float64(termSum[grade]) / float64(g.Count)
		g.ModelPD = 1 - math.Pow(1-cfg.PD[grade], g.AverageTerm/12)
		if g.Rate < prev {
			rep.Monotonic = false
		}
		prev = g.Rate
		rep.Grades = append(rep.Grades, *g)
	}
	return rep
}

// auc is the chance that a random good borrower has a higher score than a random bad one (ties count half).
func auc[T any](xs []T, get func(T) (int, bool)) float64 {
	type p struct {
		s   int
		bad bool
	}
	ps := make([]p, len(xs))
	var bads, goods float64
	for i, x := range xs {
		s, b := get(x)
		ps[i] = p{s, b}
		if b {
			bads++
		} else {
			goods++
		}
	}
	if bads == 0 || goods == 0 {
		return 0.5
	}
	slices.SortFunc(ps, func(a, b p) int { return a.s - b.s })
	var goodsUpTo, pairs float64
	for i := 0; i < len(ps); {
		j := i
		var g, b float64
		for j < len(ps) && ps[j].s == ps[i].s {
			if ps[j].bad {
				b++
			} else {
				g++
			}
			j++
		}
		goodsUpTo += g
		pairs += b * ((goods - goodsUpTo) + g/2) // goods scored higher than these bads, plus half the ties
		i = j
	}
	return pairs / (bads * goods)
}

// ks is the largest gap between the cumulative shares of bad and good borrowers along the score.
func ks[T any](xs []T, get func(T) (int, bool)) float64 {
	type p struct {
		s   int
		bad bool
	}
	ps := make([]p, len(xs))
	var bads, goods float64
	for i, x := range xs {
		s, b := get(x)
		ps[i] = p{s, b}
		if b {
			bads++
		} else {
			goods++
		}
	}
	if bads == 0 || goods == 0 {
		return 0
	}
	slices.SortFunc(ps, func(a, b p) int { return a.s - b.s })
	var cb, cg, best float64
	for i := 0; i < len(ps); {
		j := i
		for j < len(ps) && ps[j].s == ps[i].s {
			if ps[j].bad {
				cb++
			} else {
				cg++
			}
			j++
		}
		best = math.Max(best, math.Abs(cb/bads-cg/goods))
		i = j
	}
	return best
}

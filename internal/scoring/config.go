package scoring

import "math"

// Config holds the model's parameters. Every number here is illustrative: the prototype has no
// historical default statistics, so the PD table and the point tiers are NOT calibrated on data.
// The twelve criteria and their weights follow the analyst's scorecard (docs/scoring.md); they sum to 100.
type Config struct {
	// Maximum points of each criterion.
	MaxDSCR, MaxDebtEBITDA, MaxProfitability, MaxLiquidity, MaxOverdue, MaxObligations int
	MaxAge, MaxOKVED, MaxStatement, MaxReputation, MaxCollateral, MaxPurpose           int
	// Lower bounds of the grades A, B and C by score; below GradeCFrom is D.
	GradeAFrom, GradeBFrom, GradeCFrom int
	// PD is the annual probability of default by grade.
	PD map[Grade]float64
	// MaxLoad is the highest share of average monthly revenue that all loan payments may take.
	MaxLoad             float64
	HardStopOverdueDays int
	// TechnicalOverdueDays is the longest overdue still seen as technical (a late transfer, not a trouble).
	TechnicalOverdueDays int
	// MinRepaidCoverage is how much of the requested amount the borrower must already have repaid in other
	// loans (repaid total / requested amount) for an automatic offer. It is checked next to the request and
	// never changes the class of the borrower.
	MinRepaidCoverage float64
	// MinAutoBKIScore is the lowest bureau score for an automatic offer. The bureau score is not one of the
	// twelve criteria, so it works as a condition of the automatic decision, like the repayment experience.
	MinAutoBKIScore int
	// MinDSCR is the lowest cover of all loan payments by monthly cash flow (cash flow / payments).
	MinDSCR float64
	// OperatingMargin is the share of revenue left as cash flow before debt payments, by industry risk group
	// (1 low risk .. 3 high). Used only when the accounts of the company were not received.
	OperatingMargin map[int]float64
	// GrossMarginLevel is the gross margin of a healthy company of the industry group.
	GrossMarginLevel map[int]float64
}

func DefaultConfig() Config {
	return Config{
		MaxDSCR: 15, MaxDebtEBITDA: 10, MaxProfitability: 10, MaxLiquidity: 5, MaxOverdue: 15, MaxObligations: 10,
		MaxAge: 10, MaxOKVED: 5, MaxStatement: 5, MaxReputation: 5, MaxCollateral: 5, MaxPurpose: 5,
		GradeAFrom: 80, GradeBFrom: 50, GradeCFrom: 30,
		PD:                   map[Grade]float64{GradeA: 0.01, GradeB: 0.04, GradeC: 0.10, GradeD: 0.25},
		MaxLoad:              0.30,
		HardStopOverdueDays:  90,
		TechnicalOverdueDays: 5,
		MinRepaidCoverage:    0.5,
		MinAutoBKIScore:      650,
		MinDSCR:              1.25,
		OperatingMargin:      map[int]float64{1: 0.35, 2: 0.30, 3: 0.25},
		GrossMarginLevel:     map[int]float64{1: 0.30, 2: 0.25, 3: 0.20},
	}
}

// MonthlyPayment is the annuity payment in kopecks. Zero term returns 0.
func MonthlyPayment(amount int64, rateBP, termMonths int) int64 {
	if termMonths <= 0 || amount <= 0 {
		return 0
	}
	if rateBP == 0 {
		return amount / int64(termMonths)
	}
	r := float64(rateBP) / 10000 / 12
	p := float64(amount) * r / (1 - math.Pow(1+r, -float64(termMonths)))
	return int64(math.Round(p))
}

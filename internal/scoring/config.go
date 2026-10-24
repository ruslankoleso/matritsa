package scoring

import "math"

// Config holds the model's parameters. Every number here is illustrative: the prototype has no
// historical default statistics, so the PD table and the point tiers are NOT calibrated on data.
type Config struct {
	// Maximum points of each borrower factor (they sum to 100).
	MaxAge, MaxStability, MaxExistingDebt, MaxBKI, MaxOverdue, MaxOKVED int
	// PD is the annual probability of default by grade.
	PD map[Grade]float64
	// MaxLoad is the highest share of average monthly revenue that all loan payments may take.
	MaxLoad             float64
	HardStopOverdueDays int
	// MinRepaidCoverage is how much of the requested amount the borrower must already have repaid in other
	// loans (repaid total / requested amount) for an automatic offer. It is checked next to the request and
	// never changes the class of the borrower.
	MinRepaidCoverage float64
	// MinDSCR is the lowest cover of all loan payments by monthly cash flow (cash flow / payments).
	MinDSCR float64
	// OperatingMargin is the share of revenue left as cash flow before debt payments, by industry risk group
	// (1 low risk .. 3 high). A simplification: the bank has revenue, not a cash flow statement.
	OperatingMargin map[int]float64
}

func DefaultConfig() Config {
	return Config{
		MaxAge: 15, MaxStability: 20, MaxExistingDebt: 20, MaxBKI: 25, MaxOverdue: 15, MaxOKVED: 5,
		PD:                  map[Grade]float64{GradeA: 0.01, GradeB: 0.03, GradeC: 0.06, GradeD: 0.12, GradeE: 0.25},
		MaxLoad:             0.30,
		HardStopOverdueDays: 90,
		MinRepaidCoverage:   0.5,
		MinDSCR:             1.25,
		OperatingMargin:     map[int]float64{1: 0.35, 2: 0.30, 3: 0.25},
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

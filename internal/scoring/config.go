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
	// StableCV and ModerateCV are the limits of the spread of monthly revenue (coefficient of variation): up to
	// StableCV stable, up to ModerateCV moderate, above strong fluctuations.
	StableCV, ModerateCV float64
	// LeverageStop is the Debt/EBITDA above which the machine does not refuse: a person looks at the application.
	LeverageStop float64
	// RecurringOverdues is how many late payments in the last 12 months lower the tier of the criterion by one.
	RecurringOverdues int
	// CourtYears is how far back court cases count; LargeSuitShare the share of the annual revenue above which
	// a suit against the company is large.
	CourtYears     float64
	LargeSuitShare float64
	// BalanceCover* are the tiers of the liquidity taken from the statement, months of expenses on the account.
	BalanceCoverGood, BalanceCoverOK float64
	// RetailCurrentRatio is the comfortable current ratio of trade, where goods turn over fast.
	RetailCurrentRatio float64
	// CollateralDiscount is the share of the market value of a pledge that the bank deducts (the middle of the
	// ranges of the analyst: real estate 20-40%, equipment 20-50%, vehicles 30-70%, goods in turnover 40-60%).
	CollateralDiscount map[string]float64
	// PurposeAmountTolerance is the allowed difference between the amount in the confirming document and the request.
	PurposeAmountTolerance float64
	// RiskFreeBP is the risk-free rate in the pricing: rate = risk-free + PD x LGD + operating costs of the product.
	// Illustrative; with the operating costs it makes the base rate of a product (products.OperatingCostBP).
	RiskFreeBP int
}

func DefaultConfig() Config {
	return Config{
		MaxDSCR: 15, MaxDebtEBITDA: 10, MaxProfitability: 10, MaxLiquidity: 5, MaxOverdue: 15, MaxObligations: 10,
		MaxAge: 10, MaxOKVED: 5, MaxStatement: 5, MaxReputation: 5, MaxCollateral: 5, MaxPurpose: 5,
		GradeAFrom: 80, GradeBFrom: 50, GradeCFrom: 30,
		PD:                     map[Grade]float64{GradeA: 0.01, GradeB: 0.04, GradeC: 0.10, GradeD: 0.25},
		MaxLoad:                0.30,
		HardStopOverdueDays:    90,
		TechnicalOverdueDays:   3,
		MinRepaidCoverage:      0.5,
		MinAutoBKIScore:        650,
		MinDSCR:                1.25,
		OperatingMargin:        map[int]float64{1: 0.35, 2: 0.30, 3: 0.25},
		GrossMarginLevel:       map[int]float64{1: 0.30, 2: 0.25, 3: 0.20},
		StableCV:               0.10,
		ModerateCV:             0.25,
		LeverageStop:           4.0,
		RecurringOverdues:      2,
		CourtYears:             3,
		LargeSuitShare:         0.05,
		BalanceCoverGood:       1.5,
		BalanceCoverOK:         1.0,
		RetailCurrentRatio:     1.2,
		CollateralDiscount:     map[string]float64{CollateralRealEstate: 0.30, CollateralEquipment: 0.35, CollateralTransport: 0.50, CollateralGoods: 0.50},
		PurposeAmountTolerance: 0.10,
		RiskFreeBP:             1400,
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

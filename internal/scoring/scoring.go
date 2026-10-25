// Package scoring is a transparent points-based risk model: every criterion
// returns its own points and a plain-language note, and the sum is the score.
package scoring

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"matritsa/internal/integrations"
)

type Grade string

const (
	GradeA Grade = "A"
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeD Grade = "D"
)

// GradeFor maps a 0..100 score to a risk grade.
func GradeFor(score int, cfg Config) Grade {
	switch {
	case score >= cfg.GradeAFrom:
		return GradeA
	case score >= cfg.GradeBFrom:
		return GradeB
	case score >= cfg.GradeCFrom:
		return GradeC
	default:
		return GradeD
	}
}

// Quality is how a grade maps to the quality categories of a loan under Bank of Russia Regulation 590-P and
// to the range of the loss reserve. The mapping is the analyst's; the bank sets the exact reserve.
type Quality struct {
	Category string `json:"category"`
	Reserve  string `json:"reserve"`
	Risk     string `json:"risk"`
}

func QualityOf(g Grade) Quality {
	switch g {
	case GradeA:
		return Quality{"I–II категория (стандартные и нестандартные ссуды)", "0–20 %", "низкий риск"}
	case GradeB:
		return Quality{"II–III категория (нестандартные и сомнительные ссуды)", "1–50 %", "умеренный риск"}
	case GradeC:
		return Quality{"III–IV категория (сомнительные и проблемные ссуды)", "21–100 %", "высокий риск"}
	default:
		return Quality{"IV–V категория (проблемные и безнадёжные ссуды)", "51–100 %", "критический риск"}
	}
}

// Collateral levels of the request.
const (
	CollateralNone    = "none"
	CollateralPartial = "partial"
	CollateralFull    = "full"
)

// Input is everything the model needs; amounts are in kopecks.
// The score uses the borrower, the way the request is made out (collateral, purpose) and, through DSCR, the
// payment of the new loan: a bigger or longer loan covers worse and may get a lower class. The grade belongs to
// the loan, as the quality category of a loan does under 590-P.
type Input struct {
	// Borrower.
	CompanyAgeMonths   int
	MonthlyRevenue     []int64 // last months, newest last
	MonthlyObligations int64   // existing monthly loan payments
	BKI                *integrations.BKIReport
	EGRUL              integrations.EGRULRecord
	OKVEDGroup         int // 1 low risk .. 3 high risk
	// Request, apart from its size.
	Collateral  string // none | partial | full ("" counts as none)
	PurposeText string
	PurposeDocs bool // a document that backs the purpose is attached (invoice, contract)
	// Request size.
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
	// NoData: the bank has nothing to judge this criterion by. It earns no points, but the borrower is not
	// known to be bad on it: the decision tells the two apart.
	NoData bool `json:"noData,omitempty"`
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
	// MissingPoints is what the criteria without data could have given at most.
	MissingPoints int `json:"-"`
	// BKIScore is the bureau score (0 without a history); it is a condition of the automatic decision.
	BKIScore int `json:"-"`

	Payment             int64    `json:"payment"`
	DebtLoad            float64  `json:"debtLoad"`
	MaxAffordableAmount int64    `json:"maxAffordableAmount"` // the stricter of the two limits below
	HardStops           []string `json:"hardStops"`

	// Two measures that answer different questions: DebtLoad is the share of revenue that goes to
	// payments; DSCR is whether the cash flow covers them. Each gives its own limit on the amount.
	DSCR            float64 `json:"dscr"`
	MonthlyCashFlow int64   `json:"monthlyCashFlow"`
	MaxAmountByLoad int64   `json:"maxAmountByLoad"`
	MaxAmountByDSCR int64   `json:"maxAmountByDscr"`

	// HasCoverFigures is false for a record stored before the cover and repayment measures existed:
	// its zeros mean "never calculated", not a result.
	HasCoverFigures bool `json:"-"`
}

// Score evaluates the input. It never panics on empty or zero revenue.
func Score(in Input, cfg Config) Result {
	avg := averageRevenue(in.MonthlyRevenue)
	cash := cashFlow(in, avg, cfg)
	// A criterion is scored only from data the bank has. No data means no points, with a note saying so:
	// a young company or one without accounts cannot buy points with what is unknown about it.
	known := in.EGRUL.Statement != nil && avg > 0                  // the cash flow comes from the bank statement
	payment := MonthlyPayment(in.Amount, in.RateBP, in.TermMonths) // at the base rate of the product: the premium depends on the class
	factors := []Factor{
		dscrFactor(cash, in.MonthlyObligations+payment, known, cfg),
		debtEBITDAFactor(in.BKI, cash, known, cfg),
		profitabilityFactor(in, avg, cfg),
		liquidityFactor(in.EGRUL.Finance, cfg),
		overdueFactor(in.BKI, cfg),
		obligationsFactor(in, cfg),
		ageFactor(in.CompanyAgeMonths, cfg),
		okvedFactor(in.OKVEDGroup, cfg),
		statementFactor(in.EGRUL.Statement, cfg),
		reputationFactor(in.EGRUL, cfg),
		collateralFactor(in.Collateral, cfg),
		purposeFactor(in, cfg),
	}
	score, missing := 0, 0
	for _, f := range factors {
		score += f.Points
		if f.NoData {
			missing += f.Max
		}
	}
	grade := GradeFor(score, cfg)
	hard := hardStops(in, avg, cfg)
	if avg <= 0 || len(hard) > 0 {
		// Without proven revenue nothing can be repaid, and a hard stop ends the scoring: the worst grade.
		grade = GradeD
	}

	res := Result{Score: score, Grade: grade, Factors: factors, HardStops: hard, LGD: in.LGD, EAD: in.Amount, MissingPoints: missing}
	res.PD = cfg.PD[grade]
	res.CumulativePD = cumulativePD(res.PD, in.TermMonths)
	res.ELRate = res.CumulativePD * in.LGD
	res.ExpectedLoss = int64(math.Round(float64(in.Amount) * res.ELRate))
	if bp, ok := Premium(grade, in.LGD, cfg); ok {
		res.RiskPremiumBP = bp
	}

	if in.BKI != nil {
		res.RepaidTotal = in.BKI.TotalRepaid
		if in.BKI.HasHistory {
			res.BKIScore = in.BKI.Score
		}
		if in.Amount > 0 {
			res.RepaidCoverage = float64(in.BKI.TotalRepaid) / float64(in.Amount)
		}
	}

	res.Payment = payment
	if avg > 0 {
		res.DebtLoad = float64(in.MonthlyObligations+res.Payment) / float64(avg)
	}
	res.MonthlyCashFlow = cash
	if payments := in.MonthlyObligations + res.Payment; res.MonthlyCashFlow > 0 && payments > 0 {
		res.DSCR = float64(res.MonthlyCashFlow) / float64(payments)
	}
	res.MaxAmountByLoad = maxAmount(cfg.MaxLoad*float64(avg)-float64(in.MonthlyObligations), avg, in)
	res.MaxAmountByDSCR = maxAmount(float64(res.MonthlyCashFlow)/cfg.MinDSCR-float64(in.MonthlyObligations), res.MonthlyCashFlow, in)
	res.MaxAffordableAmount = min(res.MaxAmountByLoad, res.MaxAmountByDSCR)
	return res
}

// cashFlow is the monthly cash flow before debt service: receipts minus operating write-offs on the bank
// statement. Without the statement, only for the limit on the amount, an estimate by the industry group is used;
// it earns no points.
func cashFlow(in Input, avg int64, cfg Config) int64 {
	m := margin(in.OKVEDGroup, cfg)
	if st := in.EGRUL.Statement; st != nil {
		m = st.NOIMargin
	}
	if m <= 0 {
		return 0
	}
	return int64(float64(avg) * m)
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
// rate: (PD_grade - PD_A) x LGD. Grade D gets no offer at all.
func Premium(g Grade, lgd float64, cfg Config) (int, bool) {
	if g == GradeD {
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

func pct(v float64) int { return int(math.Round(v * 100)) }

func ratio(v float64) string { return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1) }

// 1. DSCR: does the cash flow cover all loan payments, the ones the company already has and the new one.
func dscrFactor(cash, payments int64, known bool, cfg Config) Factor {
	f := Factor{Key: "dscr", Title: "Покрытие долга денежным потоком (DSCR)", Max: cfg.MaxDSCR}
	switch {
	case !known:
		f.NoData, f.Note = true, "Нет данных: без анализа выписки денежный поток не определить"
		return f
	case payments <= 0:
		f.Points, f.Note = cfg.MaxDSCR, "Платежей по кредитам нет: денежный поток ничем не занят, запас полный"
		return f
	case cash <= 0:
		f.Note = "Денежного потока нет: платежи по кредитам нечем покрывать"
		return f
	}
	d := float64(cash) / float64(payments)
	switch {
	case d >= 1.5:
		f.Points, f.Note = cfg.MaxDSCR, fmt.Sprintf("DSCR %s с платежом по заявке: сильный запас прочности", ratio(d))
	case d >= 1.25:
		f.Points, f.Note = cfg.MaxDSCR*10/15, fmt.Sprintf("DSCR %s с платежом по заявке: стандартный банковский порог", ratio(d))
	case d >= 1.0:
		f.Points, f.Note = cfg.MaxDSCR*5/15, fmt.Sprintf("DSCR %s с платежом по заявке: пограничная зона, платежи покрываются впритык", ratio(d))
	default:
		f.Note = fmt.Sprintf("DSCR %s с платежом по заявке: денежного потока не хватает на платежи по кредитам", ratio(d))
	}
	return f
}

// 2. Debt to EBITDA: how many years of operating result the existing debt takes.
func debtEBITDAFactor(b *integrations.BKIReport, cash int64, known bool, cfg Config) Factor {
	f := Factor{Key: "debt_ebitda", Title: "Долговая нагрузка (Долг / EBITDA)", Max: cfg.MaxDebtEBITDA}
	switch {
	case b == nil || !b.HasHistory:
		f.NoData, f.Note = true, "Нет данных: кредитной истории в бюро нет, долг не подтверждён"
		return f
	case !known:
		f.NoData, f.Note = true, "Нет данных: без анализа выписки EBITDA не определить"
		return f
	case b.TotalDebt <= 0:
		f.Points, f.Note = cfg.MaxDebtEBITDA, "Ссудной задолженности нет"
		return f
	}
	if cash <= 0 {
		f.Note = "Операционного результата нет: долг нечем обслуживать"
		return f
	}
	d := float64(b.TotalDebt) / (float64(cash) * 12)
	switch {
	case d < 1.5:
		f.Points, f.Note = cfg.MaxDebtEBITDA, fmt.Sprintf("Долг / EBITDA %s: низкая нагрузка", ratio(d))
	case d <= 3.0:
		f.Points, f.Note = cfg.MaxDebtEBITDA/2, fmt.Sprintf("Долг / EBITDA %s: приемлемая нагрузка", ratio(d))
	default:
		f.Note = fmt.Sprintf("Долг / EBITDA %s: высокая нагрузка (выше 3)", ratio(d))
	}
	return f
}

// 3. Profitability and stability of revenue. Stability is the spread of monthly revenue, the trend is the
// second half of the period against the first, the margin is compared with a healthy company of the industry.
func profitabilityFactor(in Input, avg int64, cfg Config) Factor {
	f := Factor{Key: "profitability", Title: "Рентабельность и стабильность выручки", Max: cfg.MaxProfitability}
	rev := in.MonthlyRevenue
	switch {
	case avg <= 0 || len(rev) < 3:
		f.NoData, f.Note = true, "Нет данных: выручки за 3 месяца и более нет"
		return f
	case in.EGRUL.Finance == nil:
		f.NoData, f.Note = true, "Нет данных: бухгалтерской отчётности нет, рентабельность не определить"
		return f
	}
	var sq float64
	for _, v := range rev {
		d := float64(v - avg)
		sq += d * d
	}
	cv := math.Sqrt(sq/float64(len(rev))) / float64(avg)
	half := len(rev) / 2
	first, second := averageRevenue(rev[:half]), averageRevenue(rev[len(rev)-half:])
	trend := 0.0
	if first > 0 {
		trend = float64(second)/float64(first) - 1
	}
	gm := in.EGRUL.Finance // not nil, checked above
	level := cfg.GrossMarginLevel[in.OKVEDGroup]
	switch {
	case gm.GrossMargin <= 0:
		f.Note = "Бизнес убыточен: валовая прибыль не положительна"
	case trend <= -0.10:
		f.Note = fmt.Sprintf("Выручка падает: %d%% к началу периода", pct(trend))
	case cv < 0.15 && gm.GrossMargin >= level:
		f.Points = cfg.MaxProfitability
		f.Note = fmt.Sprintf("Выручка стабильна (колебания %d%%), рентабельность продаж %d%% выше отраслевого уровня %d%%", pct(cv), pct(gm.GrossMargin), pct(level))
	case cv < 0.30:
		f.Points = cfg.MaxProfitability / 2
		f.Note = fmt.Sprintf("Умеренные колебания выручки: коэффициент вариации %d%%", pct(cv))
		if gm.GrossMargin < level {
			f.Note += fmt.Sprintf(", рентабельность продаж %d%% ниже отраслевого уровня %d%%", pct(gm.GrossMargin), pct(level))
		}
	default:
		f.Note = fmt.Sprintf("Выручка нестабильна: коэффициент вариации %d%%", pct(cv))
	}
	return f
}

// 4. Current ratio: current assets over short-term liabilities.
func liquidityFactor(fin *integrations.Finance, cfg Config) Factor {
	f := Factor{Key: "liquidity", Title: "Ликвидность (текущая)", Max: cfg.MaxLiquidity}
	switch {
	case fin == nil:
		f.NoData, f.Note = true, "Нет данных: бухгалтерская отчётность не получена"
	case fin.CurrentRatio >= 1.5:
		f.Points, f.Note = cfg.MaxLiquidity, fmt.Sprintf("Коэффициент текущей ликвидности %s: комфортный", ratio(fin.CurrentRatio))
	case fin.CurrentRatio >= 1.0:
		f.Points, f.Note = cfg.MaxLiquidity*3/5, fmt.Sprintf("Коэффициент текущей ликвидности %s: пограничный", ratio(fin.CurrentRatio))
	default:
		f.Note = fmt.Sprintf("Коэффициент текущей ликвидности %s: ниже 1, риск кассового разрыва", ratio(fin.CurrentRatio))
	}
	return f
}

// 5. Overdue payments in the bureau history over 36 months.
func overdueFactor(b *integrations.BKIReport, cfg Config) Factor {
	f := Factor{Key: "overdue", Title: "Просрочки за 36 месяцев", Max: cfg.MaxOverdue}
	switch {
	case b == nil || !b.HasHistory:
		f.NoData, f.Note = true, "Нет данных: кредитной истории в бюро нет, просрочки оценить нельзя"
	case b.MaxOverdueDays == 0:
		f.Points, f.Note = cfg.MaxOverdue, "Просрочек по платежам не было"
	case b.MaxOverdueDays <= cfg.TechnicalOverdueDays:
		f.Points, f.Note = cfg.MaxOverdue*10/15, fmt.Sprintf("Только техническая просрочка: до %d дн.", b.MaxOverdueDays)
	case b.MaxOverdueDays <= 30:
		f.Points, f.Note = cfg.MaxOverdue*5/15, fmt.Sprintf("Просрочка до %d дн.", b.MaxOverdueDays)
	default:
		f.Note = fmt.Sprintf("Просрочка более 30 дней (до %d дн.)", b.MaxOverdueDays)
	}
	return f
}

// 6. Current obligations: whether there are any and how they are served.
func obligationsFactor(in Input, cfg Config) Factor {
	f := Factor{Key: "obligations", Title: "Действующие обязательства", Max: cfg.MaxObligations}
	b := in.BKI
	active := in.MonthlyObligations > 0 || (b != nil && b.HasHistory && b.ActiveLoans > 0)
	switch {
	case b == nil || !b.HasHistory:
		f.NoData, f.Note = true, "Нет данных: кредитной истории в бюро нет, обязательства не подтверждены"
	case b.Restructured:
		f.Note = "По действующему кредиту была реструктуризация: признак финансовых затруднений"
	case !active:
		f.Points, f.Note = cfg.MaxObligations, "Действующих кредитных обязательств нет"
	default:
		f.Points, f.Note = cfg.MaxObligations/2, "Есть действующие кредиты, обслуживаются без реструктуризации"
	}
	return f
}

// 7. Age of the business.
func ageFactor(months int, cfg Config) Factor {
	f := Factor{Key: "company_age", Title: "Возраст бизнеса", Max: cfg.MaxAge}
	switch {
	case months > 60:
		f.Points, f.Note = cfg.MaxAge, fmt.Sprintf("Срок работы %s: зона низкого риска", years(months))
	case months >= 36:
		f.Points, f.Note = cfg.MaxAge*7/10, fmt.Sprintf("Срок работы %s: зона среднего риска", years(months))
	case months >= 12:
		f.Points, f.Note = cfg.MaxAge*4/10, fmt.Sprintf("Срок работы %s: зона повышенного риска", years(months))
	default:
		f.Note = fmt.Sprintf("Срок работы %s: меньше года, истории недостаточно", years(months))
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

// 8. Industry risk group.
func okvedFactor(group int, cfg Config) Factor {
	f := Factor{Key: "okved_risk", Title: "Отраслевой риск", Max: cfg.MaxOKVED}
	switch group {
	case 0:
		f.NoData, f.Note = true, "Нет данных: отрасль не определена"
	case 1:
		f.Points, f.Note = cfg.MaxOKVED, "Отрасль с устойчивым спросом: низкий риск"
	case 2:
		f.Points, f.Note = cfg.MaxOKVED*3/5, "Отрасль со средней зависимостью от рыночной конъюнктуры: средний риск"
	default:
		f.Note = "Отрасль с повышенной зависимостью от рыночной конъюнктуры: высокий риск"
	}
	return f
}

// 9. Anomalies found in the bank statement.
func statementFactor(st *integrations.Statement, cfg Config) Factor {
	f := Factor{Key: "statement", Title: "Аномалии в выписке", Max: cfg.MaxStatement}
	switch {
	case st == nil:
		f.NoData, f.Note = true, "Нет данных: выписка не проанализирована"
	case st.Anomaly == 0:
		f.Points, f.Note = cfg.MaxStatement, "Аномалий в выписке не найдено"
	case st.Anomaly == 1:
		f.Points, f.Note = cfg.MaxStatement*2/5, "Незначительные аномалии: единичные скачки оборотов"
	default:
		f.Note = "Существенные аномалии: круговые переводы или сомнительные контрагенты"
	}
	return f
}

// 10. Reputation and court cases.
func reputationFactor(e integrations.EGRULRecord, cfg Config) Factor {
	f := Factor{Key: "reputation", Title: "Репутация и судебные споры", Max: cfg.MaxReputation}
	switch {
	case e.Bankrupt:
		f.Note = "Идёт процедура банкротства"
	case e.Litigation == 0:
		f.Points, f.Note = cfg.MaxReputation, "Репутация чистая: крупных споров нет"
	case e.Litigation == 1:
		f.Points, f.Note = cfg.MaxReputation*2/5, "Есть незначительные судебные споры"
	default:
		f.Note = "Есть крупные иски к компании или её руководителю"
	}
	return f
}

// 11. Collateral or a guarantee.
func collateralFactor(level string, cfg Config) Factor {
	f := Factor{Key: "collateral", Title: "Обеспечение", Max: cfg.MaxCollateral}
	switch level {
	case CollateralFull:
		f.Points, f.Note = cfg.MaxCollateral, "Есть залог или поручительство, покрывающие кредит"
	case CollateralPartial:
		f.Points, f.Note = cfg.MaxCollateral*2/5, "Обеспечение покрывает кредит частично"
	default:
		f.Note = "Обеспечения нет"
	}
	return f
}

// 12. Does the purpose and the amount have a backing: a document, or at least a stated purpose.
func purposeFactor(in Input, cfg Config) Factor {
	f := Factor{Key: "purpose", Title: "Соответствие цели и суммы", Max: cfg.MaxPurpose}
	switch {
	case in.PurposeDocs:
		f.Points, f.Note = cfg.MaxPurpose, "Цель и сумма подтверждены документом (счёт или договор)"
	case utf8.RuneCountInString(strings.TrimSpace(in.PurposeText)) >= 20:
		f.Points, f.Note = cfg.MaxPurpose*2/5, "Цель названа, но документов, подтверждающих сумму, нет: нужно уточнение"
	default:
		f.NoData, f.Note = true, "Нет данных: цель не описана и документов нет"
	}
	return f
}

// margin is the cash-flow share of revenue for an industry group; unknown groups use the most cautious one.
func margin(group int, cfg Config) float64 {
	if m, ok := cfg.OperatingMargin[group]; ok {
		return m
	}
	return cfg.OperatingMargin[3]
}

// maxAmount is the largest amount whose monthly payment fits into room (money per month left for the new
// payment). base is the revenue or cash flow the room was taken from: without it nothing can be repaid.
func maxAmount(room float64, base int64, in Input) int64 {
	if base <= 0 || in.TermMonths <= 0 || room <= 0 {
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

	DSCR            float64 `json:"dscr"`
	MonthlyCashFlow int64   `json:"monthlyCashFlow"`
	MaxAmountByLoad int64   `json:"maxAmountByLoad"`
	MaxAmountByDSCR int64   `json:"maxAmountByDscr"`

	// V is the version of the figures: 2 added the cover and repayment measures.
	V int `json:"v"`
}

func (r Result) Metrics() Metrics {
	return Metrics{PD: r.PD, CumulativePD: r.CumulativePD, LGD: r.LGD, EAD: r.EAD, ExpectedLoss: r.ExpectedLoss, ELRate: r.ELRate, RiskPremiumBP: r.RiskPremiumBP, Payment: r.Payment, RepaidTotal: r.RepaidTotal, RepaidCoverage: r.RepaidCoverage,
		DSCR: r.DSCR, MonthlyCashFlow: r.MonthlyCashFlow, MaxAmountByLoad: r.MaxAmountByLoad, MaxAmountByDSCR: r.MaxAmountByDSCR, V: 2}
}

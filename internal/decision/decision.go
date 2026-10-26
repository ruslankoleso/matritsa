// Package decision turns a scoring result into a credit decision for a product.
// Scoring says how risky the borrower is; the decision also depends on the
// product, the requested amount and bank limits.
package decision

import (
	"fmt"
	"strings"

	"matritsa/internal/products"
	"matritsa/internal/scoring"
)

type Outcome string

const (
	AutoOffer Outcome = "auto_offer"
	Review    Outcome = "review"
	Reject    Outcome = "reject"
)

const rub = int64(100)

// roundStep keeps counter-offers tidy: amounts are rounded down to 10 000 ₽.
const roundStep = 10_000 * rub

type Offer struct {
	Amount         int64  `json:"amount"`
	TermMonths     int    `json:"termMonths"`
	RateBP         int    `json:"rateBp"`
	MonthlyPayment int64  `json:"monthlyPayment"`
	Source         string `json:"source"` // auto | recommended | analyst
}

type Decision struct {
	Outcome     Outcome  `json:"outcome"`
	Offer       *Offer   `json:"offer,omitempty"`
	Recommended *Offer   `json:"recommended,omitempty"`
	Reasons     []string `json:"reasons"`
}

// MakeOffer builds an offer for the grade; ok=false for grade D, which gets no offer.
func MakeOffer(p products.Product, g scoring.Grade, amount int64, term int, source string) (Offer, bool) {
	premium, ok := scoring.Premium(g, p.LGD, scoring.DefaultConfig())
	if !ok {
		return Offer{}, false
	}
	rate := p.BaseRateBP + premium
	return Offer{Amount: amount, TermMonths: term, RateBP: rate, MonthlyPayment: scoring.MonthlyPayment(amount, rate, term), Source: source}, true
}

// Rescorer scores the same borrower for another amount of the same request. The class depends on the
// amount (DSCR counts the payment of the new loan), so a smaller amount offered instead of the requested one
// is a loan of its own and gets its own class.
type Rescorer func(amount int64) scoring.Result

// Decide decides without rescoring: a counter-offer is priced with the class of the requested amount.
func Decide(res scoring.Result, p products.Product, amount int64, term int) Decision {
	return DecideRescoring(res, p, amount, term, nil)
}

// counterAmount is the amount to offer instead of the requested one when it does not fit the affordability limit.
func counterAmount(res scoring.Result, p products.Product, amount int64) (int64, []string) {
	rec := amount
	var reasons []string
	if rec > res.MaxAffordableAmount {
		rec = res.MaxAffordableAmount / roundStep * roundStep
		if rec < p.MinAmount {
			rec = p.MinAmount
		}
		if res.MaxAmountByDSCR > 0 && res.MaxAmountByDSCR < res.MaxAmountByLoad {
			reasons = append(reasons, "Запрошенная сумма превышает возможности денежного потока (покрытие платежей ниже допустимого): рекомендована меньшая")
		} else {
			reasons = append(reasons, "Запрошенная сумма превышает допустимую долговую нагрузку на выручку: рекомендована меньшая")
		}
	}
	if rec > p.MaxAmount {
		rec = p.MaxAmount
	}
	return rec, reasons
}

func rejectD(res scoring.Result) Decision {
	reason := fmt.Sprintf("Оценка риска не соответствует требованиям банка: класс D, %d баллов из 100", res.Score)
	if weak := zeroCriteria(res); weak != "" {
		reason += ". Нет баллов по критериям: " + weak
	}
	return Decision{Outcome: Reject, Reasons: []string{reason}}
}

// ReasonNeedsCollateral is the refusal of a class C loan that comes with no accepted collateral (decision 6.1).
const ReasonNeedsCollateral = "Класс C (высокий риск): заявка рассматривается только при обеспечении — залоге или поручительстве с подтверждающим документом"

// DecideRescoring decides on the scored request. Two things are never left to the machine (decisions 3.1, 7.1 and
// 7.2 of the analyst): a gap in the data (a criterion without data) blocks both an automatic offer and an automatic
// refusal, and Debt/EBITDA above the stop blocks an automatic refusal. Such an application goes to an analyst with
// the reasons and with what the system would have done. A stop factor is a condition of admission and still refuses at once.
func DecideRescoring(res scoring.Result, p products.Product, amount int64, term int, rescore Rescorer) Decision {
	d := decideKnown(res, p, amount, term, rescore)
	if len(res.HardStops) > 0 {
		return d
	}
	gaps := res.MissingPoints > 0
	leverage := res.HighLeverage && d.Outcome == Reject
	if !gaps && !leverage {
		return d
	}
	var holds []string
	if gaps {
		holds = append(holds, fmt.Sprintf("По критериям без данных (%s) баллы не начислены: автоматическое решение не принимается, заявку рассматривает сотрудник", noDataCriteria(res)))
	}
	if leverage {
		holds = append(holds, fmt.Sprintf("Долг / EBITDA %s выше %s: автоматический отказ не выдаётся, заявку рассматривает сотрудник",
			strings.Replace(fmt.Sprintf("%.1f", res.DebtEBITDA), ".", ",", 1), strings.Replace(fmt.Sprintf("%.0f", scoring.DefaultConfig().LeverageStop), ".", ",", 1)))
	}
	switch d.Outcome {
	case AutoOffer:
		rec := *d.Offer
		rec.Source = "recommended"
		return Decision{Outcome: Review, Recommended: &rec, Reasons: append(holds, "Без пробелов в данных система одобрила бы автоматически на запрошенных условиях")}
	case Reject:
		return Decision{Outcome: Review, Reasons: append(append(holds, "Иначе система автоматически отказала бы:"), d.Reasons...)}
	}
	d.Reasons = append(holds, d.Reasons...)
	return d
}

func decideKnown(res scoring.Result, p products.Product, amount int64, term int, rescore Rescorer) Decision {
	cfg := scoring.DefaultConfig()
	if len(res.HardStops) > 0 {
		return Decision{Outcome: Reject, Reasons: append([]string(nil), res.HardStops...)}
	}
	if res.MaxAffordableAmount < p.MinAmount {
		if res.Grade == scoring.GradeD {
			return rejectD(res)
		}
		return Decision{Outcome: Reject, Reasons: []string{
			fmt.Sprintf("Долговая нагрузка не позволяет выдать даже минимальную сумму по продукту (%d ₽)", p.MinAmount/rub),
		}}
	}

	// The class of the requested amount; a smaller amount offered instead may deserve another one.
	rec, reasons := counterAmount(res, p, amount)
	grade, counted := res.Grade, res.CollateralCounted
	if rec < amount && rescore != nil {
		r2 := rescore(rec)
		if len(r2.HardStops) > 0 || r2.Grade == scoring.GradeD {
			return rejectD(r2)
		}
		if r2.Grade != res.Grade {
			reasons = append(reasons, fmt.Sprintf("Рекомендованная сумма оценена отдельно: класс %s, %d баллов из 100 (на запрошенной сумме класс %s, %d баллов)", r2.Grade, r2.Score, res.Grade, res.Score))
		}
		grade = r2.Grade
		counted = r2.CollateralCounted
	} else if res.Grade == scoring.GradeD {
		return rejectD(res)
	}
	if grade == scoring.GradeC && !counted {
		return Decision{Outcome: Reject, Reasons: []string{ReasonNeedsCollateral}}
	}

	experienced := res.RepaidCoverage >= cfg.MinRepaidCoverage
	bureauOK := res.BKIScore >= cfg.MinAutoBKIScore
	if res.Grade == scoring.GradeA && amount <= p.AutoApproveLimit && amount <= res.MaxAffordableAmount && experienced && bureauOK {
		offer, _ := MakeOffer(p, res.Grade, amount, term, "auto")
		return Decision{Outcome: AutoOffer, Offer: &offer, Reasons: []string{"Низкий риск и сумма в пределах лимита: решение принято автоматически"}}
	}

	if !experienced {
		reasons = append(reasons, fmt.Sprintf("Недостаточный опыт погашения кредитов: погашено на %d ₽, это %d%% от запрошенной суммы (для автоматического решения нужно не менее %d%%)",
			res.RepaidTotal/rub, int(res.RepaidCoverage*100), int(cfg.MinRepaidCoverage*100)))
	}
	if !bureauOK && res.BKIScore > 0 {
		reasons = append(reasons, fmt.Sprintf("Балл бюро кредитных историй %d ниже %d: для автоматического решения нужен не ниже", res.BKIScore, cfg.MinAutoBKIScore))
	}
	switch {
	case res.Grade != scoring.GradeA:
		reasons = append(reasons, "Оценка риска в промежуточной зоне: решение принимает сотрудник банка")
	case amount > p.AutoApproveLimit:
		reasons = append(reasons, "Сумма превышает лимит автоматического решения: требуется рассмотрение сотрудником")
	}
	offer, _ := MakeOffer(p, grade, rec, term, "recommended")
	return Decision{Outcome: Review, Recommended: &offer, Reasons: reasons}
}

// zeroCriteria names the criteria that earned no points, in the order of the scorecard.
func zeroCriteria(res scoring.Result) string {
	var names []string
	for _, f := range res.Factors {
		if f.Points == 0 && f.Max > 0 {
			names = append(names, f.Title)
		}
	}
	return strings.Join(names, "; ")
}

// noDataCriteria names the criteria the bank had nothing to judge.
func noDataCriteria(res scoring.Result) string {
	var names []string
	for _, f := range res.Factors {
		if f.NoData {
			names = append(names, f.Title)
		}
	}
	return strings.Join(names, "; ")
}

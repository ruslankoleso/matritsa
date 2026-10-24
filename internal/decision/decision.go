// Package decision turns a scoring result into a credit decision for a product.
// Scoring says how risky the borrower is; the decision also depends on the
// product, the requested amount and bank limits.
package decision

import (
	"fmt"

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

// MakeOffer builds an offer for the grade; ok=false for grade E.
func MakeOffer(p products.Product, g scoring.Grade, amount int64, term int, source string) (Offer, bool) {
	premium, ok := scoring.Premium(g, p.LGD, scoring.DefaultConfig())
	if !ok {
		return Offer{}, false
	}
	rate := p.BaseRateBP + premium
	return Offer{Amount: amount, TermMonths: term, RateBP: rate, MonthlyPayment: scoring.MonthlyPayment(amount, rate, term), Source: source}, true
}

func Decide(res scoring.Result, p products.Product, amount int64, term int) Decision {
	if len(res.HardStops) > 0 {
		return Decision{Outcome: Reject, Reasons: append([]string(nil), res.HardStops...)}
	}
	if res.Grade == scoring.GradeE {
		return Decision{Outcome: Reject, Reasons: []string{"Оценка риска не соответствует требованиям банка для выдачи кредита"}}
	}
	if res.MaxAffordableAmount < p.MinAmount {
		return Decision{Outcome: Reject, Reasons: []string{
			fmt.Sprintf("Долговая нагрузка не позволяет выдать даже минимальную сумму по продукту (%d ₽)", p.MinAmount/rub),
		}}
	}

	if res.Grade == scoring.GradeA && amount <= p.AutoApproveLimit && amount <= res.MaxAffordableAmount {
		offer, _ := MakeOffer(p, res.Grade, amount, term, "auto")
		return Decision{Outcome: AutoOffer, Offer: &offer, Reasons: []string{"Низкий риск и сумма в пределах лимита: решение принято автоматически"}}
	}

	recAmount := amount
	var reasons []string
	if recAmount > res.MaxAffordableAmount {
		recAmount = res.MaxAffordableAmount / roundStep * roundStep
		if recAmount < p.MinAmount {
			recAmount = p.MinAmount
		}
		reasons = append(reasons, "Запрошенная сумма превышает допустимую нагрузку на бизнес: рекомендована меньшая")
	}
	if recAmount > p.MaxAmount {
		recAmount = p.MaxAmount
	}
	if res.Grade == scoring.GradeA {
		reasons = append(reasons, "Сумма превышает лимит автоматического решения: требуется рассмотрение сотрудником")
	} else {
		reasons = append(reasons, "Оценка риска в промежуточной зоне: решение принимает сотрудник банка")
	}
	rec, _ := MakeOffer(p, res.Grade, recAmount, term, "recommended")
	return Decision{Outcome: Review, Recommended: &rec, Reasons: reasons}
}

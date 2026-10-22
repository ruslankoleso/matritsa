package ai

import (
	"context"
	"fmt"
	"math"
	"strings"

	"matritsa/internal/decision"
	"matritsa/internal/scoring"
)

type Explainer struct{ llm LLM }

func NewExplainer(llm LLM) *Explainer { return &Explainer{llm: llm} }

const explainSystem = `Ты помогаешь объяснить кредитное решение простым русским языком в 3-4 предложениях.
Используй ТОЛЬКО факты из сообщения, ничего не выдумывай, не меняй цифры и итог решения. Без канцелярита и без markdown.`

// Explain returns a plain-language explanation and its source (ai | rules).
// audience is "staff" (analyst) or "client".
func (e *Explainer) Explain(ctx context.Context, res scoring.Result, d decision.Decision, audience string) (string, string) {
	facts := fallbackText(res, d, audience)
	if e.llm != nil {
		who := "аналитика банка"
		if audience == "client" {
			who = "клиента (предпринимателя, без банковской терминологии)"
		}
		reply, err := e.llm.Complete(ctx, explainSystem, "Аудитория: "+who+".\nФакты:\n"+facts)
		if err == nil && strings.TrimSpace(reply) != "" {
			return strings.TrimSpace(reply), "ai"
		}
	}
	return facts, "rules"
}

func rubles(kopecks int64) string {
	s := fmt.Sprintf("%d", kopecks/100)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	return string(out) + " ₽"
}

func offerText(o *decision.Offer) string {
	return fmt.Sprintf("%s на %d мес. под %.1f%% годовых (платёж около %s в месяц)", rubles(o.Amount), o.TermMonths, float64(o.RateBP)/100, rubles(o.MonthlyPayment))
}

func fallbackText(res scoring.Result, d decision.Decision, audience string) string {
	var strong, weak []scoring.Factor
	for _, f := range res.Factors {
		switch {
		case f.Points == f.Max:
			strong = append(strong, f)
		case f.Points*2 < f.Max:
			weak = append(weak, f)
		}
	}
	if audience == "client" {
		return clientText(strong, weak, d)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Класс риска %s — %d баллов из 100.", res.Grade, res.Score)
	if len(strong) > 0 {
		b.WriteString(" В плюс: ")
		b.WriteString(joinNotes(strong))
		b.WriteString(".")
	}
	if len(weak) > 0 {
		b.WriteString(" Тянет вниз: ")
		b.WriteString(joinNotes(weak))
		b.WriteString(".")
	}
	if res.EAD > 0 {
		fmt.Fprintf(&b, " Ожидаемые потери: %s (PD за срок %s × LGD %d%% × сумма %s).", rubles(res.ExpectedLoss), percent(res.CumulativePD), int(math.Round(res.LGD*100)), rubles(res.EAD))
	}
	b.WriteString(" " + outcomeText(d))
	return b.String()
}

func percent(v float64) string {
	s := strings.Replace(fmt.Sprintf("%.1f", v*100), ".", ",", 1)
	return strings.TrimSuffix(s, ",0") + "%"
}

func joinNotes(fs []scoring.Factor) string {
	parts := make([]string, len(fs))
	for i, f := range fs {
		parts[i] = strings.TrimRight(f.Note, ".")
	}
	return strings.Join(parts, "; ")
}

func outcomeText(d decision.Decision) string {
	switch d.Outcome {
	case decision.AutoOffer:
		return "Решение принято автоматически: предложение на запрошенных условиях — " + offerText(d.Offer) + "."
	case decision.Review:
		if d.Recommended != nil {
			return "Рекомендация системы — предложить " + offerText(d.Recommended) + ". Окончательное решение за сотрудником банка."
		}
		return "Окончательное решение за сотрудником банка."
	default:
		return "Рекомендация — отказать. Причины: " + strings.Join(d.Reasons, "; ") + "."
	}
}

var clientStrong = map[string]string{
	"company_age": "ваш бизнес работает достаточно давно", "revenue_stability": "выручка ровная от месяца к месяцу",
	"existing_debt": "действующие кредиты почти не нагружают бизнес", "bki_score": "у вас хорошая кредитная история",
	"overdue": "вы платите без просрочек", "okved_risk": "в вашей отрасли устойчивый спрос",
}

var clientWeak = map[string]string{
	"company_age": "бизнес пока молодой", "revenue_stability": "выручка заметно колеблется от месяца к месяцу",
	"existing_debt": "уже действующие платежи по кредитам забирают заметную часть выручки", "bki_score": "кредитная история пока не позволяет уверенно оценить надёжность",
	"overdue": "в прошлом были просрочки", "okved_risk": "отрасль заметно зависит от рыночной ситуации",
}

func pick(fs []scoring.Factor, dict map[string]string) []string {
	var out []string
	for _, f := range fs {
		if s, ok := dict[f.Key]; ok {
			out = append(out, s)
		}
	}
	return out
}

func clientText(strong, weak []scoring.Factor, d decision.Decision) string {
	var b strings.Builder
	if s := pick(strong, clientStrong); len(s) > 0 {
		b.WriteString("Что нам понравилось: " + strings.Join(s, ", ") + ". ")
	}
	if w := pick(weak, clientWeak); len(w) > 0 {
		b.WriteString("На что мы обратили внимание: " + strings.Join(w, ", ") + ". ")
	}
	switch d.Outcome {
	case decision.AutoOffer:
		b.WriteString("Поэтому готовы предложить " + offerText(d.Offer) + ".")
	case decision.Review:
		if d.Recommended != nil {
			b.WriteString("Поэтому можем предложить " + offerText(d.Recommended) + ". Сотрудник банка ещё посмотрит заявку и подтвердит условия.")
		} else {
			b.WriteString("Сотрудник банка ещё посмотрит заявку и подтвердит условия.")
		}
	default:
		b.WriteString("К сожалению, сейчас мы не можем одобрить заявку. " + strings.Join(d.Reasons, ". ") + ". Можно подать заявку на меньшую сумму или другой продукт.")
	}
	return strings.TrimSpace(b.String())
}

package ai

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"matritsa/internal/decision"
	"matritsa/internal/scoring"
)

type Explainer struct{ llm LLM }

func NewExplainer(llm LLM) *Explainer { return &Explainer{llm: llm} }

const explainSystem = `Ты помогаешь объяснить кредитное решение ясным деловым русским языком в 3-4 предложениях.
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
		if err == nil && explanationSound(strings.TrimSpace(reply), facts) {
			return strings.TrimSpace(reply), "ai"
		}
	}
	return facts, "rules"
}

// explanationSound checks the model's retelling the way the chat checks its answers: every number comes from
// the facts, nothing is promised, and it stays a short retelling. A rejected retelling gives way to the facts.
func explanationSound(reply, facts string) bool {
	if reply == "" || utf8.RuneCountInString(reply) > 3*utf8.RuneCountInString(facts)+200 {
		return false
	}
	low := strings.ToLower(reply)
	for _, m := range promiseMarks {
		if strings.Contains(low, m) {
			return false
		}
	}
	return figuresKnown(reply, facts)
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
	rate := strings.Replace(fmt.Sprintf("%.1f", float64(o.RateBP)/100), ".", ",", 1)
	return fmt.Sprintf("%s на %d мес. под %s%% годовых (платёж около %s в месяц)", rubles(o.Amount), o.TermMonths, rate, rubles(o.MonthlyPayment))
}

func fallbackText(res scoring.Result, d decision.Decision, audience string) string {
	var strong, weak, unknown []scoring.Factor
	for _, f := range res.Factors {
		switch {
		case f.NoData: // nothing is known about it, so it is neither a strength nor a risk
			unknown = append(unknown, f)
		case f.Points == f.Max:
			strong = append(strong, f)
		case f.Points*2 < f.Max:
			weak = append(weak, f)
		}
	}
	if audience == "client" {
		return clientText(strong, weak, len(unknown) > 0, d)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Класс риска %s — %d баллов из 100.", res.Grade, res.Score)
	if len(strong) > 0 {
		b.WriteString(" Сильные стороны: ")
		b.WriteString(joinNotes(strong))
		b.WriteString(".")
	}
	if len(weak) > 0 {
		b.WriteString(" Факторы риска: ")
		b.WriteString(joinNotes(weak))
		b.WriteString(".")
	}
	if len(unknown) > 0 {
		titles := make([]string, len(unknown))
		for i, f := range unknown {
			titles[i] = f.Title
		}
		b.WriteString(" Нет данных по критериям: " + strings.Join(titles, "; ") + " (баллов за них нет).")
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
	"dscr": "денежный поток с запасом покрывает платежи по кредитам", "debt_ebitda": "долг небольшой по сравнению с операционным результатом",
	"profitability": "выручка стабильна, бизнес рентабелен", "liquidity": "ликвидность достаточная", "overdue": "просрочек по платежам не было",
	"obligations": "действующих кредитных обязательств нет", "company_age": "срок работы компании достаточен для оценки", "okved_risk": "отрасль с устойчивым спросом",
	"statement": "в банковской выписке нет аномалий", "reputation": "репутация компании чистая", "collateral": "есть обеспечение",
	"purpose": "цель и сумма подтверждены документами",
}

var clientWeak = map[string]string{
	"dscr": "денежного потока мало для платежей по действующим кредитам", "debt_ebitda": "долг велик по сравнению с операционным результатом",
	"profitability": "выручка нестабильна или бизнес мало прибылен", "liquidity": "ликвидность низкая", "overdue": "ранее были просрочки по платежам",
	"obligations": "есть действующие кредиты", "company_age": "срок работы компании небольшой", "okved_risk": "отрасль зависит от рыночной конъюнктуры",
	"statement": "в банковской выписке есть аномалии", "reputation": "есть судебные споры", "collateral": "обеспечения нет",
	"purpose": "цель и сумма не подтверждены документами",
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

func clientText(strong, weak []scoring.Factor, gaps bool, d decision.Decision) string {
	var b strings.Builder
	if s := pick(strong, clientStrong); len(s) > 0 {
		b.WriteString("Сильные стороны: " + strings.Join(s, ", ") + ". ")
	}
	if w := pick(weak, clientWeak); len(w) > 0 {
		b.WriteString("Ограничивающие факторы: " + strings.Join(w, ", ") + ". ")
	}
	if gaps {
		b.WriteString("По части показателей данных пока нет, поэтому оценка осторожная. ")
	}
	switch d.Outcome {
	case decision.AutoOffer:
		b.WriteString("Банк готов предложить " + offerText(d.Offer) + ".")
	case decision.Review:
		if d.Recommended != nil {
			b.WriteString("Банк может предложить " + offerText(d.Recommended) + ". Условия подтвердит сотрудник банка.")
		} else {
			b.WriteString("Условия подтвердит сотрудник банка.")
		}
	default:
		advice, _ := decision.NextSteps(d.Reasons)
		b.WriteString("Банк не может одобрить заявку. " + strings.Join(d.Reasons, ". ") + ". " + advice)
	}
	return strings.TrimSpace(b.String())
}

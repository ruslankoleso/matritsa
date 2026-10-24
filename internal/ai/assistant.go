package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"matritsa/internal/products"
)

const rub = int64(100)

type Recommendation struct {
	ProductID  products.ID
	Amount     int64 // kopecks
	TermMonths int
	Rationale  string
	Source     string // ai | rules
}

type Assistant struct{ llm LLM }

// NewAssistant: a nil llm means "AI disabled" — rules only.
func NewAssistant(llm LLM) *Assistant { return &Assistant{llm: llm} }

const systemPrompt = `Ты помощник банка для владельцев малого бизнеса. По описанию потребности выбери ОДИН кредитный продукт.
Продукты: working_capital (оборотные средства: закупки, зарплаты, сезонные расходы), equipment (покупка оборудования и техники),
overdraft (кассовые разрывы, запас на счёте), express (небольшая сумма, срочно).
Ответь ТОЛЬКО JSON без пояснений: {"productId":"...","amountRub":число,"termMonths":число,"rationale":"1-2 предложения по-русски, ясным деловым языком, почему этот продукт"}.
Не принимай кредитных решений и не обещай одобрение.`

var ErrEmptyText = errors.New("опишите, на что нужны деньги")

func (a *Assistant) Recommend(ctx context.Context, text string) (Recommendation, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Recommendation{}, ErrEmptyText
	}
	if a.llm != nil {
		if rec, ok := a.fromModel(ctx, text); ok {
			return rec, nil
		}
	}
	return Fallback(text), nil
}

func (a *Assistant) fromModel(ctx context.Context, text string) (Recommendation, bool) {
	complete := a.llm.Complete
	if j, ok := a.llm.(JSONCompleter); ok {
		complete = j.CompleteJSON // the answer is parsed as JSON, so ask for nothing else
	}
	reply, err := complete(ctx, systemPrompt, text)
	if err != nil {
		return Recommendation{}, false
	}
	raw := extractJSON(reply)
	if raw == "" {
		return Recommendation{}, false
	}
	var out struct {
		ProductID string  `json:"productId"`
		AmountRub float64 `json:"amountRub"`
		Term      float64 `json:"termMonths"`
		Rationale string  `json:"rationale"`
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		// Small models slip in one predictable way: `"rationale\":\"text\"}`. The content is fine, so one
		// repair is tried; whatever it yields still goes through the checks below.
		if json.Unmarshal([]byte(strings.ReplaceAll(raw, `\"`, `"`)), &out) != nil {
			return Recommendation{}, false
		}
	}
	p, ok := products.Get(products.ID(out.ProductID))
	if !ok || strings.TrimSpace(out.Rationale) == "" {
		return Recommendation{}, false
	}
	amount, term := clamp(p, int64(out.AmountRub)*rub, int(out.Term))
	return Recommendation{ProductID: p.ID, Amount: amount, TermMonths: term, Rationale: strings.TrimSpace(out.Rationale), Source: "ai"}, true
}

func extractJSON(s string) string {
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return ""
	}
	return s[i : j+1]
}

func clamp(p products.Product, amount int64, term int) (int64, int) {
	if amount < p.MinAmount {
		amount = p.MinAmount
	}
	if amount > p.MaxAmount {
		amount = p.MaxAmount
	}
	if term < p.MinTerm {
		term = p.MinTerm
	}
	if term > p.MaxTerm {
		term = p.MaxTerm
	}
	return amount, term
}

var (
	equipmentWords = []string{"станок", "станк", "оборудован", "техник", "автомобил", "грузовик", "машин", "трактор", "печь", "печи", "холодильн", "принтер", "инструмент"}
	overdraftWords = []string{"касс", "разрыв", "овердрафт", "задержк", "отсрочк", "не успева"}
	workingWords   = []string{"зарплат", "закуп", "товар", "сырь", "оборот", "аренд", "налог", "реклам", "поставщик", "сезон"}
	amountRe       = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(млн|миллион\w*|тыс\w*|т\.\s?р\.?|к\b)?`)
)

func hasAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// Fallback picks a product by keywords and parses an amount like "2 млн" or "500 тысяч".
func Fallback(text string) Recommendation {
	t := strings.ToLower(text)
	var pid products.ID
	switch {
	case hasAny(t, equipmentWords):
		pid = products.Equipment
	case hasAny(t, overdraftWords):
		pid = products.Overdraft
	case hasAny(t, workingWords):
		pid = products.WorkingCapital
	default:
		pid = products.Express
	}
	p, _ := products.Get(pid)
	defaultAmount := map[products.ID]int64{products.WorkingCapital: 1_000_000, products.Equipment: 1_500_000, products.Overdraft: 500_000, products.Express: 200_000}
	defaultTerm := map[products.ID]int{products.WorkingCapital: 12, products.Equipment: 36, products.Overdraft: 12, products.Express: 6}
	amount := defaultAmount[pid] * rub
	if parsed, ok := parseAmount(t); ok {
		amount = parsed
	}
	amount, term := clamp(p, amount, defaultTerm[pid])
	return Recommendation{
		ProductID: pid, Amount: amount, TermMonths: term, Source: "rules",
		Rationale: fmt.Sprintf("По описанию подходит продукт «%s». %s", p.Title, p.Hint),
	}
}

func parseAmount(t string) (int64, bool) {
	for _, m := range amountRe.FindAllStringSubmatch(t, -1) {
		num, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		if err != nil {
			continue
		}
		unit := m[2]
		switch {
		case strings.HasPrefix(unit, "млн"), strings.HasPrefix(unit, "миллион"):
			return int64(num * 1_000_000 * float64(rub)), true
		case strings.HasPrefix(unit, "тыс"), strings.HasPrefix(unit, "т."), unit == "к":
			return int64(num * 1_000 * float64(rub)), true
		case num >= 10_000:
			return int64(num) * rub, true
		}
	}
	return 0, false
}

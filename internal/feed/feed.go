// Package feed builds the personal feed of offers, hints and next steps.
// It is a pure, explainable rule set: the same profile always gives the same feed.
package feed

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Card struct {
	ID            string
	Kind          string // product | action | status | tip | upsell
	Title         string
	Body          string
	Priority      int // lower is more important
	ProductID     string
	ApplicationID string
	CTALabel      string
	CTAHref       string
}

type AppSummary struct {
	ID             string
	Status         string
	ProductTitle   string
	OfferExpiresAt time.Time
}

type Profile struct {
	CompanyName        string
	OKVED              string
	CompanyAgeMonths   int
	AvgMonthlyRevenue  int64   // kopecks
	Volatility         float64 // coefficient of variation of monthly revenue
	MonthlyObligations int64
	Applications       []AppSummary
}

const maxCards = 6

var months = []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}

func ruDate(t time.Time) string { return fmt.Sprintf("%d %s", t.Day(), months[t.Month()-1]) }

func okvedClass(okved string) string {
	if len(okved) < 2 {
		return ""
	}
	return okved[:2]
}

func inRange(class string, from, to string) bool { return class >= from && class <= to }

// Build returns at most six cards sorted by priority.
func Build(p Profile) []Card {
	var cards []Card
	active := false

	for _, a := range p.Applications {
		c, isActive := appCard(a)
		if c != nil {
			cards = append(cards, *c)
		}
		active = active || isActive
	}

	class := okvedClass(p.OKVED)
	if !active {
		cards = append(cards, productCards(p, class)...)
	}
	if p.CompanyAgeMonths > 0 && p.CompanyAgeMonths < 12 {
		cards = append(cards, Card{
			ID: "tip-history", Kind: "tip", Priority: 30,
			Title: "Соберите платёжную историю",
			Body:  "Бизнесу меньше года — банку пока трудно оценить устойчивость. Регулярные поступления на счёт и своевременные платежи за пару месяцев заметно улучшат условия.",
		})
	}
	if t := sectorTip(class); t != nil {
		cards = append(cards, *t)
	}
	if len(cards) == 0 {
		cards = append(cards, Card{
			ID: "tip-start", Kind: "tip", Priority: 50,
			Title:    "С чего начать",
			Body:     "Расскажите, на что нужны деньги, — мы подберём подходящий продукт и посчитаем платёж.",
			CTALabel: "Подобрать кредит", CTAHref: "/cabinet/new",
		})
	}

	sort.SliceStable(cards, func(i, j int) bool { return cards[i].Priority < cards[j].Priority })
	if len(cards) > maxCards {
		cards = cards[:maxCards]
	}
	return cards
}

func appCard(a AppSummary) (*Card, bool) {
	href := "/cabinet/applications/" + a.ID
	switch a.Status {
	case "NEEDS_INFO":
		return &Card{ID: "app-" + a.ID + "-info", Kind: "action", Priority: 1, ApplicationID: a.ID,
			Title:    "Нужно небольшое уточнение",
			Body:     "Банк просит дополнить заявку «" + a.ProductTitle + "». Это займёт пару минут — и мы продолжим.",
			CTALabel: "Ответить", CTAHref: href}, true
	case "OFFER_MADE":
		body := "Для вас готово предложение по заявке «" + a.ProductTitle + "». Посмотрите условия и примите решение."
		if !a.OfferExpiresAt.IsZero() {
			body = "Предложение по заявке «" + a.ProductTitle + "» действует до " + ruDate(a.OfferExpiresAt) + ". Посмотрите условия и примите решение."
		}
		return &Card{ID: "app-" + a.ID + "-offer", Kind: "status", Priority: 2, ApplicationID: a.ID,
			Title: "Ваше предложение готово", Body: body, CTALabel: "Посмотреть условия", CTAHref: href}, true
	case "ANALYST_REVIEW":
		return &Card{ID: "app-" + a.ID + "-review", Kind: "status", Priority: 3, ApplicationID: a.ID,
			Title:    "Заявку рассматривает сотрудник банка",
			Body:     "Заявка «" + a.ProductTitle + "» у специалиста. Как только решение будет готово, вы увидите его здесь.",
			CTALabel: "Открыть заявку", CTAHref: href}, true
	case "SUBMITTED", "INTAKE_CHECK", "ENRICHMENT", "SCORING":
		return &Card{ID: "app-" + a.ID + "-progress", Kind: "status", Priority: 4, ApplicationID: a.ID,
			Title:    "Заявка в обработке",
			Body:     "Проверяем данные по заявке «" + a.ProductTitle + "». Обычно это занимает пару минут.",
			CTALabel: "Следить за статусом", CTAHref: href}, true
	case "SIGNING":
		return &Card{ID: "app-" + a.ID + "-signing", Kind: "action", Priority: 2, ApplicationID: a.ID,
			Title: "Остался последний шаг", Body: "Подпишите документы по заявке «" + a.ProductTitle + "».",
			CTALabel: "Подписать", CTAHref: href}, true
	case "DRAFT":
		return &Card{ID: "app-" + a.ID + "-draft", Kind: "action", Priority: 5, ApplicationID: a.ID,
			Title:    "У вас есть черновик заявки",
			Body:     "Заявка «" + a.ProductTitle + "» сохранена. Дозаполните её и отправьте.",
			CTALabel: "Продолжить", CTAHref: href}, false
	case "DISBURSED":
		return &Card{ID: "app-" + a.ID + "-upsell", Kind: "upsell", Priority: 20, ProductID: "overdraft",
			Title:    "Подушка безопасности для счёта",
			Body:     "Кредит получен — самое время подумать об овердрафте: он покроет кассовый разрыв, если клиенты заплатят позже.",
			CTALabel: "Подобрать овердрафт", CTAHref: "/cabinet/new?product=overdraft"}, false
	case "REJECTED":
		return &Card{ID: "app-" + a.ID + "-rejected", Kind: "tip", Priority: 6, ApplicationID: a.ID,
			Title:    "Что делать после отказа",
			Body:     "Посмотрите причины в заявке «" + a.ProductTitle + "». Часто помогает меньшая сумма или другой продукт.",
			CTALabel: "Посмотреть причины", CTAHref: href}, false
	}
	return nil, false
}

func productCards(p Profile, class string) []Card {
	var out []Card
	manufacturing := inRange(class, "10", "33")
	trade := inRange(class, "45", "47")
	food := class == "56"
	construction := inRange(class, "41", "43")
	transport := inRange(class, "49", "53")
	services := class == "96" || class == "56"

	if manufacturing || trade || food || p.AvgMonthlyRevenue > 0 {
		out = append(out, Card{ID: "prod-working-capital", Kind: "product", Priority: 10, ProductID: "working_capital",
			Title:    "Деньги на оборотные средства",
			Body:     workingCapitalBody(manufacturing, trade, food),
			CTALabel: "Рассчитать", CTAHref: "/cabinet/new?product=working_capital"})
	}
	if manufacturing || construction || transport {
		out = append(out, Card{ID: "prod-equipment", Kind: "product", Priority: 11, ProductID: "equipment",
			Title:    "Оборудование и техника в рассрочку",
			Body:     "Для вашей отрасли подойдёт кредит на оборудование: длинный срок — небольшой ежемесячный платёж.",
			CTALabel: "Рассчитать", CTAHref: "/cabinet/new?product=equipment"})
	}
	if p.Volatility >= 0.2 || services {
		out = append(out, Card{ID: "prod-overdraft", Kind: "product", Priority: 12, ProductID: "overdraft",
			Title:    "Овердрафт на случай кассового разрыва",
			Body:     "Выручка у вас бывает неровной. Овердрафт выручит в месяцы, когда платежи приходят позже расходов.",
			CTALabel: "Узнать условия", CTAHref: "/cabinet/new?product=overdraft"})
	}
	if p.CompanyAgeMonths < 12 || (p.AvgMonthlyRevenue > 0 && p.AvgMonthlyRevenue < 500_000*100) {
		out = append(out, Card{ID: "prod-express", Kind: "product", Priority: 13, ProductID: "express",
			Title:    "Экспресс-кредит: быстро и без лишних бумаг",
			Body:     "Небольшая сумма и быстрое решение — удобно, если деньги нужны срочно.",
			CTALabel: "Рассчитать", CTAHref: "/cabinet/new?product=express"})
	}
	return out
}

func workingCapitalBody(manufacturing, trade, food bool) string {
	switch {
	case food:
		return "Закупка продуктов, зарплаты, сезонные расходы — оборотный кредит закрывает повседневные траты общепита."
	case manufacturing:
		return "Сырьё и комплектующие дорожают — оборотный кредит позволяет закупиться заранее, а не по мере выручки."
	case trade:
		return "Товар нужно купить раньше, чем он продастся. Оборотный кредит закрывает этот промежуток."
	}
	return "Деньги на текущие расходы: закупки, зарплаты, сезонные провалы."
}

func sectorTip(class string) *Card {
	var title, body string
	switch {
	case class == "56":
		title, body = "Сезонность в общепите", "Планируйте закупки на высокий сезон заранее: так проще удержать нагрузку на платежи в спокойные месяцы."
	case inRange(class, "10", "33"):
		title, body = "Производство и сырьё", "Фиксируйте цену на сырьё заранее и сверяйте график платежей с графиком поступлений от заказчиков."
	case inRange(class, "41", "43"):
		title, body = "Строителям: аванс и этапы", "Банку проще согласовать сумму, если у вас есть договор и график этапов оплаты."
	case class == "62":
		title, body = "IT и сервисы", "Подписки и долгосрочные контракты хорошо показывают стабильность выручки — приложите их к заявке."
	case class == "96":
		title, body = "Услуги для населения", "Регулярные поступления по терминалу и онлайн-записи — сильный аргумент для банка."
	default:
		return nil
	}
	return &Card{ID: "tip-sector-" + class, Kind: "tip", Priority: 40, Title: title, Body: strings.TrimSpace(body)}
}

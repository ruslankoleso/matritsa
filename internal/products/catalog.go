// Package products is the (synthetic) catalog of credit products for small business.
package products

import "fmt"

type ID string

const (
	WorkingCapital ID = "working_capital"
	Equipment      ID = "equipment"
	Overdraft      ID = "overdraft"
	Express        ID = "express"
)

const rub = int64(100) // kopecks per ruble

// KindCollateralDoc is the document that proves the collateral; without it the collateral does not count.
const KindCollateralDoc = "collateral_doc"

// DocRequirement describes a document the client must provide for a product.
type DocRequirement struct {
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Why      string `json:"why"`
	Required bool   `json:"required"`
	// ConfirmsPurpose: the document confirms the purpose and the amount of the loan (criterion 12 of the scorecard).
	ConfirmsPurpose bool `json:"-"`
}

type Product struct {
	ID               ID     `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Hint             string `json:"hint"`
	MinAmount        int64  `json:"minAmount"`
	MaxAmount        int64  `json:"maxAmount"`
	MinTerm          int    `json:"minTerm"`
	MaxTerm          int    `json:"maxTerm"`
	BaseRateBP       int    `json:"baseRateBp"`
	AutoApproveLimit int64  `json:"autoApproveLimit"`
	// LGD is the share of the exposure the bank expects to lose if the borrower defaults (illustrative):
	// lower for products backed by the purchased asset, higher for unsecured ones.
	LGD       float64          `json:"-"`
	Documents []DocRequirement `json:"documents"`
	// OperatingCostBP is the operating cost in the pricing of the product: base rate = risk-free rate + PD of
	// class A x LGD + operating cost (the formula of the analyst), so that class A pays the base rate. Illustrative.
	OperatingCostBP int `json:"-"`
	// PurposeFromForm: the product asks for no confirming documents, the purpose described in the application
	// form is enough (a simplified product built for a quick decision).
	PurposeFromForm bool `json:"-"`
}

// FieldError points at an invalid request field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

var (
	docStatements = DocRequirement{Kind: "bank_statement", Title: "Выписка по расчётному счёту за 12 месяцев", Why: "По ней видно, как деньги приходят и уходят — так мы оценим ваши обороты", Required: true}
	docFinancials = DocRequirement{Kind: "financials", Title: "Финансовая отчётность или декларация", Why: "Показывает выручку и расходы бизнеса", Required: true}
	docInvoice    = DocRequirement{Kind: "invoice", Title: "Счёт или договор на покупку оборудования", Why: "Чтобы понимать, что именно вы покупаете и на какую сумму", Required: true, ConfirmsPurpose: true}
	docSpec       = DocRequirement{Kind: "specification", Title: "Спецификация оборудования", Why: "Подтверждает, что именно покупается; ускоряет рассмотрение", ConfirmsPurpose: true}
	docCollateral = DocRequirement{Kind: KindCollateralDoc, Title: "Документ на обеспечение: выписка из ЕГРН, ПТС, договор залога или поручительства", Why: "Без документа залог или поручительство не засчитываются"}
	docPurpose    = DocRequirement{Kind: "purpose_doc", Title: "Договор с поставщиком, счёт на оплату или смета расходов", Why: "Подтверждает, на что пойдут деньги и в каком объёме; для сезонных расходов приложите подтверждение потребности", ConfirmsPurpose: true}
	docCashPlan   = DocRequirement{Kind: "cash_plan", Title: "График поступлений и списаний", Why: "Показывает, когда возникают кассовые разрывы, которые покроет овердрафт", ConfirmsPurpose: true}
	// For an overdraft the statement itself proves the cash gaps.
	docStatementsProof = DocRequirement{Kind: "bank_statement", Title: docStatements.Title, Why: "По ней видно, как деньги приходят и уходят и когда возникают кассовые разрывы", Required: true, ConfirmsPurpose: true}
	docPassport        = DocRequirement{Kind: "passport", Title: "Паспорт руководителя", Why: "Нужен для подтверждения личности подписанта", Required: false}
)

var catalog = []Product{
	{
		ID: WorkingCapital, Title: "Кредит на оборотные средства",
		Description: "Деньги на текущие расходы бизнеса: закупить товар, выплатить зарплаты, пережить сезонный спад.",
		Hint:        "Подходит, если нужно закрыть повседневные расходы и деньги нужны на срок до двух лет.",
		MinAmount:   300_000 * rub, MaxAmount: 10_000_000 * rub, MinTerm: 3, MaxTerm: 24,
		BaseRateBP: 1650, OperatingCostBP: 190, AutoApproveLimit: 3_000_000 * rub, LGD: 0.60,
		Documents: []DocRequirement{docStatements, docFinancials, docPurpose, docCollateral},
	},
	{
		ID: Equipment, Title: "Кредит на оборудование",
		Description: "Покупка станка, техники, автомобиля или другого оборудования для работы.",
		Hint:        "Подходит, если вы знаете, что именно хотите купить, и нужен длинный срок, чтобы платежи были небольшими.",
		MinAmount:   500_000 * rub, MaxAmount: 20_000_000 * rub, MinTerm: 12, MaxTerm: 60,
		BaseRateBP: 1500, OperatingCostBP: 60, AutoApproveLimit: 3_000_000 * rub, LGD: 0.40,
		Documents: []DocRequirement{docStatements, docFinancials, docInvoice, docSpec, docCollateral},
	},
	{
		ID: Overdraft, Title: "Овердрафт",
		Description: "Запас на расчётном счёте: если денег временно не хватило, банк покрывает недостачу, а вы платите только за дни пользования.",
		Hint:        "Подходит, если бывают кассовые разрывы: деньги от клиентов приходят позже, чем нужно платить поставщикам.",
		MinAmount:   100_000 * rub, MaxAmount: 5_000_000 * rub, MinTerm: 12, MaxTerm: 12,
		BaseRateBP: 1800, OperatingCostBP: 335, AutoApproveLimit: 1_500_000 * rub, LGD: 0.65,
		Documents: []DocRequirement{docStatementsProof, docCashPlan, docCollateral},
	},
	{
		ID: Express, Title: "Экспресс-кредит",
		Description: "Небольшая сумма и быстрое решение: без залога и с минимумом документов.",
		Hint:        "Подходит, если деньги нужны срочно и сумма небольшая.",
		MinAmount:   50_000 * rub, MaxAmount: 1_000_000 * rub, MinTerm: 3, MaxTerm: 12,
		BaseRateBP: 2100, OperatingCostBP: 625, AutoApproveLimit: 1_000_000 * rub, LGD: 0.75,
		Documents:       []DocRequirement{docStatements, docPassport, docCollateral},
		PurposeFromForm: true,
	},
}

// Catalog returns a copy of all products.
func Catalog() []Product {
	out := make([]Product, len(catalog))
	copy(out, catalog)
	return out
}

func Get(id ID) (Product, bool) {
	for _, p := range catalog {
		if p.ID == id {
			return p, true
		}
	}
	return Product{}, false
}

// ValidateRequest checks amount (kopecks) and term against the product limits.
func ValidateRequest(p Product, amount int64, term int) []FieldError {
	var errs []FieldError
	if amount < p.MinAmount || amount > p.MaxAmount {
		errs = append(errs, FieldError{"amount", fmt.Sprintf("Сумма для этого продукта — от %d до %d ₽", p.MinAmount/rub, p.MaxAmount/rub)})
	}
	if term < p.MinTerm || term > p.MaxTerm {
		if p.MinTerm == p.MaxTerm {
			errs = append(errs, FieldError{"termMonths", fmt.Sprintf("Срок для этого продукта — %d мес.", p.MinTerm)})
		} else {
			errs = append(errs, FieldError{"termMonths", fmt.Sprintf("Срок для этого продукта — от %d до %d мес.", p.MinTerm, p.MaxTerm)})
		}
	}
	return errs
}

package seed

// DemoApp is one ready-made application of a demo person. The set is built so that a reviewer who
// signs in sees every kind of outcome at once instead of filing applications by hand.
type DemoApp struct {
	Login   string
	Product string // product id
	Amount  int64  // kopecks
	Term    int
	Purpose string
	// Submit false leaves a draft; NoDocs leaves the required documents out of it.
	Submit bool
	NoDocs bool
	// What a bank employee does once the application waits for a decision: counter | approve | request_info | reject.
	Staff        string
	StaffComment string
	Counter      *DemoCounter
	// What the client does with an offer: accept | decline. Cancel withdraws the application.
	Client string
	Cancel bool
	// Want is the status the application must end in (checked by tests).
	Want string
}

type DemoCounter struct {
	Amount int64
	Term   int
	RateBP int
}

// DemoApplications lists the curated applications, oldest first within each person.
func DemoApplications() []DemoApp {
	return []DemoApp{
		// client1, a reliable bakery: one loan already disbursed, a new automatic offer waiting
		{Login: "client1", Product: "express", Amount: 300_000 * rub, Term: 6, Purpose: "Ремонт печи", Submit: true, Client: "accept", Want: "DISBURSED"},
		{Login: "client1", Product: "working_capital", Amount: 1_000_000 * rub, Term: 12, Purpose: "Закупка муки и сырья перед сезоном", Submit: true, Want: "OFFER_MADE"},
		// client2, an indebted entrepreneur: one application waits for a decision, one got a counter-offer
		{Login: "client2", Product: "overdraft", Amount: 500_000 * rub, Term: 12, Purpose: "Покрытие кассовых разрывов", Submit: true, Staff: "counter",
			StaffComment: "Предлагаем меньшую сумму: платёж будет посильным при текущем обороте", Counter: &DemoCounter{Amount: 400_000 * rub, Term: 12, RateBP: 2450}, Want: "OFFER_MADE"},
		{Login: "client2", Product: "working_capital", Amount: 3_000_000 * rub, Term: 24, Purpose: "Пополнение оборотных средств к сезону", Submit: true, Want: "ANALYST_REVIEW"},
		// client3, a young company: the analyst asked a question
		{Login: "client3", Product: "express", Amount: 500_000 * rub, Term: 6, Purpose: "Закупка оборудования для офиса", Submit: true, Staff: "request_info",
			StaffComment: "Пришлите договор аренды помещения", Want: "NEEDS_INFO"},
		// client4, a liquidated company: an automatic rejection
		{Login: "client4", Product: "working_capital", Amount: 500_000 * rub, Term: 12, Purpose: "Закупка товара", Submit: true, Want: "REJECTED"},
		// client5, a borderline case: the same borrower asks for 1.5 and 20 million
		{Login: "client5", Product: "equipment", Amount: 1_500_000 * rub, Term: 36, Purpose: "Покупка оборудования для расширения салона", Submit: true, Want: "ANALYST_REVIEW"},
		{Login: "client5", Product: "equipment", Amount: 20_000_000 * rub, Term: 40, Purpose: "Открытие второго салона", Submit: true, Want: "ANALYST_REVIEW"},
		// client6, an overdue above 90 days: an automatic rejection by a hard rule
		{Login: "client6", Product: "working_capital", Amount: 1_000_000 * rub, Term: 12, Purpose: "Закупка материалов", Submit: true, Want: "REJECTED"},
		// client7, a strong borrower asking for more than the automatic limit
		{Login: "client7", Product: "working_capital", Amount: 6_000_000 * rub, Term: 18, Purpose: "Закупка сырья по долгосрочному контракту", Submit: true, Want: "ANALYST_REVIEW"},
		// client8, a good borrower with little repayment history; a second request refused by the analyst
		{Login: "client8", Product: "working_capital", Amount: 3_000_000 * rub, Term: 24, Purpose: "Расширение производства", Submit: true, Staff: "reject",
			StaffComment: "Обороты не подтверждают запрошенную сумму. Повторите заявку на меньшую сумму", Want: "REJECTED"},
		{Login: "client8", Product: "working_capital", Amount: 800_000 * rub, Term: 12, Purpose: "Закупка древесины", Submit: true, Want: "ANALYST_REVIEW"},
		// client9, payments on existing loans leave no room: an automatic rejection by affordability
		{Login: "client9", Product: "working_capital", Amount: 1_000_000 * rub, Term: 12, Purpose: "Оплата топлива и ремонта автопарка", Submit: true, Want: "REJECTED"},
		// client10, bankruptcy: an automatic rejection by a hard rule
		{Login: "client10", Product: "working_capital", Amount: 1_000_000 * rub, Term: 12, Purpose: "Закупка материалов", Submit: true, Want: "REJECTED"},
		// client11, a regular client with every kind of history
		{Login: "client11", Product: "express", Amount: 400_000 * rub, Term: 6, Purpose: "Обновление оборудования", Submit: true, Client: "accept", Want: "DISBURSED"},
		{Login: "client11", Product: "working_capital", Amount: 1_500_000 * rub, Term: 12, Purpose: "Закупка материалов", Submit: true, Client: "decline", Want: "DECLINED_BY_CLIENT"},
		{Login: "client11", Product: "express", Amount: 100_000 * rub, Term: 3, Purpose: "Временные расходы", Submit: true, Cancel: true, Want: "CANCELLED"},
		{Login: "client11", Product: "equipment", Amount: 2_000_000 * rub, Term: 24, Purpose: "Покупка оборудования", NoDocs: true, Want: "DRAFT"},
	}
}

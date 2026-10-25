package integrations

import (
	"errors"
	"time"
)

// ErrUnavailable is returned when an external source cannot be reached after retries.
var ErrUnavailable = errors.New("external source unavailable")

// ErrNotFound is returned when the external source does not know the INN.
var ErrNotFound = errors.New("not found in external source")

// BKIReport is a (mocked) credit bureau report for a company.
type BKIReport struct {
	HasHistory     bool  `json:"hasHistory"`
	ActiveLoans    int   `json:"activeLoans"`
	TotalDebt      int64 `json:"totalDebt"`      // kopecks
	ClosedLoans    int   `json:"closedLoans"`    // loans already repaid in full
	TotalRepaid    int64 `json:"totalRepaid"`    // kopecks, the sum of those loans
	MaxOverdueDays int   `json:"maxOverdueDays"` // the longest overdue in the last 36 months
	Score          int   `json:"score"`          // 300..850
	// Restructured: a current loan has been restructured, a mark of financial difficulty.
	Restructured bool `json:"restructured,omitempty"`
}

// EGRULRecord is a (mocked) state registry record.
type EGRULRecord struct {
	INN          string    `json:"inn"`
	Name         string    `json:"name"`
	Status       string    `json:"status"` // active | liquidated | liquidating
	RegisteredAt time.Time `json:"registeredAt"`
	OKVED        string    `json:"okved"`
	Director     string    `json:"director"`
	Address      string    `json:"address"`
	Bankrupt     bool      `json:"bankrupt"`
	// Litigation is the weight of court cases against the company and its director (arbitration cases,
	// disqualification): 0 none, 1 minor, 2 major. Simulated, as the whole record.
	Litigation int `json:"litigation,omitempty"`
	// Finance is the published accounts, Statement the analysis of the bank statement. Nil: not received.
	Finance   *Finance   `json:"finance,omitempty"`
	Statement *Statement `json:"statement,omitempty"`
}

// Finance holds what the model takes from the published accounts. A sole trader often publishes none.
// Simulated, as the whole record: a real model would read the reports of the company.
type Finance struct {
	GrossMargin  float64 `json:"grossMargin"`  // gross profit / revenue
	CurrentRatio float64 `json:"currentRatio"` // current assets / short-term liabilities
}

// Statement is what the bank sees in the statement of the settlement account. The cash flow is the receipts minus
// the operating write-offs of the month; debt service is not taken out of it (it is the denominator of DSCR).
type Statement struct {
	NOIMargin float64 `json:"noiMargin"` // (receipts - operating write-offs) / receipts
	// Anomaly is the finding of the analysis: 0 none, 1 minor, 2 major
	// (circular transfers, sudden jumps in turnover, doubtful counterparties).
	Anomaly int `json:"anomaly"`
}

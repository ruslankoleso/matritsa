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
	// Finance comes from the published accounts and the analysis of the bank statement. Nil: not received.
	Finance *Finance `json:"finance,omitempty"`
}

// Finance holds what the model needs from the accounts and the statement. The bank has revenue by month
// from the statement; the rest is simulated, as a real model would take it from the reports of the company.
type Finance struct {
	GrossMargin  float64 `json:"grossMargin"`  // gross profit / revenue
	NOIMargin    float64 `json:"noiMargin"`    // net operating income (before debt service) / revenue
	CurrentRatio float64 `json:"currentRatio"` // current assets / short-term liabilities
	// StatementAnomaly is the finding of the statement analysis: 0 none, 1 minor, 2 major
	// (circular transfers, sudden jumps in turnover, doubtful counterparties).
	StatementAnomaly int `json:"statementAnomaly"`
}

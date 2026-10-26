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
	// DirectorDisqualified: the head of the company is in the register of disqualified persons.
	DirectorDisqualified bool `json:"directorDisqualified,omitempty"`
	// UnreliableData: the registry has marked the data about the company as unreliable.
	UnreliableData bool `json:"unreliableData,omitempty"`
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
// The other figures are the indicators of the analyst's table of anomalies (scoring.StatementFindings); the jump
// of the monthly turnover is read from the revenue by month and needs no field.
type Statement struct {
	NOIMargin float64 `json:"noiMargin"` // (receipts - operating write-offs) / receipts
	// CircularShare is the share of the turnover that looks like transit or circular transfers, CircularMonths the
	// number of months of the last twelve in which they occurred.
	CircularShare  float64 `json:"circularShare"`
	CircularMonths int     `json:"circularMonths"`
	CashShare      float64 `json:"cashShare"` // cash withdrawals / turnover
	TaxBurden      float64 `json:"taxBurden"` // taxes paid / turnover
	// DubiousPayments is the number of payments in the last twelve months to counterparties of doubtful standing,
	// DubiousShare their share of the turnover.
	DubiousPayments int     `json:"dubiousPayments"`
	DubiousShare    float64 `json:"dubiousShare"`
}

// NormalStatement is a statement without anomalies: a little cash, taxes at the usual level.
func NormalStatement(margin float64) *Statement {
	return &Statement{NOIMargin: margin, CashShare: 0.05, TaxBurden: 0.03}
}

// StatementOfLevel builds a statement with the finding of the given level: 0 none, 1 minor (a high share of cash),
// 2 major (regular circular transfers). Used by the simulated sources.
func StatementOfLevel(margin float64, level int) *Statement {
	st := NormalStatement(margin)
	switch level {
	case 1:
		st.CashShare = 0.20
	case 2:
		st.CircularShare, st.CircularMonths = 0.12, 4
	}
	return st
}

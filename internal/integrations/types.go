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
	TotalDebt      int64 `json:"totalDebt"`   // kopecks
	ClosedLoans    int   `json:"closedLoans"` // loans already repaid in full
	TotalRepaid    int64 `json:"totalRepaid"` // kopecks, the sum of those loans
	MaxOverdueDays int   `json:"maxOverdueDays"`
	Score          int   `json:"score"` // 300..850
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
}

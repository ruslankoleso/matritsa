// Package appdata is the typed shape of applications.data (jsonb).
package appdata

import (
	"encoding/json"
	"time"

	"matritsa/internal/decision"
	"matritsa/internal/integrations"
)

type InfoRequest struct {
	Text        string    `json:"text"`
	Items       []string  `json:"items,omitempty"`
	RequestedBy string    `json:"requestedBy"` // system | staff
	At          time.Time `json:"at"`
}

type InfoAnswer struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

type Enrichment struct {
	EGRUL     *integrations.EGRULRecord `json:"egrul,omitempty"`
	BKI       *integrations.BKIReport   `json:"bki,omitempty"`
	Attempts  int                       `json:"attempts"`
	LastError string                    `json:"lastError,omitempty"`
	NextAt    time.Time                 `json:"nextAt,omitempty"`
}

type Review struct {
	Recommended *decision.Offer `json:"recommended,omitempty"`
	Reasons     []string        `json:"reasons,omitempty"`
	ManualCheck bool            `json:"manualCheck,omitempty"` // external sources were unavailable
}

type Rejection struct {
	Reasons []string `json:"reasons"`
	By      string   `json:"by"` // system | staff
}

type Data struct {
	Fields      map[string]string `json:"fields,omitempty"` // wizard answers
	InfoRequest *InfoRequest      `json:"infoRequest,omitempty"`
	InfoAnswers []InfoAnswer      `json:"infoAnswers,omitempty"`
	Enrichment  *Enrichment       `json:"enrichment,omitempty"`
	Review      *Review           `json:"review,omitempty"`
	Rejection   *Rejection        `json:"rejection,omitempty"`
}

// Parse decodes jsonb; malformed or empty input yields an empty Data.
func Parse(raw []byte) Data {
	var d Data
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &d)
	}
	return d
}

func (d Data) Marshal() []byte {
	b, _ := json.Marshal(d)
	return b
}

package httpapi

import (
	"net/http"

	"matritsa/internal/httpapi/gen"
)

// GetProcessMetrics reports the process quality measures of docs/product.md from the audit trail.
func (s *Server) GetProcessMetrics(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.DecisionMetrics(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	f, err := s.Store.FollowUpMetrics(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	share := func(part, whole int32) float64 {
		if whole == 0 {
			return 0
		}
		return float64(part) / float64(whole)
	}
	writeJSON(w, http.StatusOK, gen.ProcessMetrics{
		Submitted: int(d.Submitted), Decided: int(d.Decided), DecidedAuto: int(d.DecidedAuto),
		AutoShare:         share(d.DecidedAuto, d.Decided),
		MedianAutoSeconds: d.MedianAutoSeconds, MedianAnalystSeconds: d.MedianAnalystSeconds,
		AskedInfo: int(f.AskedInfo), InfoShare: share(f.AskedInfo, f.Submitted),
		Offered: int(d.Offered), Accepted: int(f.Accepted), Declined: int(f.Declined), Expired: int(f.Expired),
		AcceptanceShare: share(f.Accepted, f.Accepted+f.Declined+f.Expired),
		WaitingAnalyst:  int(f.WaitingAnalyst),
	})
}

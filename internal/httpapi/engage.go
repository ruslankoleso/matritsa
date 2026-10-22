package httpapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"matritsa/internal/auth"
	"matritsa/internal/feed"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/workflow"
)

func (s *Server) GetFeed(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	user, err := s.Store.GetUser(r.Context(), claims.UserID)
	if err != nil || user.CompanyID == nil {
		writeJSON(w, http.StatusOK, []gen.FeedCard{})
		return
	}
	co, err := s.Store.GetCompany(r.Context(), *user.CompanyID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	fin, err := s.Store.ListFinancials(r.Context(), co.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	p := feed.Profile{
		CompanyName: co.Name, OKVED: co.Okved, MonthlyObligations: co.MonthlyObligations,
		CompanyAgeMonths: int(time.Since(co.RegisteredAt.Time).Hours() / 24 / 30),
	}
	if n := len(fin); n > 0 {
		var sum float64
		for _, f := range fin {
			sum += float64(f.Revenue)
		}
		mean := sum / float64(n)
		var sq float64
		for _, f := range fin {
			d := float64(f.Revenue) - mean
			sq += d * d
		}
		p.AvgMonthlyRevenue = int64(mean)
		if mean > 0 {
			p.Volatility = math.Sqrt(sq/float64(n)) / mean
		}
	}
	apps, err := s.Store.ListApplicationsByUser(r.Context(), user.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	for _, a := range apps {
		sum := feed.AppSummary{ID: a.ID, Status: a.Status, ProductTitle: productTitle(a.ProductID)}
		if workflow.Status(a.Status) == workflow.OfferMade {
			if o, err := s.Store.GetActiveOffer(r.Context(), a.ID); err == nil {
				sum.OfferExpiresAt = o.ExpiresAt.Time
			}
		}
		p.Applications = append(p.Applications, sum)
	}
	cards := feed.Build(p)
	out := make([]gen.FeedCard, len(cards))
	for i, c := range cards {
		fc := gen.FeedCard{Id: c.ID, Kind: gen.FeedCardKind(c.Kind), Title: c.Title, Body: c.Body, Priority: c.Priority}
		if c.ProductID != "" {
			v := c.ProductID
			fc.ProductId = &v
		}
		if c.ApplicationID != "" {
			v := c.ApplicationID
			fc.ApplicationId = &v
		}
		if c.CTALabel != "" {
			l, h := c.CTALabel, c.CTAHref
			fc.CtaLabel, fc.CtaHref = &l, &h
		}
		out[i] = fc
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) ListNotifications(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	ns, err := s.Store.ListNotifications(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	out := make([]gen.Notification, len(ns))
	for i, n := range ns {
		out[i] = gen.Notification{Id: uid(n.ID), Text: n.Text, Read: n.ReadAt.Valid, CreatedAt: n.CreatedAt.Time}
		if n.ApplicationID != nil {
			id := uid(*n.ApplicationID)
			out[i].ApplicationId = &id
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) MarkNotificationRead(w http.ResponseWriter, r *http.Request, id openapiUUID) {
	claims, _ := auth.FromContext(r.Context())
	if err := s.Store.MarkNotificationRead(r.Context(), markReadParams(id.String(), claims.UserID)); err != nil {
		internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StreamEvents is a Server-Sent Events stream of status changes and notifications.
func (s *Server) StreamEvents(w http.ResponseWriter, r *http.Request, _ gen.StreamEventsParams) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "no_streaming", "Потоковая передача недоступна")
		return
	}
	claims, _ := auth.FromContext(r.Context())
	userCh, cancelUser := s.Hub.Subscribe(claims.UserID)
	defer cancelUser()
	var staffCh <-chan eventsEvent
	if claims.Role == "staff" {
		ch, cancel := s.Hub.Subscribe("staff")
		defer cancel()
		staffCh = ch
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	tick := time.NewTicker(s.Heartbeat)
	defer tick.Stop()
	send := func(e eventsEvent) bool {
		b, _ := json.Marshal(e)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, b); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-userCh:
			if !open || !send(e) {
				return
			}
		case e, open := <-staffCh:
			if !open || !send(e) {
				return
			}
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

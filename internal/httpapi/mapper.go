package httpapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"matritsa/internal/appdata"
	"matritsa/internal/decision"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

func uid(s string) openapi_types.UUID { return openapi_types.UUID(uuid.MustParse(s)) }

func toCompany(c db.Company) gen.Company {
	return gen.Company{
		Id: uid(c.ID), Inn: c.Inn, Name: c.Name, Kind: gen.CompanyKind(c.Kind),
		RegisteredAt: openapi_types.Date{Time: c.RegisteredAt.Time}, Okved: c.Okved, Director: c.Director, Address: c.Address,
	}
}

func toProduct(p products.Product) gen.Product {
	docs := make([]gen.DocRequirement, len(p.Documents))
	for i, d := range p.Documents {
		docs[i] = gen.DocRequirement{Kind: d.Kind, Title: d.Title, Why: d.Why, Required: d.Required}
	}
	return gen.Product{
		Id: gen.ProductId(p.ID), Title: p.Title, Description: p.Description, Hint: p.Hint, MinAmount: p.MinAmount, MaxAmount: p.MaxAmount,
		MinTerm: p.MinTerm, MaxTerm: p.MaxTerm, BaseRateBp: p.BaseRateBP, AutoApproveLimit: p.AutoApproveLimit, Documents: docs,
	}
}

func productTitle(id string) string {
	if p, ok := products.Get(products.ID(id)); ok {
		return p.Title
	}
	return id
}

func rubles(kopecks int64) string {
	s := fmt.Sprintf("%d", kopecks/100)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	return string(out) + " ₽"
}

func toSummary(a db.Application) gen.ApplicationSummary {
	return gen.ApplicationSummary{
		Id: uid(a.ID), ProductId: a.ProductID, ProductTitle: productTitle(a.ProductID), Amount: a.Amount, TermMonths: int(a.TermMonths),
		Status: gen.Status(a.Status), StatusMessage: workflow.ClientMessage(workflow.Status(a.Status)),
		CreatedAt: a.CreatedAt.Time, UpdatedAt: a.UpdatedAt.Time,
	}
}

// clientActions says what the client may do in the status.
func clientActions(s workflow.Status) []gen.ApplicationAllowedActions {
	switch s {
	case workflow.Draft:
		return []gen.ApplicationAllowedActions{gen.Edit, gen.Upload, gen.Submit, gen.Cancel}
	case workflow.NeedsInfo:
		return []gen.ApplicationAllowedActions{gen.Upload, gen.Respond, gen.Cancel}
	case workflow.AnalystReview:
		return []gen.ApplicationAllowedActions{gen.Cancel}
	case workflow.OfferMade:
		return []gen.ApplicationAllowedActions{gen.Accept, gen.Decline, gen.Cancel}
	}
	return []gen.ApplicationAllowedActions{}
}

func toOffer(o db.Offer, requested db.Application) gen.Offer {
	out := gen.Offer{
		Amount: o.Amount, TermMonths: int(o.TermMonths), RateBp: int(o.RateBp), MonthlyPayment: o.MonthlyPayment,
		Source: gen.OfferSource(o.Source), ExpiresAt: o.ExpiresAt.Time,
	}
	changes := offerChanges(o.Amount, int(o.TermMonths), requested)
	if len(changes) > 0 {
		out.Changes = &changes
	}
	return out
}

func offerChanges(amount int64, term int, requested db.Application) []string {
	var changes []string
	if amount != requested.Amount {
		changes = append(changes, fmt.Sprintf("Сумма: %s вместо %s", rubles(amount), rubles(requested.Amount)))
	}
	if term != int(requested.TermMonths) {
		changes = append(changes, fmt.Sprintf("Срок: %d мес. вместо %d мес.", term, requested.TermMonths))
	}
	return changes
}

func fromDecisionOffer(o decision.Offer, requested db.Application, expires time.Time) gen.Offer {
	out := gen.Offer{Amount: o.Amount, TermMonths: o.TermMonths, RateBp: o.RateBP, MonthlyPayment: o.MonthlyPayment, Source: gen.OfferSource(o.Source), ExpiresAt: expires}
	if ch := offerChanges(o.Amount, o.TermMonths, requested); len(ch) > 0 {
		out.Changes = &ch
	}
	return out
}

// loadApplication builds the client-facing view of an application.
func (s *Server) loadApplication(ctx context.Context, a db.Application) (gen.Application, error) {
	co, err := s.Store.GetCompany(ctx, a.CompanyID)
	if err != nil {
		return gen.Application{}, err
	}
	docs, err := s.Store.ListDocuments(ctx, a.ID)
	if err != nil {
		return gen.Application{}, err
	}
	data := appdata.Parse(a.Data)
	status := workflow.Status(a.Status)
	sum := toSummary(a)
	out := gen.Application{
		Id: sum.Id, ProductId: sum.ProductId, ProductTitle: sum.ProductTitle, Amount: sum.Amount, TermMonths: sum.TermMonths,
		Status: sum.Status, StatusMessage: sum.StatusMessage, CreatedAt: sum.CreatedAt, UpdatedAt: sum.UpdatedAt,
		Version: int(a.Version), Purpose: a.Purpose, Fields: data.Fields, AllowedActions: clientActions(status),
		MissingItems: []string{}, Company: toCompany(co), Documents: make([]gen.Document, len(docs)),
	}
	if out.Fields == nil {
		out.Fields = map[string]string{}
	}
	for i, d := range docs {
		out.Documents[i] = gen.Document{Id: uid(d.ID), Kind: d.Kind, Filename: d.Filename, Size: d.Size, UploadedAt: d.UploadedAt.Time}
	}
	if status == workflow.Draft || status == workflow.NeedsInfo {
		missing, err := s.Pipeline.Missing(ctx, a)
		if err != nil {
			return gen.Application{}, err
		}
		out.MissingItems = append(out.MissingItems, missing...)
	}
	if status == workflow.NeedsInfo && data.InfoRequest != nil {
		ir := data.InfoRequest
		req := gen.InfoRequest{Text: ir.Text, RequestedBy: gen.InfoRequestRequestedBy(ir.RequestedBy), At: ir.At}
		if len(ir.Items) > 0 {
			req.Items = &ir.Items
		}
		out.InfoRequest = &req
	}
	switch status {
	case workflow.OfferMade, workflow.Signing, workflow.Disbursed, workflow.DeclinedByClient, workflow.Expired:
		if o, err := s.Store.GetActiveOffer(ctx, a.ID); err == nil {
			offer := toOffer(o, a)
			out.Offer = &offer
		}
	case workflow.Rejected:
		rej := gen.Rejection{Reasons: []string{}, By: gen.RejectionBySystem}
		if data.Rejection != nil {
			rej.Reasons = data.Rejection.Reasons
			if data.Rejection.By == "staff" {
				rej.By = gen.RejectionByStaff
			}
		}
		if c := s.lastStaffComment(ctx, a.ID, "AnalystReject"); c != "" {
			rej.Comment = &c
		}
		rej.Advice, rej.CanReapply = decision.NextSteps(rej.Reasons)
		out.Rejection = &rej
	}
	return out, nil
}

func (s *Server) lastStaffComment(ctx context.Context, appID, event string) string {
	evs, err := s.Store.ListEvents(ctx, appID)
	if err != nil {
		return ""
	}
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Event == event {
			return evs[i].Comment
		}
	}
	return ""
}

func actorKind(actor string) string {
	switch {
	case strings.HasPrefix(actor, "client"):
		return "client"
	case strings.HasPrefix(actor, "staff"):
		return "staff"
	}
	return "system"
}

// clientComments are the events whose comment is written for the client: what the intake check
// found missing and the client's own answers. Comments of staff are addressed to the client too.
// Other comments are the system's working notes ("Класс риска D, 88 баллов из 100") and stay with the bank.
var clientComments = map[workflow.Event]bool{
	workflow.IntakeNeedsInfo: true, workflow.ClientProvidedInfo: true, workflow.ClientAnswered: true,
}

// toTimeline maps the audit trail; forClient hides the bank's working notes.
func toTimeline(evs []db.ApplicationEvent, forClient bool) []gen.TimelineEntry {
	out := make([]gen.TimelineEntry, 0, len(evs))
	for _, e := range evs {
		msg := workflow.ClientMessage(workflow.Status(e.ToStatus))
		if e.FromStatus == e.ToStatus {
			msg = e.Comment
		}
		actor := actorKind(e.Actor)
		entry := gen.TimelineEntry{At: e.At.Time, Actor: actor, Event: e.Event, FromStatus: e.FromStatus, ToStatus: e.ToStatus, Message: msg}
		if forClient && actor != "staff" && !clientComments[workflow.Event(e.Event)] {
			out = append(out, entry)
			continue
		}
		if e.Comment != "" {
			c := e.Comment
			entry.Comment = &c
		}
		out = append(out, entry)
	}
	return out
}

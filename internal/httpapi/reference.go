package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"matritsa/internal/ai"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/integrations"
	"matritsa/internal/products"
)

func (s *Server) ListProducts(w http.ResponseWriter, _ *http.Request) {
	cat := products.Catalog()
	out := make([]gen.Product, len(cat))
	for i, p := range cat {
		out[i] = toProduct(p)
	}
	writeJSON(w, http.StatusOK, out)
}

func validINN(inn string) bool {
	if len(inn) != 10 && len(inn) != 12 {
		return false
	}
	for _, c := range inn {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *Server) LookupCompany(w http.ResponseWriter, r *http.Request, params gen.LookupCompanyParams) {
	inn := strings.TrimSpace(params.Inn)
	if !validINN(inn) {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Проверьте ИНН", []fieldErr{{"inn", "ИНН состоит из 10 цифр (для организаций) или 12 цифр (для ИП)"}})
		return
	}
	rec, err := s.Lookup.EGRUL(r.Context(), inn)
	switch {
	case errors.Is(err, integrations.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Компания с таким ИНН не найдена в реестре")
		return
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, "registry_unavailable", "Реестр временно недоступен. Можно ввести данные вручную и продолжить.")
		return
	}
	kind := gen.CompanyLookupKindOoo
	if len(inn) == 12 {
		kind = gen.CompanyLookupKindIp
	}
	reg := rec.RegisteredAt
	if reg.IsZero() {
		reg = time.Now()
	}
	writeJSON(w, http.StatusOK, gen.CompanyLookup{
		Inn: rec.INN, Name: rec.Name, Kind: kind, Status: rec.Status, RegisteredAt: openapi_types.Date{Time: reg},
		Okved: rec.OKVED, Director: rec.Director, Address: rec.Address,
	})
}

func (s *Server) RecommendProduct(w http.ResponseWriter, r *http.Request) {
	var req gen.RecommendRequest
	if !decodeBody(w, r, &req) {
		return
	}
	rec, err := s.Assistant.Recommend(r.Context(), req.Text)
	if errors.Is(err, ai.ErrEmptyText) {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Расскажите, на что нужны деньги", []fieldErr{{"text", "Опишите потребность хотя бы парой слов"}})
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	amount, term := rec.Amount, rec.TermMonths
	writeJSON(w, http.StatusOK, gen.Recommendation{
		ProductId: string(rec.ProductID), Amount: &amount, TermMonths: &term, Rationale: rec.Rationale, Source: gen.RecommendationSource(rec.Source),
	})
}

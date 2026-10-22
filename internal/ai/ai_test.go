package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"matritsa/internal/decision"
	"matritsa/internal/scoring"
)

type fakeLLM struct {
	reply string
	err   error
	calls int
}

func (f *fakeLLM) Complete(_ context.Context, _, _ string) (string, error) {
	f.calls++
	return f.reply, f.err
}

func TestRecommend_OllamaDown_FallsBackToRules(t *testing.T) {
	a := NewAssistant(&fakeLLM{err: errors.New("connection refused")})
	rec, err := a.Recommend(context.Background(), "Хочу купить станок для мастерской")
	if err != nil || rec.Source != "rules" || rec.ProductID != "equipment" {
		t.Fatalf("want rules fallback with equipment, got %+v %v", rec, err)
	}
}

func TestRecommend_GarbageJSON_FallsBack(t *testing.T) {
	a := NewAssistant(&fakeLLM{reply: "извините, я не понял {{{"})
	rec, _ := a.Recommend(context.Background(), "нужны деньги на зарплаты")
	if rec.Source != "rules" || rec.ProductID != "working_capital" {
		t.Fatalf("garbage must fall back: %+v", rec)
	}
}

func TestRecommend_ModelReturnsUnknownProduct_FallsBack(t *testing.T) {
	a := NewAssistant(&fakeLLM{reply: `{"productId":"mortgage","amountRub":1000000,"termMonths":12,"rationale":"x"}`})
	rec, _ := a.Recommend(context.Background(), "кассовый разрыв в конце месяца")
	if rec.Source != "rules" || rec.ProductID != "overdraft" {
		t.Fatalf("unknown product must fall back: %+v", rec)
	}
}

func TestRecommend_ValidModelAnswer_UsedWithSourceAI(t *testing.T) {
	a := NewAssistant(&fakeLLM{reply: `{"productId":"express","amountRub":300000,"termMonths":6,"rationale":"Небольшая сумма и срочно"}`})
	rec, err := a.Recommend(context.Background(), "срочно нужно триста тысяч")
	if err != nil || rec.Source != "ai" || rec.ProductID != "express" || rec.Amount != 300_000*rub || rec.TermMonths != 6 {
		t.Fatalf("valid model answer must be used: %+v %v", rec, err)
	}
	if rec.Rationale == "" {
		t.Fatal("rationale required")
	}
}

func TestRecommend_AmountClampedToProductRange(t *testing.T) {
	a := NewAssistant(&fakeLLM{reply: `{"productId":"express","amountRub":900000000,"termMonths":99,"rationale":"много"}`})
	rec, _ := a.Recommend(context.Background(), "очень много денег")
	if rec.ProductID != "express" || rec.Amount != 1_000_000*rub || rec.TermMonths != 12 {
		t.Fatalf("amount and term must be clamped to the product range: %+v", rec)
	}
}

func TestRecommend_ModelNeverDecides_OnlySuggests(t *testing.T) {
	a := NewAssistant(&fakeLLM{reply: `{"productId":"express","amountRub":0,"termMonths":0,"rationale":"ok"}`})
	rec, _ := a.Recommend(context.Background(), "что-нибудь")
	if rec.Amount < 50_000*rub || rec.TermMonths < 3 {
		t.Fatalf("zero values must be raised to product minimums: %+v", rec)
	}
}

func TestRecommend_NilLLM_UsesRules(t *testing.T) {
	rec, err := NewAssistant(nil).Recommend(context.Background(), "купить грузовик")
	if err != nil || rec.Source != "rules" || rec.ProductID != "equipment" {
		t.Fatalf("%+v %v", rec, err)
	}
}

func TestRecommend_EmptyText_Error(t *testing.T) {
	if _, err := NewAssistant(nil).Recommend(context.Background(), "   "); err == nil {
		t.Fatal("empty text must be rejected")
	}
}

func TestFallback_KeywordsAndAmounts(t *testing.T) {
	cases := []struct {
		text   string
		pid    string
		amount int64
	}{
		{"Нужен станок за 2 млн", "equipment", 2_000_000 * rub},
		{"закупить товар на 500 тысяч", "working_capital", 500_000 * rub},
		{"кассовый разрыв", "overdraft", 0},
		{"срочно немного денег", "express", 0},
	}
	for _, c := range cases {
		rec := Fallback(c.text)
		if string(rec.ProductID) != c.pid {
			t.Errorf("%q → %s, want %s", c.text, rec.ProductID, c.pid)
		}
		if c.amount != 0 && rec.Amount != c.amount {
			t.Errorf("%q → amount %d, want %d", c.text, rec.Amount, c.amount)
		}
		if rec.Amount < 50_000*rub || rec.TermMonths < 3 || rec.Rationale == "" {
			t.Errorf("%q → incomplete recommendation %+v", c.text, rec)
		}
	}
}

func sampleResult() (scoring.Result, decision.Decision) {
	res := scoring.Result{Score: 59, Grade: scoring.GradeC, PD: 0.06, CumulativePD: 0.1126, LGD: 0.6, EAD: 300_000_000, ExpectedLoss: 20_268_000, ELRate: 0.06756, Factors: []scoring.Factor{
		{Key: "company_age", Title: "Возраст бизнеса", Points: 10, Max: 10, Note: "Бизнес работает 8 лет — это плюс"},
		{Key: "existing_debt", Title: "Текущая долговая нагрузка", Points: 0, Max: 20, Note: "Действующие кредиты забирают около 52% выручки — это слишком много"},
		{Key: "bki_score", Title: "Кредитная история", Points: 7, Max: 20, Note: "Средний балл бюро кредитных историй"},
	}}
	rec := decision.Offer{Amount: 800_000 * rub, TermMonths: 24, RateBP: 2100}
	return res, decision.Decision{Outcome: decision.Review, Recommended: &rec, Reasons: []string{"Оценка риска в серой зоне"}}
}

func TestExplain_Fallback_MentionsTopFactors(t *testing.T) {
	res, d := sampleResult()
	text, src := NewExplainer(nil).Explain(context.Background(), res, d, "staff")
	if src != "rules" {
		t.Fatalf("source = %s", src)
	}
	for _, want := range []string{"C", "59", "Бизнес работает 8 лет", "слишком много"} {
		if !strings.Contains(text, want) {
			t.Errorf("explanation misses %q:\n%s", want, text)
		}
	}
}

func TestExplain_ClientAudience_NoInternalJargon(t *testing.T) {
	res, d := sampleResult()
	text, _ := NewExplainer(nil).Explain(context.Background(), res, d, "client")
	if strings.Contains(text, "скоринг") || strings.Contains(text, "балл") {
		t.Errorf("client text must avoid jargon:\n%s", text)
	}
}

func TestExplain_UsesLLMWhenAvailable_ElseFallback(t *testing.T) {
	res, d := sampleResult()
	text, src := NewExplainer(&fakeLLM{reply: "Короткое пояснение от модели."}).Explain(context.Background(), res, d, "staff")
	if src != "ai" || text != "Короткое пояснение от модели." {
		t.Fatalf("got %q %q", text, src)
	}
	text, src = NewExplainer(&fakeLLM{err: errors.New("down")}).Explain(context.Background(), res, d, "staff")
	if src != "rules" || text == "" {
		t.Fatalf("fallback expected, got %q %q", text, src)
	}
	text, src = NewExplainer(&fakeLLM{reply: "  "}).Explain(context.Background(), res, d, "staff")
	if src != "rules" || text == "" {
		t.Fatalf("empty model answer must fall back, got %q %q", text, src)
	}
}

func TestOllama_ParsesChatResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path != "/api/chat" || req["model"] != "qwen2.5:3b" || req["stream"] != false {
			t.Errorf("bad request %s %+v", r.URL.Path, req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"role": "assistant", "content": `{"ok":true}`}})
	}))
	defer srv.Close()
	got, err := NewOllama(srv.URL, "qwen2.5:3b").Complete(context.Background(), "sys", "user")
	if err != nil || got != `{"ok":true}` {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestOllama_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	o := NewOllama(srv.URL, "m")
	o.Timeout = 100 * time.Millisecond
	start := time.Now()
	if _, err := o.Complete(context.Background(), "s", "u"); err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > time.Second {
		t.Fatal("timeout not respected")
	}
}

func TestOllama_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "model not found", 404) }))
	defer srv.Close()
	if _, err := NewOllama(srv.URL, "m").Complete(context.Background(), "s", "u"); err == nil {
		t.Fatal("non-200 must be an error")
	}
}

func TestExplain_Staff_StatesExpectedLossWithItsInputs(t *testing.T) {
	res, d := sampleResult()
	text, _ := NewExplainer(nil).Explain(context.Background(), res, d, "staff")
	for _, want := range []string{"Ожидаемые потери", "202 680", "PD", "LGD 60%", "3 000 000"} {
		if !strings.Contains(text, want) {
			t.Errorf("explanation misses %q:\n%s", want, text)
		}
	}
}

func TestExplain_Staff_NoExpectedLossSentenceWithoutMetrics(t *testing.T) {
	res, d := sampleResult()
	res.EAD, res.ExpectedLoss = 0, 0
	text, _ := NewExplainer(nil).Explain(context.Background(), res, d, "staff")
	if strings.Contains(text, "Ожидаемые потери") {
		t.Fatalf("no EL data → no EL sentence:\n%s", text)
	}
}

func TestExplain_Client_ExistingDebtIsPutInPlainWords(t *testing.T) {
	res, d := sampleResult()
	text, _ := NewExplainer(nil).Explain(context.Background(), res, d, "client")
	if !strings.Contains(text, "уже действующие платежи") {
		t.Fatalf("client text must explain the existing-debt weakness:\n%s", text)
	}
	if strings.Contains(text, "PD") || strings.Contains(text, "LGD") {
		t.Fatalf("client text must not show risk jargon:\n%s", text)
	}
}

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
	"matritsa/internal/products"
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
		{Key: "dscr", Title: "Покрытие долга денежным потоком (DSCR)", Points: 0, Max: 15, Note: "DSCR 0,6: денежного потока не хватает на текущие платежи, чрезмерная нагрузка"},
		{Key: "liquidity", Title: "Ликвидность (текущая)", Points: 0, Max: 5, Note: "Нет данных: бухгалтерская отчётность не получена", NoData: true},
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
	for _, want := range []string{"C", "59", "Бизнес работает 8 лет", "чрезмерная нагрузка", "Нет данных по критериям: Ликвидность (текущая)"} {
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
	if !strings.Contains(text, "денежного потока мало для платежей по действующим кредитам") {
		t.Fatalf("client text must explain the debt weakness:\n%s", text)
	}
	if !strings.Contains(text, "данных пока нет") || strings.Contains(text, "ликвидность низкая") {
		t.Fatalf("a missing figure is said to be missing, not turned into a weakness:\n%s", text)
	}
	if strings.Contains(text, "PD") || strings.Contains(text, "LGD") {
		t.Fatalf("client text must not show risk jargon:\n%s", text)
	}
}

// capture records the requests an Ollama server receives.
func capture(t *testing.T, reply string) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"role": "assistant", "content": reply}})
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestOllama_JSONCallForcesJSONOutput_PlainCallDoesNot(t *testing.T) {
	srv, got := capture(t, `{"ok":true}`)
	o := NewOllama(srv.URL, "m")
	if _, err := o.CompleteJSON(context.Background(), "s", "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Complete(context.Background(), "s", "u"); err != nil {
		t.Fatal(err)
	}
	if (*got)[0]["format"] != "json" {
		t.Fatalf("a call that expects JSON must ask the model for JSON: %+v", (*got)[0])
	}
	if _, has := (*got)[1]["format"]; has {
		t.Fatalf("a plain-text call must not force JSON: %+v", (*got)[1])
	}
}

func TestOllama_TurnsOffThinking_SoTheAnswerIsShortAndClean(t *testing.T) {
	srv, got := capture(t, "ok")
	_, _ = NewOllama(srv.URL, "m").Complete(context.Background(), "s", "u")
	if (*got)[0]["think"] != false {
		t.Fatalf("thinking must be off: %+v", (*got)[0])
	}
}

type jsonOnlyLLM struct {
	fakeLLM
	jsonCalls int
}

func (j *jsonOnlyLLM) CompleteJSON(ctx context.Context, s, u string) (string, error) {
	j.jsonCalls++
	return j.fakeLLM.Complete(ctx, s, u)
}

func TestRecommend_AsksForJSON_WhenTheModelCan(t *testing.T) {
	l := &jsonOnlyLLM{fakeLLM: fakeLLM{reply: `{"productId":"express","amountRub":300000,"termMonths":6,"rationale":"Небольшая сумма и срочно"}`}}
	rec, err := NewAssistant(l).Recommend(context.Background(), "срочно нужны 300 тысяч")
	if err != nil || rec.Source != "ai" || l.jsonCalls != 1 {
		t.Fatalf("%+v %v json calls %d", rec, err, l.jsonCalls)
	}
}

// Small models quote badly in one predictable way: `"rationale\":\"text\"}`. The answer itself is fine,
// so it is repaired instead of being thrown away.
func TestRecommend_RepairsTheQuotingSlipOfSmallModels(t *testing.T) {
	broken := `{"productId":"equipment","amountRub":2000000,"termMonths":18,"rationale\":\"Этот продукт предназначен для покупки оборудования и техники.\"}`
	rec, err := NewAssistant(&fakeLLM{reply: broken}).Recommend(context.Background(), "купить станок за 2 млн")
	if err != nil || rec.Source != "ai" || rec.ProductID != products.Equipment || rec.Amount != 2_000_000*rub || rec.TermMonths != 18 {
		t.Fatalf("%+v %v", rec, err)
	}
	if !strings.Contains(rec.Rationale, "покупки оборудования") || strings.Contains(rec.Rationale, `\`) {
		t.Fatalf("the explanation must come out clean: %q", rec.Rationale)
	}
}

func TestRecommend_RepairDoesNotLetNonsenseThrough(t *testing.T) {
	for _, reply := range []string{
		`{"productId":"mortgage","amountRub":1000000,"termMonths":12,"rationale\":\"x\"}`, // unknown product
		`{"productId":"express","amountRub":300000,"termMonths":6,"rationale\":\"\"}`,     // empty explanation
		`not json at all \":\" }`,
	} {
		rec, _ := NewAssistant(&fakeLLM{reply: reply}).Recommend(context.Background(), "срочно 300 тысяч")
		if rec.Source != "rules" {
			t.Errorf("a bad answer must fall back to the rules: %q gave %+v", reply, rec)
		}
	}
}

func TestRecommend_ValidJSONIsNotTouched(t *testing.T) {
	ok := `{"productId":"express","amountRub":300000,"termMonths":6,"rationale":"Сказано: \"срочно\" и мало"}`
	rec, _ := NewAssistant(&fakeLLM{reply: ok}).Recommend(context.Background(), "срочно 300 тысяч")
	if rec.Source != "ai" || !strings.Contains(rec.Rationale, `"срочно"`) {
		t.Fatalf("a correct answer with its own escaped quotes must stay as it is: %+v", rec)
	}
}

// Some Qwen3-based models leak the end of their reasoning into the answer.
func TestOllama_StripsLeakedThinkingTags(t *testing.T) {
	for in, want := range map[string]string{
		"</think>\nКредитная заявка имеет класс B.":            "Кредитная заявка имеет класс B.",
		"<think>рассуждаю</think>\n\nИтог: отказ.":             "Итог: отказ.",
		">\nКредитное решение — отказ.":                        "Кредитное решение — отказ.",
		"  \n\nОбычный ответ без тегов.":                       "Обычный ответ без тегов.",
		`{"productId":"express"}`:                              `{"productId":"express"}`,
		"Сравнение: 3 > 2, и это не тег.":                      "Сравнение: 3 > 2, и это не тег.",
		"Модель написала </think> посреди и продолжила текст.": "посреди и продолжила текст.",
		"assistant Для овердрафта нужна выписка.":              "Для овердрафта нужна выписка.",
		"assistant\nСтавка от 15 %.":                           "Ставка от 15 %.",
		"Assistant: Ставка от 15 %.":                           "Ставка от 15 %.",
		"Assistants в банке не работают.":                      "Assistants в банке не работают.",
	} {
		srv, _ := capture(t, in)
		got, err := NewOllama(srv.URL, "m").Complete(context.Background(), "s", "u")
		if err != nil || got != want {
			t.Errorf("%q\n got %q\nwant %q (%v)", in, got, want, err)
		}
	}
}

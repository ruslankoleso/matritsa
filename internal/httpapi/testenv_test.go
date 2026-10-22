package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"matritsa/internal/ai"
	"matritsa/internal/auth"
	"matritsa/internal/events"
	"matritsa/internal/httpapi"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/integrations"
	"matritsa/internal/pipeline"
	"matritsa/internal/products"
	"matritsa/internal/seed"
	"matritsa/internal/store"
	"matritsa/internal/store/storetest"
)

const rub = int64(100)

type fakeSources struct{ down atomic.Bool }

func (f *fakeSources) BKI(_ context.Context, inn string) (*integrations.BKIReport, error) {
	if f.down.Load() {
		return nil, integrations.ErrUnavailable
	}
	if c, ok := seed.ByINN(inn); ok && c.BKI != nil {
		return c.BKI, nil
	}
	return &integrations.BKIReport{}, nil
}

func (f *fakeSources) EGRUL(_ context.Context, inn string) (integrations.EGRULRecord, error) {
	if f.down.Load() {
		return integrations.EGRULRecord{}, integrations.ErrUnavailable
	}
	if c, ok := seed.ByINN(inn); ok {
		return c.EGRUL(), nil
	}
	return integrations.EGRULRecord{}, integrations.ErrNotFound
}

type testEnv struct {
	t    *testing.T
	s    *store.Store
	srv  *httptest.Server
	pipe *pipeline.Pipeline
	hub  *events.Hub
	src  *fakeSources
	mu   sync.Mutex
	tok  map[string]string
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	s := storetest.New(t)
	if err := store.Seed(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	src := &fakeSources{}
	hub := events.NewHub()
	cfg := pipeline.DefaultConfig()
	cfg.RetryDelay = 0
	pipe := pipeline.New(s, src, hub, cfg)
	h := httpapi.New(httpapi.Deps{
		Store: s, Pipeline: pipe, Hub: hub, Issuer: auth.NewIssuer("test-secret-test-secret-test-1234"),
		Lookup: src, Assistant: ai.NewAssistant(nil), Explainer: ai.NewExplainer(nil),
		DataDir: t.TempDir(), LoginLimiter: auth.NewLimiter(5, time.Minute), Heartbeat: 50 * time.Millisecond,
	}).Handler()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &testEnv{t: t, s: s, srv: srv, pipe: pipe, hub: hub, src: src, tok: map[string]string{}}
}

// token logs in once per login and caches the access token.
func (e *testEnv) token(login string) string {
	e.t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	if tok, ok := e.tok[login]; ok {
		return tok
	}
	code, raw := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": login, "password": seed.DemoPassword})
	if code != 200 {
		e.t.Fatalf("login %s: %d %s", login, code, raw)
	}
	var tp gen.TokenPair
	mustJSON(e.t, raw, &tp)
	e.tok[login] = tp.AccessToken
	return tp.AccessToken
}

func (e *testEnv) do(method, path, token string, body any, headers ...string) (int, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func mustJSON(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("bad json %q: %v", raw, err)
	}
}

func (e *testEnv) upload(token, appID, kind, filename string, content []byte) (int, []byte) {
	e.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("kind", kind)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	fw, _ := w.CreatePart(h)
	_, _ = fw.Write(content)
	_ = w.Close()
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/v1/applications/"+appID+"/documents", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

var pdfBytes = append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 200)...)

// draft creates a draft for the logged-in client and uploads all required documents.
func (e *testEnv) draft(login string, pid products.ID, amount int64, term int, withDocs bool) gen.Application {
	e.t.Helper()
	tok := e.token(login)
	code, raw := e.do("POST", "/api/v1/applications", tok, map[string]any{
		"productId": string(pid), "amount": amount, "termMonths": term, "purpose": "Закупка сырья на сезон",
	})
	if code != 201 {
		e.t.Fatalf("create draft: %d %s", code, raw)
	}
	var app gen.Application
	mustJSON(e.t, raw, &app)
	if withDocs {
		p, _ := products.Get(pid)
		for _, d := range p.Documents {
			if d.Required {
				if c, r := e.upload(tok, app.Id.String(), d.Kind, d.Kind+".pdf", pdfBytes); c != 201 {
					e.t.Fatalf("upload %s: %d %s", d.Kind, c, r)
				}
			}
		}
	}
	return app
}

func (e *testEnv) submit(login string, app gen.Application, key string) (int, gen.Application) {
	e.t.Helper()
	code, raw := e.do("POST", "/api/v1/applications/"+app.Id.String()+"/submit", e.token(login), nil, "Idempotency-Key", key)
	var out gen.Application
	if code == 200 {
		mustJSON(e.t, raw, &out)
	}
	return code, out
}

func (e *testEnv) get(login, id string) gen.Application {
	e.t.Helper()
	code, raw := e.do("GET", "/api/v1/applications/"+id, e.token(login), nil)
	if code != 200 {
		e.t.Fatalf("get application: %d %s", code, raw)
	}
	var out gen.Application
	mustJSON(e.t, raw, &out)
	return out
}

// run submits and drives the pipeline synchronously.
func (e *testEnv) run(login string, app gen.Application) gen.Application {
	e.t.Helper()
	if code, _ := e.submit(login, app, "key-"+app.Id.String()); code != 200 {
		e.t.Fatalf("submit failed: %d", code)
	}
	if err := e.pipe.Process(context.Background(), app.Id.String()); err != nil {
		e.t.Fatal(err)
	}
	return e.get(login, app.Id.String())
}

func (e *testEnv) staffCard(id string) gen.StaffApplication {
	e.t.Helper()
	code, raw := e.do("GET", "/api/v1/staff/applications/"+id, e.token("staff1"), nil)
	if code != 200 {
		e.t.Fatalf("staff card: %d %s", code, raw)
	}
	var out gen.StaffApplication
	mustJSON(e.t, raw, &out)
	return out
}

// submitDirect submits through the pipeline, bypassing the API completeness check.
func (e *testEnv) submitDirect(app gen.Application) {
	e.t.Helper()
	a, err := e.s.GetApplication(context.Background(), app.Id.String())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.pipe.Apply(context.Background(), a.ID, "Submit", "test", "", a.Version); err != nil {
		e.t.Fatal(err)
	}
	if err := e.pipe.Process(context.Background(), a.ID); err != nil {
		e.t.Fatal(err)
	}
}

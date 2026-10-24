package mocks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"matritsa/internal/integrations"
)

func get(h http.Handler, path string, hdr ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMockBKI_SeededINN_Deterministic(t *testing.T) {
	h := BKI(Options{})
	for i := 0; i < 2; i++ {
		rec := get(h, "/bki/7701234501")
		var rep integrations.BKIReport
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &rep) != nil {
			t.Fatalf("bad response %d %s", rec.Code, rec.Body)
		}
		if !rep.HasHistory || rep.Score != 790 || rep.MaxOverdueDays != 0 {
			t.Fatalf("unexpected report %+v", rep)
		}
	}
}

func TestMockBKI_MalformedINN_NoHistory(t *testing.T) {
	var rep integrations.BKIReport
	rec := get(BKI(Options{}), "/bki/5000000001")
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &rep) != nil || rep.HasHistory {
		t.Fatalf("unknown INN must return 200 with no history, got %d %s", rec.Code, rec.Body)
	}
}

func TestMockBKI_OverdueCompany(t *testing.T) {
	var rep integrations.BKIReport
	_ = json.Unmarshal(get(BKI(Options{}), "/bki/7701234506").Body.Bytes(), &rep)
	if rep.MaxOverdueDays != 120 {
		t.Fatalf("want 120 overdue days, got %d", rep.MaxOverdueDays)
	}
}

func TestMockEGRUL_Liquidated(t *testing.T) {
	var rec integrations.EGRULRecord
	r := get(EGRUL(Options{}), "/egrul/7701234504")
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &rec) != nil || rec.Status != "liquidated" {
		t.Fatalf("want liquidated, got %d %s", r.Code, r.Body)
	}
}

func TestMockEGRUL_MalformedINN_404(t *testing.T) {
	if r := get(EGRUL(Options{}), "/egrul/5000000001"); r.Code != 404 {
		t.Fatalf("want 404, got %d", r.Code)
	}
}

func TestMock_FailHeader_Returns503(t *testing.T) {
	if r := get(BKI(Options{}), "/bki/7701234501", "X-Mock-Fail", "1"); r.Code != 503 {
		t.Fatalf("X-Mock-Fail must produce 503, got %d", r.Code)
	}
}

func TestMock_FailSwitch_Toggle(t *testing.T) {
	opts := Options{Fail: new(FailSwitch)}
	h := BKI(opts)
	opts.Fail.Set(true)
	if r := get(h, "/bki/7701234501"); r.Code != 503 {
		t.Fatalf("switch on → 503, got %d", r.Code)
	}
	opts.Fail.Set(false)
	if r := get(h, "/bki/7701234501"); r.Code != 200 {
		t.Fatalf("switch off → 200, got %d", r.Code)
	}
}

func TestMocks_AnyValidINNGetsTheSameSyntheticCompany(t *testing.T) {
	var reg integrations.EGRULRecord
	var rep integrations.BKIReport
	for i := 0; i < 2; i++ {
		r := get(EGRUL(Options{}), "/egrul/5000000000")
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &reg) != nil || reg.INN != "5000000000" || reg.Name == "" {
			t.Fatalf("a valid unknown INN must resolve in the registry: %d %s", r.Code, r.Body)
		}
	}
	first := reg
	_ = json.Unmarshal(get(EGRUL(Options{}), "/egrul/5000000000").Body.Bytes(), &reg)
	if reg.Name != first.Name {
		t.Fatal("the registry record must be stable")
	}
	b := get(BKI(Options{}), "/bki/5000000000")
	if b.Code != 200 || json.Unmarshal(b.Body.Bytes(), &rep) != nil {
		t.Fatalf("bureau: %d %s", b.Code, b.Body)
	}
}

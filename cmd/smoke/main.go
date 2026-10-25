// Command smoke runs the main scenarios against a running stack (BASE_URL, default http://localhost:8080).
// It is the quick "does the whole thing work" check for the jury and for CI.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"time"
)

var base = envOr("BASE_URL", "http://localhost:8080")

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func fail(format string, a ...any) {
	fmt.Printf("SMOKE FAILED: "+format+"\n", a...)
	os.Exit(1)
}

func call(method, path, token string, body any, hdr ...string) (int, map[string]any, []byte) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m, raw
}

func login(l string) string {
	code, m, raw := call("POST", "/api/v1/auth/login", "", map[string]string{"login": l, "password": "demo1234"})
	if code != 200 {
		fail("login %s: %d %s", l, code, raw)
	}
	return m["accessToken"].(string)
}

func upload(tok, appID, kind string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("kind", kind)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+kind+`.pdf"`)
	fw, _ := w.CreatePart(h)
	_, _ = fw.Write(append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("demo "), 100)...))
	_ = w.Close()
	req, _ := http.NewRequest("POST", base+"/api/v1/applications/"+appID+"/documents", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 201 {
		fail("upload %s: %v", kind, err)
	}
	resp.Body.Close()
}

func waitStatus(tok, id, want string) map[string]any {
	deadline := time.Now().Add(40 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		_, m, _ := call("GET", "/api/v1/applications/"+id, tok, nil)
		last, _ = m["status"].(string)
		if last == want {
			return m
		}
		time.Sleep(500 * time.Millisecond)
	}
	fail("application %s stuck in %s, wanted %s", id, last, want)
	return nil
}

func newApp(tok string, product string, amount int64, term int, docs ...string) string {
	code, m, raw := call("POST", "/api/v1/applications", tok, map[string]any{"productId": product, "amount": amount, "termMonths": term, "purpose": "Закупка сырья на сезон"})
	if code != 201 {
		fail("create: %d %s", code, raw)
	}
	id := m["id"].(string)
	for _, d := range docs {
		upload(tok, id, d)
	}
	code, _, raw = call("POST", "/api/v1/applications/"+id+"/submit", tok, nil, "Idempotency-Key", "smoke-"+id)
	if code != 200 {
		fail("submit: %d %s", code, raw)
	}
	return id
}

func main() {
	deadline := time.Now().Add(90 * time.Second)
	for {
		resp, err := http.Get(base + "/readyz")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			fail("service did not become ready at %s", base)
		}
		time.Sleep(time.Second)
	}
	fmt.Println("✓ service ready")

	// Scenario 1: reliable bakery gets an automatic offer and takes it.
	c1 := login("client1")
	id1 := newApp(c1, "working_capital", 100_000_000, 12, "bank_statement", "financials")
	app := waitStatus(c1, id1, "OFFER_MADE")
	if app["offer"] == nil {
		fail("offer missing")
	}
	fmt.Println("✓ client1: automatic offer")
	code, m, raw := call("POST", "/api/v1/applications/"+id1+"/offer/accept", c1, nil)
	if code != 200 || m["status"] != "DISBURSED" {
		fail("accept: %d %s", code, raw)
	}
	fmt.Println("✓ client1: accepted, disbursed (demo)")

	// Scenario 2: indebted café goes to an analyst who makes a counter-offer.
	c2 := login("client2")
	id2 := newApp(c2, "working_capital", 300_000_000, 24, "bank_statement", "financials")
	waitStatus(c2, id2, "ANALYST_REVIEW")
	staff := login("staff1")
	code, card, raw := call("GET", "/api/v1/staff/applications/"+id2, staff, nil)
	if code != 200 {
		fail("staff card: %d %s", code, raw)
	}
	version := card["application"].(map[string]any)["version"]
	code, _, raw = call("POST", "/api/v1/staff/applications/"+id2+"/decision", staff, map[string]any{
		"action": "counter", "comment": "Предлагаем меньшую сумму", "version": version,
		"offer": map[string]any{"amount": 80_000_000, "termMonths": 24, "rateBp": 2450},
	})
	if code != 200 {
		fail("counter-offer: %d %s", code, raw)
	}
	waitStatus(c2, id2, "OFFER_MADE")
	fmt.Println("✓ client2: analyst counter-offer reached the client")

	// Scenario 3: liquidated company is rejected automatically.
	c4 := login("client4")
	id4 := newApp(c4, "working_capital", 50_000_000, 12, "bank_statement", "financials")
	waitStatus(c4, id4, "REJECTED")
	fmt.Println("✓ client4: automatic rejection with reasons")

	// Scenario 4: a client asks for a person, a support specialist joins and answers in the same talk.
	c3 := login("client3")
	code, thread, raw := call("POST", "/api/v1/assistant/chat", c3, map[string]any{"message": "Соедините меня с живым сотрудником"})
	if code != 200 || thread["status"] != "waiting" {
		fail("chat request for a person: %d %s", code, raw)
	}
	chatID := thread["id"].(string)
	sup := login("support1")
	if code, _, raw = call("POST", "/api/v1/support/chats/"+chatID+"/join", sup, nil); code != 200 {
		fail("support join: %d %s", code, raw)
	}
	if code, _, raw = call("POST", "/api/v1/support/chats/"+chatID+"/messages", sup, map[string]any{"message": "Здравствуйте! Чем помочь?"}); code != 200 {
		fail("support message: %d %s", code, raw)
	}
	code, thread, raw = call("GET", "/api/v1/assistant/chat", c3, nil)
	if code != 200 || thread["status"] != "live" {
		fail("client sees the talk: %d %s", code, raw)
	}
	if code, _, raw = call("POST", "/api/v1/support/chats/"+chatID+"/close", sup, nil); code != 200 {
		fail("support close: %d %s", code, raw)
	}
	fmt.Println("✓ client3: asked for a person, a support specialist answered and closed the talk")

	fmt.Println("SMOKE OK")
}

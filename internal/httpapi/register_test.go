package httpapi_test

import (
	"fmt"
	"testing"
	"time"

	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
	"matritsa/internal/seed"
)

func signup(login, inn string) map[string]any {
	return map[string]any{"fullName": "Иванов Иван Иванович", "login": login, "password": "s3cret-pass", "inn": inn, "consent": true}
}

// freshINN is a valid INN that is not a demo company and gives a reliable synthetic company.
func freshINN(t *testing.T, kind seed.Archetype) string {
	t.Helper()
	for n := int64(7_950_000_000); n < 7_950_100_000; n++ {
		inn := fmt.Sprintf("%010d", n)
		if seed.ValidINN(inn) && seed.ArchetypeOf(inn) == kind {
			return inn
		}
	}
	t.Fatal("no INN found")
	return ""
}

func TestRegister_CreatesAccountAndSignsIn(t *testing.T) {
	e := newEnv(t)
	inn := freshINN(t, seed.Reliable)
	code, raw := e.do("POST", "/api/v1/auth/register", "", signup("Ivanov.Test", inn))
	var tp gen.TokenPair
	mustJSON(t, raw, &tp)
	if code != 201 || tp.AccessToken == "" || tp.Role != gen.TokenPairRoleClient || tp.FullName != "Иванов Иван Иванович" {
		t.Fatalf("%d %s", code, raw)
	}
	code, raw = e.do("GET", "/api/v1/me", tp.AccessToken, nil)
	var me gen.Me
	mustJSON(t, raw, &me)
	if code != 200 || me.Login != "ivanov.test" || me.Company == nil || me.Company.Inn != inn {
		t.Fatalf("the new account must see its own company: %d %s", code, raw)
	}
	// the chosen password works for a normal sign-in afterwards
	if code, _ := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "IVANOV.test", "password": "s3cret-pass"}); code != 200 {
		t.Fatalf("login after sign-up: %d", code)
	}
}

func TestRegister_NewClientCanApplyAndGetAnAutomaticOffer(t *testing.T) {
	e := newEnv(t)
	inn := freshINN(t, seed.Reliable)
	_, raw := e.do("POST", "/api/v1/auth/register", "", signup("reliable1", inn))
	var tp gen.TokenPair
	mustJSON(t, raw, &tp)
	e.tok["reliable1"] = tp.AccessToken
	app := e.draft("reliable1", products.WorkingCapital, 1_000_000*rub, 12, true)
	done := e.run("reliable1", app)
	if string(done.Status) != "OFFER_MADE" {
		t.Fatalf("a reliable synthetic company must get an offer on its own, got %s", done.Status)
	}
}

func TestRegister_RejectsBadForms(t *testing.T) {
	e := newEnv(t)
	inn := freshINN(t, seed.Borderline)
	cases := map[string]func(m map[string]any){
		"short login":      func(m map[string]any) { m["login"] = "ab" },
		"login with space": func(m map[string]any) { m["login"] = "иван иванов" },
		"short password":   func(m map[string]any) { m["password"] = "short" },
		"long password":    func(m map[string]any) { m["password"] = string(make([]byte, 73)) },
		"empty name":       func(m map[string]any) { m["fullName"] = " " },
		"bad INN checksum": func(m map[string]any) { m["inn"] = "5000000001" },
		"no consent":       func(m map[string]any) { m["consent"] = false },
	}
	for name, mutate := range cases {
		body := signup("form-"+fmt.Sprint(len(name)), inn)
		mutate(body)
		if code, raw := e.do("POST", "/api/v1/auth/register", "", body); code != 422 {
			t.Errorf("%s: want 422, got %d %s", name, code, raw)
		}
	}
}

func TestRegister_LoginTaken_409(t *testing.T) {
	e := newEnv(t)
	inn := freshINN(t, seed.Borderline)
	if code, _ := e.do("POST", "/api/v1/auth/register", "", signup("client1", inn)); code != 409 {
		t.Fatalf("a demo login must stay taken: got %d", code)
	}
	// and nothing was created along the way: the INN is still free
	if code, raw := e.do("POST", "/api/v1/auth/register", "", signup("free-login", inn)); code != 201 {
		t.Fatalf("the INN must still be free: %d %s", code, raw)
	}
}

func TestRegister_CompanyAlreadyRegistered_409(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.do("POST", "/api/v1/auth/register", "", signup("someone-else", "7701234501")); code != 409 {
		t.Fatalf("a demo company must not be taken over: got %d", code)
	}
	inn := freshINN(t, seed.Young)
	if code, _ := e.do("POST", "/api/v1/auth/register", "", signup("first-owner", inn)); code != 201 {
		t.Fatalf("first sign-up: %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/auth/register", "", signup("second-owner", inn)); code != 409 {
		t.Fatalf("second sign-up on the same company must be refused, got %d", code)
	}
	// the first owner's data was not touched
	if code, _ := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "first-owner", "password": "s3cret-pass"}); code != 200 {
		t.Fatal("first owner must still sign in")
	}
}

func TestRegister_RegistryDown_503(t *testing.T) {
	e := newEnv(t)
	e.src.down.Store(true)
	if code, _ := e.do("POST", "/api/v1/auth/register", "", signup("registry-down", freshINN(t, seed.Reliable))); code != 503 {
		t.Fatalf("want 503, got %d", code)
	}
}

func TestRegister_RateLimitedPerAddress(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 10; i++ {
		e.do("POST", "/api/v1/auth/register", "", signup("x", "5000000001"), "X-Forwarded-For", "203.0.113.50")
	}
	if code, _ := e.do("POST", "/api/v1/auth/register", "", signup("spam", freshINN(t, seed.Reliable)), "X-Forwarded-For", "203.0.113.50"); code != 429 {
		t.Fatalf("the 11th attempt from one address must wait, got %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/auth/register", "", signup("other-person", freshINN(t, seed.Reliable)), "X-Forwarded-For", "198.51.100.77"); code != 201 {
		t.Fatalf("another address must not be affected, got %d", code)
	}
}

func TestRegister_CannotCreateStaff(t *testing.T) {
	e := newEnv(t)
	body := signup("sneaky", freshINN(t, seed.Reliable))
	body["role"] = "staff"
	code, raw := e.do("POST", "/api/v1/auth/register", "", body)
	var tp gen.TokenPair
	mustJSON(t, raw, &tp)
	if code != 201 || tp.Role != gen.TokenPairRoleClient {
		t.Fatalf("the form has no role: it must always be a client: %d %s", code, raw)
	}
	if code, _ := e.do("GET", "/api/v1/staff/queue", tp.AccessToken, nil); code != 403 {
		t.Fatalf("a signed-up client must not reach the staff workspace: %d", code)
	}
}

func TestSignupLookupAndExamples_ArePublic(t *testing.T) {
	e := newEnv(t)
	inn := freshINN(t, seed.Reliable)
	code, raw := e.do("GET", "/api/v1/auth/company?inn="+inn, "", nil)
	var c gen.CompanyLookup
	mustJSON(t, raw, &c)
	if code != 200 || c.Inn != inn || c.Name == "" {
		t.Fatalf("%d %s", code, raw)
	}
	if code, _ := e.do("GET", "/api/v1/auth/company?inn=5000000001", "", nil); code != 422 {
		t.Fatalf("bad checksum: want 422, got %d", code)
	}
	if code, _ := e.do("GET", "/api/v1/auth/company?inn=7701234501", "", nil); code != 409 {
		t.Fatalf("a registered company: want 409, got %d", code)
	}
	code, raw = e.do("GET", "/api/v1/auth/registration-examples", "", nil)
	var ex []gen.RegistrationExample
	mustJSON(t, raw, &ex)
	if code != 200 || len(ex) != 7 {
		t.Fatalf("%d %s", code, raw)
	}
	for _, x := range ex {
		if code, _ := e.do("GET", "/api/v1/auth/company?inn="+x.Inn, "", nil); code != 200 {
			t.Errorf("example %s must be free to register: %d", x.Inn, code)
		}
	}
}

func TestMe_ShowsWhenConsentWasGiven_OnlyForSelfRegisteredAccounts(t *testing.T) {
	e := newEnv(t)
	_, raw := e.do("POST", "/api/v1/auth/register", "", signup("with-consent", freshINN(t, seed.Reliable)))
	var tp gen.TokenPair
	mustJSON(t, raw, &tp)
	_, raw = e.do("GET", "/api/v1/me", tp.AccessToken, nil)
	var me gen.Me
	mustJSON(t, raw, &me)
	if me.ConsentedAt == nil || time.Since(*me.ConsentedAt) > time.Minute {
		t.Fatalf("a registered account must show when it consented: %+v", me.ConsentedAt)
	}
	_, raw = e.do("GET", "/api/v1/me", e.token("client1"), nil)
	var demo gen.Me
	mustJSON(t, raw, &demo)
	if demo.ConsentedAt != nil {
		t.Fatal("a demo account never consented")
	}
}

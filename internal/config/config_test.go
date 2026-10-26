package config

import (
	"net/netip"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"HTTP_ADDR", "DATABASE_URL", "JWT_SECRET", "PRODUCT_NAME", "BKI_URL", "EGRUL_URL", "OLLAMA_URL", "OLLAMA_MODEL", "AI_ENABLED", "APP_ENV", "DATA_DIR", "TRUSTED_PROXIES"} {
		t.Setenv(k, "")
	}
}

func TestLoad_DefaultsAndOverrides(t *testing.T) {
	clearEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ProductName != "Матрица" || !c.AIEnabled || c.HTTPAddr != ":8080" {
		t.Fatalf("bad defaults: %+v", c)
	}
	t.Setenv("PRODUCT_NAME", "Икс")
	t.Setenv("AI_ENABLED", "false")
	t.Setenv("HTTP_ADDR", ":9000")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ProductName != "Икс" || c.AIEnabled || c.HTTPAddr != ":9000" {
		t.Fatalf("overrides ignored: %+v", c)
	}
}

func TestLoad_ProdRequiresJWTSecret(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_ENV", "prod")
	if _, err := Load(); err == nil {
		t.Fatal("prod without JWT_SECRET must fail")
	}
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load(); err != nil {
		t.Fatalf("prod with secret must load: %v", err)
	}
}

func TestLoad_BadBoolFails(t *testing.T) {
	clearEnv(t)
	t.Setenv("AI_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Fatal("invalid AI_ENABLED must fail")
	}
}

func TestLoad_DemoDataIsOffInProdUnlessAsked(t *testing.T) {
	clearEnv(t)
	t.Setenv("SEED_ON_START", "")
	c, err := Load()
	if err != nil || !c.SeedOnStart {
		t.Fatalf("dev loads demo data by default: %+v %v", c, err)
	}
	t.Setenv("APP_ENV", "prod")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	if c, err = Load(); err != nil || c.SeedOnStart {
		t.Fatalf("prod must not create demo users by default: %+v %v", c, err)
	}
	t.Setenv("SEED_ON_START", "true")
	if c, err = Load(); err != nil || !c.SeedOnStart {
		t.Fatalf("explicit request must be honoured: %+v %v", c, err)
	}
}

func TestLoad_TrustedProxies(t *testing.T) {
	clearEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	trusts := func(c Config, ip string) bool {
		for _, p := range c.TrustedProxies {
			if p.Contains(netip.MustParseAddr(ip)) {
				return true
			}
		}
		return false
	}
	// By default the proxy is somewhere in a private network (nginx in Docker Compose), never on the internet.
	if !trusts(c, "172.18.0.5") || !trusts(c, "127.0.0.1") || trusts(c, "203.0.113.7") {
		t.Fatalf("default trusted proxies: %v", c.TrustedProxies)
	}
	t.Setenv("TRUSTED_PROXIES", "10.1.2.3/32, 192.168.7.0/24")
	if c, err = Load(); err != nil || !trusts(c, "10.1.2.3") || trusts(c, "10.1.2.4") || !trusts(c, "192.168.7.9") {
		t.Fatalf("explicit list: %v %v", c.TrustedProxies, err)
	}
	t.Setenv("TRUSTED_PROXIES", "none")
	if c, err = Load(); err != nil || len(c.TrustedProxies) != 0 {
		t.Fatalf("none must trust nobody: %v %v", c.TrustedProxies, err)
	}
	t.Setenv("TRUSTED_PROXIES", "not-a-network")
	if _, err = Load(); err == nil {
		t.Fatal("a bad list must stop the start")
	}
}

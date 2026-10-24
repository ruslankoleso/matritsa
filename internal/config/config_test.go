package config

import "testing"

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"HTTP_ADDR", "DATABASE_URL", "JWT_SECRET", "PRODUCT_NAME", "BKI_URL", "EGRUL_URL", "OLLAMA_URL", "OLLAMA_MODEL", "AI_ENABLED", "APP_ENV", "DATA_DIR"} {
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

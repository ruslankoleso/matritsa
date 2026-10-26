// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env         string // dev | prod
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	ProductName string
	DataDir     string
	BKIURL      string
	EGRULURL    string
	OllamaURL   string
	OllamaModel string
	AIEnabled   bool
	SeedOnStart bool
}

func Load() (Config, error) {
	c := Config{
		Env:         get("APP_ENV", "dev"),
		HTTPAddr:    get("HTTP_ADDR", ":8080"),
		DatabaseURL: get("DATABASE_URL", "postgres://matritsa:matritsa@localhost:5433/matritsa?sslmode=disable"),
		JWTSecret:   get("JWT_SECRET", ""),
		ProductName: get("PRODUCT_NAME", "Матрица"),
		DataDir:     get("DATA_DIR", "./data"),
		BKIURL:      get("BKI_URL", "http://localhost:8081"),
		EGRULURL:    get("EGRUL_URL", "http://localhost:8082"),
		OllamaURL:   get("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel: get("OLLAMA_MODEL", "hf.co/t-tech/T-lite-it-2.1-GGUF:Q4_K_M"),
	}
	ai, err := parseBool(get("AI_ENABLED", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("AI_ENABLED: %w", err)
	}
	c.AIEnabled = ai
	// Demo users with a published password must not appear in a real environment by accident.
	seedDefault := "true"
	if c.Env == "prod" {
		seedDefault = "false"
	}
	seedStart, err := parseBool(get("SEED_ON_START", seedDefault))
	if err != nil {
		return Config{}, fmt.Errorf("SEED_ON_START: %w", err)
	}
	c.SeedOnStart = seedStart
	if c.JWTSecret == "" {
		if c.Env == "prod" {
			return Config{}, fmt.Errorf("JWT_SECRET is required when APP_ENV=prod")
		}
		c.JWTSecret = "dev-only-secret-change-me-0123456789"
	}
	return c, nil
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseBool(s string) (bool, error) {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("expected true/false, got %q", s)
	}
	return b, nil
}

// Package storetest creates throwaway migrated databases for integration tests.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"matritsa/internal/store"
)

const defaultURL = "postgres://matritsa:matritsa@localhost:5433/matritsa?sslmode=disable"

func adminURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return defaultURL
}

// New returns a Store on a fresh, migrated database that is dropped on cleanup.
func New(t testing.TB) *store.Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminURL())
	if err != nil {
		t.Fatalf("test database unavailable (run `docker compose up -d postgres`): %v", err)
	}
	defer admin.Close(ctx)

	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "test_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	u, err := url.Parse(adminURL())
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		c2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()
		a, err := pgx.Connect(c2, adminURL())
		if err != nil {
			return
		}
		defer a.Close(c2)
		_, _ = a.Exec(c2, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return s
}

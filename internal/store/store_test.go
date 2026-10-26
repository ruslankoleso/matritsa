package store_test

import (
	"context"
	"testing"

	"matritsa/internal/seed"
	"matritsa/internal/store"
	"matritsa/internal/store/storetest"
)

func TestMigrate_Up_Down_Up(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	if err := s.MigrateDown(ctx); err != nil {
		t.Fatalf("down: %v", err)
	}
	var n int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_name = 'applications'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("applications table must be gone after down: n=%d err=%v", n, err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func TestSeed_Idempotent(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := store.Seed(ctx, s); err != nil {
			t.Fatalf("seed run %d: %v", i, err)
		}
	}
	companies, _ := s.CountCompanies(ctx)
	users, _ := s.CountUsers(ctx)
	wantCompanies := int64(len(seed.Companies()))
	wantUsers := wantCompanies + int64(len(seed.StaffAccounts()))
	if int64(companies) != wantCompanies || int64(users) != wantUsers {
		t.Fatalf("want %d companies and %d users, got %d and %d", wantCompanies, wantUsers, companies, users)
	}
}

func TestSeed_FinancialsStored(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	if err := store.Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	c, err := s.GetCompanyByINN(ctx, "7701234501")
	if err != nil {
		t.Fatal(err)
	}
	fin, err := s.ListFinancials(ctx, c.ID)
	if err != nil || len(fin) != 12 {
		t.Fatalf("want 12 months of revenue, got %d err=%v", len(fin), err)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	err := s.WithTx(ctx, func(q *dbQueries) error {
		if _, err := q.UpsertUser(ctx, upsertUserParams()); err != nil {
			return err
		}
		return context.Canceled
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if n, _ := s.CountUsers(ctx); n != 0 {
		t.Fatalf("user must be rolled back, got %d", n)
	}
}

func TestTryLock_OneHolderAtATime(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	release, ok, err := s.TryLock(ctx, "pipeline:a1")
	if err != nil || !ok {
		t.Fatalf("first lock: ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.TryLock(ctx, "pipeline:a1"); err != nil || ok {
		t.Fatalf("second holder must be refused while the first holds it: ok=%v err=%v", ok, err)
	}
	if r, ok, err := s.TryLock(ctx, "pipeline:a2"); err != nil || !ok {
		t.Fatalf("another key is independent: ok=%v err=%v", ok, err)
	} else {
		r()
	}
	release()
	r, ok, err := s.TryLock(ctx, "pipeline:a1")
	if err != nil || !ok {
		t.Fatalf("after release the lock is free: ok=%v err=%v", ok, err)
	}
	r()
}

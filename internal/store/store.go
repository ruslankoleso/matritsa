// Package store wraps PostgreSQL access: pool, migrations, transactions.
package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"matritsa/internal/store/db"
	"matritsa/migrations"
)

type Store struct {
	Pool *pgxpool.Pool
	*db.Queries
}

// Open connects to the database.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{Pool: pool, Queries: db.New(pool)}, nil
}

func (s *Store) Close() { s.Pool.Close() }

func withGoose(pool *pgxpool.Pool, fn func(*sql.DB) error) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return fn(sqlDB)
}

// Migrate applies all pending migrations.
func (s *Store) Migrate(ctx context.Context) error {
	return withGoose(s.Pool, func(d *sql.DB) error { return goose.UpContext(ctx, d, ".") })
}

// MigrateDown rolls back every migration (used by tests).
func (s *Store) MigrateDown(ctx context.Context) error {
	return withGoose(s.Pool, func(d *sql.DB) error { return goose.DownToContext(ctx, d, ".", 0) })
}

// WithTx runs fn in a transaction; it commits on nil error and rolls back otherwise.
func (s *Store) WithTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.Queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// TryLock takes a PostgreSQL advisory lock named by key without waiting. ok=false means another
// connection, possibly another instance of the API, holds it. The lock lives on one pooled connection
// and goes away with it, so a crashed instance never leaves it behind. release must be called when ok.
func (s *Store) TryLock(ctx context.Context, key string) (release func(), ok bool, err error) {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, key).Scan(&ok); err != nil || !ok {
		conn.Release()
		return nil, false, err
	}
	return func() {
		// A fresh context: the caller's may be cancelled already, and the lock must still be given back.
		if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, key); err != nil {
			conn.Conn().Close(context.Background()) // closing the connection drops the lock too
		}
		conn.Release()
	}, true, nil
}

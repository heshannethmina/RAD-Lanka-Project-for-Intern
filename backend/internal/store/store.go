// Package store is the Postgres access layer: users, login sessions and
// rooms. Nothing here knows about HTTP, and nothing above here writes SQL.
//
// This is the application's own database. Judge0 has a separate one that this
// package must never be pointed at — see docker-compose.app.yml.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned instead of pgx.ErrNoRows, so callers do not have to
// import pgx to tell "no such row" from "the database is broken". Everything
// above this package should treat those two very differently.
var ErrNotFound = errors.New("store: not found")

// ErrConflict means a uniqueness constraint rejected the write — today only
// ever a duplicate email on registration.
var ErrConflict = errors.New("store: conflict")

// Store owns a connection pool. The zero value is not usable; use Open.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects and verifies the connection before returning, so a bad
// DATABASE_URL fails at startup rather than on the first request a user
// makes.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: parse DATABASE_URL: %w", err)
	}

	// A dev laptop and a small VPS both do better with a bounded pool than
	// with Postgres' default of "as many as asked for". Interview traffic is
	// tiny; the WebSocket hub holds no connections at all.
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close releases the pool. Safe to call on a nil Store so shutdown paths do
// not have to check.
func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/heshannethmina/interview-platform/backend/migrations"
)

// Migrate applies every migration that has not run yet, in filename order.
//
// Deliberately hand-rolled rather than pulled from a library. The whole job is
// "read a table, compare, run the rest in order", the schema is small, and a
// migration tool is one more thing to install on the deploy host. If this ever
// needs branching or down-migrations, replace it then, not now.
//
// Each migration runs inside its own transaction together with the row that
// records it, so a failure leaves neither a half-applied schema nor a lie in
// schema_migrations. Postgres has transactional DDL; this would not be safe on
// MySQL.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	applied, err := s.appliedVersions(ctx)
	if err != nil {
		return err
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		if err := s.applyMigration(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) appliedVersions(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("store: read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("store: scan schema_migrations: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: read schema_migrations: %w", err)
	}
	return applied, nil
}

func migrationNames() ([]string, error) {
	entries, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("store: list migrations: %w", err)
	}
	// Glob's order is not documented as sorted, and the whole scheme depends
	// on order, so sort explicitly rather than trusting it.
	sort.Strings(entries)
	return entries, nil
}

func (s *Store) applyMigration(ctx context.Context, name string) error {
	body, err := migrations.FS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("store: read migration %s: %w", name, err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin migration %s: %w", name, err)
	}
	// Rollback after a successful Commit is a no-op, so this needs no flag.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, string(body)); err != nil {
		return fmt.Errorf("store: apply migration %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("store: record migration %s: %w", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit migration %s: %w", name, err)
	}
	return nil
}

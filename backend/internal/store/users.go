package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is Postgres' SQLSTATE for a broken unique constraint.
const uniqueViolation = "23505"

// User is an interviewer. Candidates are deliberately not users: they arrive
// through an invite link and never hold an account, which is most of the
// point of the shareable link.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// CreateUser inserts an interviewer, returning ErrConflict if the email is
// already registered.
//
// The caller passes an already-hashed password: this package must never see a
// plaintext one, so there is no way for it to end up in a query log.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at
	`, email, passwordHash).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("store: create user: %w", err)
	}
	return &u, nil
}

// UserByEmail looks up an interviewer for login. The match is
// case-insensitive, so somebody who registered as Alice@ can log in as alice@.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE lower(email) = lower($1)
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: user by email: %w", err)
	}
	return &u, nil
}

// UserByID looks up an interviewer by primary key.
func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: user by id: %w", err)
	}
	return &u, nil
}

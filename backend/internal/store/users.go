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
	// Plan is the tier they are subscribed to. Plain text; internal/plan
	// turns it into limits and falls back to Free for anything it does not
	// recognise.
	Plan string
	// PromoCode, PromoPlan and PromoExpiresAt are a redeemed promotion. They
	// *override* Plan while live rather than replacing it, so when a grant
	// lapses somebody falls back to what they actually pay for instead of
	// being dropped to Free.
	//
	// A lapsed grant is left in the row rather than swept, for the same reason
	// an expired session is: the check is on the read path, so a stale grant
	// is inert, and clearing it would make every request a write.
	PromoCode      string
	PromoPlan      string
	PromoExpiresAt *time.Time
	CreatedAt      time.Time
}

// PromoActive reports whether a redeemed promotion still applies.
//
// A NULL expiry never lapses, which is the normal case: a code handed to a
// pilot customer is meant to keep working until somebody takes it away.
func (u *User) PromoActive() bool {
	if u.PromoPlan == "" {
		return false
	}
	return u.PromoExpiresAt == nil || u.PromoExpiresAt.After(time.Now())
}

// userColumns is the select list every query that returns a User uses, and
// scanUser is its matching destination list. One pair, so adding a column
// cannot half-land in some queries and not others.
//
// Qualified with "u." because sessions.go reads a user out of a join, and
// sessions has a created_at of its own — an unqualified list is ambiguous
// there. Every query below therefore aliases the table as u, the INSERT
// included.
//
// The two text columns are COALESCEd because "" already means "no promotion";
// a *string would push a nil check onto every caller for no extra state.
const userColumns = `u.id, u.email, u.password_hash, u.plan,
	COALESCE(u.promo_code, ''), COALESCE(u.promo_plan, ''),
	u.promo_expires_at, u.created_at`

func scanUser(u *User) []any {
	return []any{
		&u.ID, &u.Email, &u.PasswordHash, &u.Plan,
		&u.PromoCode, &u.PromoPlan, &u.PromoExpiresAt, &u.CreatedAt,
	}
}

// CreateUser inserts an interviewer, returning ErrConflict if the email is
// already registered.
//
// The caller passes an already-hashed password: this package must never see a
// plaintext one, so there is no way for it to end up in a query log.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users AS u (email, password_hash)
		VALUES ($1, $2)
		RETURNING `+userColumns, email, passwordHash).Scan(scanUser(&u)...)

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
		SELECT `+userColumns+`
		FROM users u
		WHERE lower(u.email) = lower($1)
	`, email).Scan(scanUser(&u)...)

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
		SELECT `+userColumns+`
		FROM users u
		WHERE u.id = $1
	`, id).Scan(scanUser(&u)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: user by id: %w", err)
	}
	return &u, nil
}

package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SessionTTL is how long a login lasts. Long enough that an interviewer is not
// logged out mid-interview, short enough that a forgotten session on a shared
// machine does not last a month.
const SessionTTL = 7 * 24 * time.Hour

// CreateSession records a login. tokenHash is auth.HashToken of the token
// handed to the client; the plaintext never reaches this package.
func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, tokenHash, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("store: create session: %w", err)
	}
	return nil
}

// UserBySessionToken resolves a session token hash to its owner.
//
// Expiry is enforced here in the WHERE clause rather than by comparing in Go,
// so there is exactly one place a stale session can be honoured — none. The
// expired row is left for DeleteExpiredSessions to sweep; leaving it costs
// nothing and deleting it on the read path would make every authenticated
// request a write.
func (s *Store) UserBySessionToken(ctx context.Context, tokenHash []byte) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, tokenHash).Scan(scanUser(&u)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: user by session token: %w", err)
	}
	return &u, nil
}

// DeleteSession logs out one session. Deleting a row that is not there is not
// an error: logging out twice, or with an already-expired token, should
// succeed quietly rather than hand the caller something to distinguish.
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("store: delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes sessions that are past their expiry and
// reports how many went. Expired rows are already unusable — this is
// housekeeping, not a security control, so it is safe to run rarely.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("store: delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

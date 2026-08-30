// Package auth holds the credential primitives: password hashing, and the
// generation and hashing of the opaque tokens used for login sessions and
// candidate invite links.
//
// There is no JWT here on purpose. Tokens are random bytes looked up in
// Postgres, which means logging out actually revokes access rather than
// waiting for an expiry claim, and there is no signing key to rotate or leak.
// The cost is one indexed lookup per authenticated request, which is nothing
// beside the WebSocket it is guarding.
package auth

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLen is deliberately low. Length rules push people toward
// "Password1!"; the real protection here is bcrypt plus rate limiting.
const MinPasswordLen = 8

// maxPasswordBytes is bcrypt's own limit. Go's implementation errors above it
// rather than truncating — some others truncate silently, which would make
// every password identical past 72 bytes. Rejecting early gives a clear
// message instead of a confusing internal error.
const maxPasswordBytes = 72

var (
	ErrPasswordTooShort = fmt.Errorf("auth: password must be at least %d characters", MinPasswordLen)
	ErrPasswordTooLong  = fmt.Errorf("auth: password must be at most %d bytes", maxPasswordBytes)
)

// HashPassword returns a bcrypt hash suitable for storing.
//
// The cost is bcrypt.DefaultCost (10). Raising it slows login for everyone as
// well as an attacker, and at this scale 10 is the accepted trade-off; revisit
// only with a measurement in hand.
func HashPassword(plain string) (string, error) {
	if utf8.RuneCountInString(plain) < MinPasswordLen {
		return "", ErrPasswordTooShort
	}
	if len(plain) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}

	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword reports whether plain matches hash.
//
// It returns a bool rather than an error because callers must not branch on
// why a login failed: "no such user" and "wrong password" have to be
// indistinguishable to the caller, or the endpoint becomes a way to find out
// which email addresses are registered.
func CheckPassword(hash, plain string) bool {
	if hash == "" || len(plain) > maxPasswordBytes {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	return err == nil
}

// DummyPasswordHash is a valid bcrypt hash of a value nobody knows.
//
// Login compares against this when the email does not exist, so the request
// costs the same bcrypt work either way. Without it, a fast rejection reveals
// that an address is not registered — a timing oracle over the user list.
var DummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// ErrInvalidCredentials is what an endpoint should surface for any failed
// login, regardless of which half was wrong.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

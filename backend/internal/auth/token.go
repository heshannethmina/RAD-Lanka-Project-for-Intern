package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

// tokenBytes is the entropy behind a session or invite token. 32 bytes is
// 256 bits: not guessable, and short enough that the encoded form still fits
// comfortably in a URL someone pastes into a chat window.
const tokenBytes = 32

// TokenHashLen is the width of a token hash, and so of the BYTEA columns that
// store one.
const TokenHashLen = sha256.Size

// NewToken returns a fresh token and its hash.
//
// The plaintext is shown to the user exactly once — in a Set-Cookie, or in the
// invite link — and only the hash is stored. A stolen database backup
// therefore yields no working sessions and no working invite links.
//
// Base64 URL encoding without padding: the invite token travels in a query
// string, and '+', '/' and '=' all have to be escaped there.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the OS entropy source is broken. There is
		// no safe fallback; refuse rather than issue a guessable token.
		return "", nil, fmt.Errorf("auth: generate token: %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken maps a token to the value stored in the database.
//
// SHA-256, not bcrypt: the input is already 256 bits of randomness, so there
// is nothing for a slow hash to defend against, and this runs on every
// authenticated request including every WebSocket upgrade.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// roomIDBytes gives a 16-character base64 ID. Unguessable, and short enough
// to read aloud over a call, which people will do.
const roomIDBytes = 12

// NewRoomID returns a random room identifier.
//
// Random rather than sequential because the ID is the URL both participants
// share, and a counter would let anyone enumerate live interviews. The
// alphabet of base64url — letters, digits, '-' and '_' — is exactly the set
// ws.ValidRoomID accepts, so a generated ID always survives that check.
func NewRoomID() (string, error) {
	b := make([]byte, roomIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate room id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// EqualHash compares two token hashes in constant time.
//
// Lookups go through the database by primary key and so do not need this, but
// any in-process comparison of a secret does — a byte-by-byte compare leaks
// how much of a guess was correct.
func EqualHash(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

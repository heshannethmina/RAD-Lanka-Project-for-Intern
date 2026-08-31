package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/plan"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// maxPromoBodyBytes bounds a redemption payload: one short code.
const maxPromoBodyBytes = 1024

// A promotion code is guessable in a way a session token is not. It has to be
// short enough to read off a slide and type, which puts it within reach of a
// script — and the prize for finding one is unmetered use of the product.
//
// So redemption is rate limited, per account, in memory. Per account rather
// than per IP because the route is behind RequireAuth: an attacker must at
// least hold a session, registration is the cheaper thing to limit, and an IP
// is shared by everyone behind one office NAT. In memory rather than in
// Postgres because a failed guess must not cost a write — that would turn the
// defence into the amplifier.
//
// The counter resets on restart, which is a real limitation and the reason
// codes should still be long enough not to be guessed in a burst. It is not
// the only control: max_redemptions and expires_at bound what a found code is
// worth.
const (
	promoAttemptLimit  = 10
	promoAttemptWindow = time.Hour
)

// promoLimiter counts recent failed redemptions per account.
//
// A mutex is right here and does not contradict the rule about room state:
// that rule is about the document and the client set, which belong to one hub
// goroutine. This is a small shared counter touched once per request from
// whichever goroutine net/http happened to hand the request to, and giving it
// an owner goroutine plus a channel would be ceremony around a map.
type promoLimiter struct {
	mu      sync.Mutex
	windows map[int64]*promoWindow
}

type promoWindow struct {
	failures int
	resetAt  time.Time
}

func newPromoLimiter() *promoLimiter {
	return &promoLimiter{windows: make(map[int64]*promoWindow)}
}

// allow reports whether this account may make another attempt.
func (l *promoLimiter) allow(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.windows[userID]
	if !ok || time.Now().After(w.resetAt) {
		l.windows[userID] = &promoWindow{failures: 1, resetAt: time.Now().Add(promoAttemptWindow)}
		return true
	}
	if w.failures >= promoAttemptLimit {
		return false
	}
	w.failures++
	return true
}

// fail records a wrong guess. Only failures count: somebody redeeming three
// legitimate codes they were sent is not the behaviour being limited.
func (l *promoLimiter) fail(userID int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Sweep here rather than on a timer: the map only grows when somebody
	// mistypes a code, and this is the only path that adds to it.
	now := time.Now()
	for id, w := range l.windows {
		if now.After(w.resetAt) {
			delete(l.windows, id)
		}
	}

	w, ok := l.windows[userID]
	if !ok {
		l.windows[userID] = &promoWindow{failures: 1, resetAt: now.Add(promoAttemptWindow)}
		return
	}
	w.failures++
}

// RedeemPromo applies a promotion code to the signed-in account.
//
// It answers with the same shape as /api/me, so the client replaces its user
// wholesale and the new limits are on screen without a second round trip.
func RedeemPromo(s *store.Store) http.HandlerFunc {
	limiter := newPromoLimiter()

	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		var req struct {
			Code string `json:"code"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxPromoBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}

		code := store.NormalizePromoCode(req.Code)
		if code == "" {
			writeError(w, http.StatusBadRequest, "enter a code")
			return
		}

		if !limiter.allow(u.ID) {
			writeError(w, http.StatusTooManyRequests,
				"too many attempts. Try again in an hour.")
			return
		}

		// plan.Grantable is passed down rather than applied to the result,
		// so a code with a typo in its plan column rolls the transaction back
		// instead of consuming the one redemption this account gets. Vetting
		// it here at all is because codes are written into the table by hand:
		// an unknown tier would otherwise fall through ByName to Free and the
		// person would be congratulated on an upgrade that did nothing.
		grant, err := s.RedeemPromoCode(r.Context(), code, u.ID, plan.Grantable)
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "that code is not valid")
			return
		case errors.Is(err, store.ErrPromoExpired):
			// Told apart from "not valid" on purpose. It does leak that the
			// code once existed, which is worth almost nothing — an expired
			// coupon grants nothing — against somebody otherwise retyping a
			// code that was never going to work.
			writeError(w, http.StatusGone, "that code has expired")
			return
		case errors.Is(err, store.ErrPromoExhausted):
			writeError(w, http.StatusGone, "that code has been fully claimed")
			return
		case errors.Is(err, store.ErrPromoAlreadyRedeemed):
			writeError(w, http.StatusConflict, "you have already used that code")
			return
		case errors.Is(err, store.ErrPromoMisconfigured):
			// Our mistake, not theirs, and it has cost them nothing — the
			// transaction rolled back, so the code still works once fixed.
			log.Printf("api: redeem promo: %v", err)
			writeError(w, http.StatusInternalServerError,
				"that code is not set up correctly. Please get in touch.")
			return
		case err != nil:
			log.Printf("api: redeem promo: %v", err)
			writeError(w, http.StatusInternalServerError, "could not redeem that code")
			return
		}

		// Re-read rather than patching the context's copy: the row is what the
		// next request will see, and building the response from anything else
		// is how a UI ends up disagreeing with the server.
		fresh, err := s.UserByID(r.Context(), u.ID)
		if err != nil {
			log.Printf("api: redeem promo: reload user: %v", err)
			writeError(w, http.StatusInternalServerError, "could not redeem that code")
			return
		}

		fingerprint := sha256.Sum256([]byte(grant.Code))
		log.Printf("api: user %d redeemed promo %s (plan %s)", u.ID, hex.EncodeToString(fingerprint[:6]), grant.Plan)
		writeJSON(w, http.StatusOK, meFor(r.Context(), s, fresh))
	}
}

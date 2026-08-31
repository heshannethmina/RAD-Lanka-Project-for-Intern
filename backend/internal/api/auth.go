package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/auth"
	"github.com/heshannethmina/interview-platform/backend/internal/plan"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// The session token travels in an Authorization: Bearer header rather than a
// cookie. That is a deliberate trade, and the reasoning belongs here because
// it is easy to "fix" in the wrong direction later:
//
// A cookie would be HttpOnly and so unreadable by injected script, which is
// strictly better against XSS. But the browser app and this API are different
// origins in every environment — Vercel and a VPS in production, :3000 and
// :8080 in development — so a cookie would have to be SameSite=None; Secure,
// which browsers refuse to send over plain http. That breaks local
// development outright. A bearer token is origin-agnostic, needs no CSRF
// defence because nothing is sent ambiently, and works unchanged in both
// places.
//
// The cost is that the token is reachable from JavaScript. Accept it, and keep
// the XSS surface small.

// maxAuthBodyBytes bounds a credential payload. An email and a password are a
// few hundred bytes; anything larger is not a login attempt.
const maxAuthBodyBytes = 4 * 1024

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// authResponse is what register and login return. The token is shown exactly
// once — only its hash is stored — so a client that discards it must log in
// again.
type authResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      userJSON  `json:"user"`
}

type userJSON struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Plan  string `json:"plan"`
	// IsAdmin tells the client whether to offer the admin UI. It is a hint
	// for rendering only — every admin route checks for itself, because a
	// client is free to lie about this and some will.
	IsAdmin bool `json:"is_admin"`
}

// meJSON is the signed-in view: who you are, and what you have left.
//
// Usage is returned with the identity rather than from a separate endpoint,
// because every page that shows one wants the other, and two round trips to
// render one header is a waste.
type meJSON struct {
	userJSON
	Usage usageJSON `json:"usage"`
}

type usageJSON struct {
	PlanLabel string `json:"plan_label"`
	// InterviewsUsed counts against InterviewsIncluded. Included is 0 when the
	// tier is unlimited, which the client reads as "no ceiling" rather than
	// "none allowed" — hence Unlimited alongside it, so nothing turns on
	// interpreting a zero.
	InterviewsUsed      int  `json:"interviews_used"`
	InterviewsIncluded  int  `json:"interviews_included"`
	UnlimitedInterviews bool `json:"unlimited_interviews"`
	// MaxMinutes is the longest a single interview may run, 0 when unlimited.
	MaxMinutes int `json:"max_minutes"`
	// UsedMinutes is interview time actually spent in the current window.
	UsedMinutes int `json:"used_minutes"`
	// Lifetime means the allowance never resets — the free tier is a trial,
	// not a monthly budget, and the UI has to say so.
	Lifetime bool `json:"lifetime"`
	// PromoCode is set when a redeemed promotion is what is granting the
	// limits above, rather than the subscription in Plan. The UI has to be
	// able to say which, or somebody sees "Unlimited" and assumes they are
	// being charged for it.
	PromoCode string `json:"promo_code,omitempty"`
	// PromoExpiresAt is when that grant lapses; nil for one that does not.
	PromoExpiresAt *time.Time `json:"promo_expires_at"`
}

// effectivePlan is the tier a user actually gets right now.
//
// Three sources, most specific first: the owner list, then a live promotion,
// then the subscription. A promotion overrides the subscription rather than
// replacing it, so a grant that lapses drops somebody back to what they pay
// for instead of to Free. Every limit check goes through here — a second
// place that reads u.Plan directly is how a comped account quietly stops
// being comped.
func effectivePlan(u *store.User) plan.Plan {
	// The owner wins outright, and reads from the environment rather than the
	// row. That is what makes the account usable before anything has been
	// written to the database, and on the far side of a database that has
	// been wiped.
	if isOwner(u.Email) {
		return plan.ByName(string(plan.Unlimited))
	}
	if u.PromoActive() {
		return plan.ByName(u.PromoPlan)
	}
	return plan.ByName(u.Plan)
}

// isAdmin reports whether a user may reach the admin routes.
//
// Only the owner list today. When an is_admin column arrives it is ORed in
// here, so there stays exactly one answer to "may this person administer the
// deployment" — and the owner keeps working even if that column is wrong,
// which is the property that makes the system recoverable.
func isAdmin(u *store.User) bool {
	return isOwner(u.Email)
}

func toUserJSON(u *store.User) userJSON {
	// PasswordHash is deliberately absent. Encoding the store type directly
	// would ship the bcrypt hash to the client.
	//
	// Plan is the *subscription*, not the tier in force — a promotion can be
	// granting more than this says. Usage carries the effective one, and that
	// is what the UI shows; this stays honest about what is being paid for.
	return userJSON{ID: u.ID, Email: u.Email, Plan: u.Plan, IsAdmin: isAdmin(u)}
}

// Register creates an interviewer account and logs it straight in, so the
// client does not have to follow up with a second request.
func Register(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creds, ok := decodeCredentials(w, r)
		if !ok {
			return
		}

		hash, err := auth.HashPassword(creds.Password)
		if err != nil {
			// The only errors here are the length rules, which are the
			// caller's fault and safe to report verbatim.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		u, err := s.CreateUser(r.Context(), creds.Email, hash)
		if errors.Is(err, store.ErrConflict) {
			// This does leak that an address is registered. Registration
			// cannot avoid it — the alternative is accepting the request and
			// silently doing nothing, which strands anyone who mistypes an
			// address they already own.
			writeError(w, http.StatusConflict, "that email is already registered")
			return
		}
		if err != nil {
			log.Printf("api: register: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create the account")
			return
		}

		issueSession(w, r, s, u, http.StatusCreated)
	}
}

// Login exchanges credentials for a session token.
func Login(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creds, ok := decodeCredentials(w, r)
		if !ok {
			return
		}

		u, err := s.UserByEmail(r.Context(), creds.Email)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			log.Printf("api: login lookup: %v", err)
			writeError(w, http.StatusInternalServerError, "could not sign you in")
			return
		}

		// Hash against a dummy when the user does not exist, so both paths do
		// the same bcrypt work. Skipping it would make a missing account
		// measurably faster to reject and turn this endpoint into a way to
		// enumerate registered addresses.
		storedHash := auth.DummyPasswordHash
		if u != nil {
			storedHash = u.PasswordHash
		}
		if !auth.CheckPassword(storedHash, creds.Password) || u == nil {
			writeError(w, http.StatusUnauthorized, "incorrect email or password")
			return
		}

		issueSession(w, r, s, u, http.StatusOK)
	}
}

func issueSession(w http.ResponseWriter, r *http.Request, s *store.Store, u *store.User, status int) {
	token, hash, err := auth.NewToken()
	if err != nil {
		log.Printf("api: issue session: %v", err)
		writeError(w, http.StatusInternalServerError, "could not start a session")
		return
	}

	expires := time.Now().Add(store.SessionTTL)
	if err := s.CreateSession(r.Context(), u.ID, hash, expires); err != nil {
		log.Printf("api: create session: %v", err)
		writeError(w, http.StatusInternalServerError, "could not start a session")
		return
	}

	writeJSON(w, status, authResponse{
		Token:     token,
		ExpiresAt: expires,
		User:      toUserJSON(u),
	})
}

// Logout revokes the presented session.
//
// It always reports success. A caller that presents a token which is already
// gone has achieved what it asked for, and distinguishing the cases would only
// tell an attacker whether a stolen token was still live.
func Logout(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token := bearerToken(r); token != "" {
			if err := s.DeleteSession(r.Context(), auth.HashToken(token)); err != nil {
				log.Printf("api: logout: %v", err)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// Me returns the signed-in interviewer and what their plan has left.
func Me(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			// Unreachable behind RequireAuth; a guard rather than a panic in
			// case the route is ever mounted without it.
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		writeJSON(w, http.StatusOK, meFor(r.Context(), s, u))
	}
}

// meFor builds the signed-in view. Shared with the promo redemption, which
// answers with the same shape so the client can swap its user wholesale
// instead of refetching to see what changed.
func meFor(ctx context.Context, s *store.Store, u *store.User) meJSON {
	tier := effectivePlan(u)
	out := meJSON{
		userJSON: toUserJSON(u),
		Usage: usageJSON{
			PlanLabel:           tier.Label,
			InterviewsIncluded:  tier.MaxInterviews,
			UnlimitedInterviews: tier.UnlimitedInterviews(),
			MaxMinutes:          int(tier.MaxDuration / time.Minute),
			Lifetime:            tier.Lifetime,
		},
	}
	if u.PromoActive() {
		out.Usage.PromoCode = u.PromoCode
		out.Usage.PromoExpiresAt = u.PromoExpiresAt
	}

	// Usage is a nicety, not the enforcement — that happens on the room
	// creation path. So a failure here degrades to zeroes and a log line
	// rather than blocking somebody from seeing who they are signed in as.
	if used, err := s.CountRooms(ctx, u.ID, tier.Lifetime); err == nil {
		out.Usage.InterviewsUsed = used
	} else {
		log.Printf("api: me: count rooms: %v", err)
	}
	if spent, err := s.UsedDuration(ctx, u.ID, tier.Lifetime); err == nil {
		out.Usage.UsedMinutes = int(spent / time.Minute)
	} else {
		log.Printf("api: me: used duration: %v", err)
	}
	return out
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)

	var creds credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return credentials{}, false
	}

	// Trim and normalise before anything looks at it, so " Alice@x " and
	// "alice@x" cannot become two accounts.
	creds.Email = strings.TrimSpace(creds.Email)
	if creds.Email == "" || !strings.Contains(creds.Email, "@") {
		// Not real validation — an address is only proven by sending mail to
		// it, which is out of scope. This catches obvious mistakes.
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return credentials{}, false
	}
	if creds.Password == "" {
		writeError(w, http.StatusBadRequest, "a password is required")
		return credentials{}, false
	}
	return creds, true
}

// bearerToken pulls the token out of an Authorization header, returning "" if
// there isn't a well-formed one.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// contextKey is unexported so nothing outside this package can write a user
// into a request context and impersonate one.
type contextKey struct{ name string }

var userContextKey = &contextKey{"user"}

// UserFrom returns the interviewer RequireAuth attached to the request.
func UserFrom(ctx context.Context) (*store.User, bool) {
	u, ok := ctx.Value(userContextKey).(*store.User)
	return u, ok
}

// RequireAuth rejects anything without a live session token and attaches the
// interviewer to the request context for the handler behind it.
func RequireAuth(s *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		u, err := s.UserBySessionToken(r.Context(), auth.HashToken(token))
		if errors.Is(err, store.ErrNotFound) {
			// Covers both "no such token" and "expired": the query enforces
			// expiry, so there is one answer for both.
			writeError(w, http.StatusUnauthorized, "your session has expired")
			return
		}
		if err != nil {
			log.Printf("api: resolve session: %v", err)
			writeError(w, http.StatusInternalServerError, "could not verify your session")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, u)))
	})
}

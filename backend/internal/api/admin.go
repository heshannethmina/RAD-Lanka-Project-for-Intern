package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/plan"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// Listing caps. These are operator tools for a product with a pilot's worth
// of accounts; when the caps stop being enough the answer is pagination, not
// a bigger number.
const (
	adminUserLimit  = 500
	adminPromoLimit = 200
)

// maxAdminBodyBytes bounds an admin payload: a code and a few fields.
const maxAdminBodyBytes = 4 * 1024

// RequireAdmin gates the admin routes. Mount it *inside* RequireAuth, which
// is what puts the user in the context for it to read.
//
// It answers 404 rather than 403, matching the room routes: a 403 confirms
// the endpoint is real and that somebody, somewhere, gets through it. To an
// account that may not administer this deployment, the admin API simply does
// not exist.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok || !isAdmin(u) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type promoJSON struct {
	Code           string     `json:"code"`
	Plan           string     `json:"plan"`
	MaxRedemptions int        `json:"max_redemptions"`
	Redemptions    int        `json:"redemptions"`
	ExpiresAt      *time.Time `json:"expires_at"`
	GrantDays      int        `json:"grant_days"`
	Note           string     `json:"note"`
	CreatedAt      time.Time  `json:"created_at"`
	Redeemers      []string   `json:"redeemers"`
}

func toPromoJSON(c *store.PromoCodeWithUse) promoJSON {
	redeemers := c.Redeemers
	if redeemers == nil {
		// A nil slice encodes as null; the client wants a list it can map
		// over without a guard.
		redeemers = []string{}
	}
	return promoJSON{
		Code: c.Code, Plan: c.Plan,
		MaxRedemptions: c.MaxRedemptions, Redemptions: c.Redemptions,
		ExpiresAt: c.ExpiresAt, GrantDays: c.GrantDays,
		Note: c.Note, CreatedAt: c.CreatedAt, Redeemers: redeemers,
	}
}

// ListPromoCodes returns every coupon with who has claimed it.
func ListPromoCodes(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		codes, err := s.PromoCodes(r.Context(), adminPromoLimit)
		if err != nil {
			log.Printf("api: admin list promo: %v", err)
			writeError(w, http.StatusInternalServerError, "could not list the codes")
			return
		}
		out := make([]promoJSON, 0, len(codes))
		for i := range codes {
			out = append(out, toPromoJSON(&codes[i]))
		}
		writeJSON(w, http.StatusOK, map[string]any{"codes": out})
	}
}

// CreatePromoCode issues a coupon.
func CreatePromoCode(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code           string     `json:"code"`
			Plan           string     `json:"plan"`
			MaxRedemptions int        `json:"max_redemptions"`
			GrantDays      int        `json:"grant_days"`
			ExpiresAt      *time.Time `json:"expires_at"`
			Note           string     `json:"note"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}

		code := store.NormalizePromoCode(req.Code)
		// The same bounds the CHECK constraint enforces, reported as a
		// sentence rather than as a constraint violation.
		if len(code) < 4 || len(code) > 64 {
			writeError(w, http.StatusBadRequest,
				"a code must be between 4 and 64 characters")
			return
		}
		if req.Plan == "" {
			req.Plan = string(plan.Unlimited)
		}
		// Vetted here as well as at redemption. Redemption has to check
		// anyway, because rows can still be written by hand — but refusing a
		// bad tier where somebody types it turns a mystery 500 later into an
		// error message now.
		if !plan.Grantable(req.Plan) {
			writeError(w, http.StatusBadRequest, "unknown plan: "+req.Plan)
			return
		}
		if req.MaxRedemptions < 0 || req.GrantDays < 0 {
			writeError(w, http.StatusBadRequest, "counts cannot be negative")
			return
		}

		created, err := s.CreatePromoCode(r.Context(), store.PromoCode{
			Code: code, Plan: req.Plan,
			MaxRedemptions: req.MaxRedemptions, GrantDays: req.GrantDays,
			ExpiresAt: req.ExpiresAt, Note: req.Note,
		})
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "that code already exists")
			return
		}
		if err != nil {
			log.Printf("api: admin create promo: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create the code")
			return
		}

		u, _ := UserFrom(r.Context())
		log.Printf("api: admin %s created promo %q (%s)", u.Email, created.Code, created.Plan)
		writeJSON(w, http.StatusCreated, toPromoJSON(&store.PromoCodeWithUse{PromoCode: *created}))
	}
}

// DeletePromoCode revokes a coupon.
//
// A "grants=revoke" query also strips the grants it handed out. Off by
// default and opt-in on purpose: stopping new claims and taking back access
// somebody is already relying on are different decisions, and the
// destructive one should have to be asked for.
func DeletePromoCode(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		revoked := int64(0)

		if r.URL.Query().Get("grants") == "revoke" {
			n, err := s.ClearPromoGrants(r.Context(), code)
			if err != nil {
				log.Printf("api: admin clear grants: %v", err)
				writeError(w, http.StatusInternalServerError, "could not revoke the grants")
				return
			}
			revoked = n
		}

		err := s.DeletePromoCode(r.Context(), code)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no such code")
			return
		}
		if err != nil {
			log.Printf("api: admin delete promo: %v", err)
			writeError(w, http.StatusInternalServerError, "could not delete the code")
			return
		}

		u, _ := UserFrom(r.Context())
		log.Printf("api: admin %s deleted promo %q (grants revoked: %d)", u.Email, code, revoked)
		writeJSON(w, http.StatusOK, map[string]any{"grants_revoked": revoked})
	}
}

type adminUserJSON struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	// Plan is the subscription; EffectivePlan is what they actually get,
	// which a promotion or the owner list can raise above it.
	Plan           string     `json:"plan"`
	EffectivePlan  string     `json:"effective_plan"`
	IsAdmin        bool       `json:"is_admin"`
	PromoCode      string     `json:"promo_code,omitempty"`
	PromoExpiresAt *time.Time `json:"promo_expires_at"`
	Rooms          int        `json:"rooms"`
	Minutes        int        `json:"minutes"`
	CreatedAt      time.Time  `json:"created_at"`
}

func toAdminUserJSON(u *store.UserWithUse) adminUserJSON {
	// PasswordHash is absent, as everywhere else. An admin has no more
	// business reading password hashes than anybody else does.
	out := adminUserJSON{
		ID: u.ID, Email: u.Email, Plan: u.Plan,
		EffectivePlan: string(effectivePlan(&u.User).Name),
		IsAdmin:       isAdmin(&u.User),
		Rooms:         u.Rooms, Minutes: u.Minutes, CreatedAt: u.CreatedAt,
	}
	if u.PromoActive() {
		out.PromoCode = u.PromoCode
		out.PromoExpiresAt = u.PromoExpiresAt
	}
	return out
}

// ListUsers returns the accounts on this deployment with what they have used.
func ListUsers(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := s.Users(r.Context(), adminUserLimit)
		if err != nil {
			log.Printf("api: admin list users: %v", err)
			writeError(w, http.StatusInternalServerError, "could not list the accounts")
			return
		}
		out := make([]adminUserJSON, 0, len(users))
		for i := range users {
			out = append(out, toAdminUserJSON(&users[i]))
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": out})
	}
}

// SetUserPlan moves an account between subscription tiers.
func SetUserPlan(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad account id")
			return
		}

		var req struct {
			Plan string `json:"plan"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}
		// Free is accepted here although it is not Grantable. A promotion
		// granting Free would be a no-op, but moving somebody back down to
		// Free is a real thing an operator has to be able to do.
		if !plan.Grantable(req.Plan) && req.Plan != string(plan.Free) {
			writeError(w, http.StatusBadRequest, "unknown plan: "+req.Plan)
			return
		}

		updated, err := s.SetUserPlan(r.Context(), id, req.Plan)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no such account")
			return
		}
		if err != nil {
			log.Printf("api: admin set plan: %v", err)
			writeError(w, http.StatusInternalServerError, "could not change the plan")
			return
		}

		u, _ := UserFrom(r.Context())
		log.Printf("api: admin %s set account %d to %q", u.Email, id, req.Plan)
		writeJSON(w, http.StatusOK, toAdminUserJSON(&store.UserWithUse{User: *updated}))
	}
}

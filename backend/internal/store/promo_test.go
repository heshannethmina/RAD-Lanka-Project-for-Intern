package store_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/auth"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// uniqueCode keeps tests independent without truncating tables, the same way
// uniqueEmail does.
func uniqueCode(t *testing.T) string {
	t.Helper()
	tok, _, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	// Base64url can contain - and _, both fine for the CHECK, but the code
	// must be upper case to satisfy it.
	return "T" + strings.ToUpper(strings.NewReplacer("-", "X", "_", "Y").Replace(tok[:12]))
}

func newPromo(t *testing.T, s *store.Store, c store.PromoCode) *store.PromoCode {
	t.Helper()
	if c.Code == "" {
		c.Code = uniqueCode(t)
	}
	if c.Plan == "" {
		c.Plan = "unlimited"
	}
	p, err := s.CreatePromoCode(context.Background(), c)
	if err != nil {
		t.Fatalf("create promo: %v", err)
	}
	return p
}

func TestNormalizePromoCodeFixesWhatPeopleActuallyType(t *testing.T) {
	// A code is read off a slide and typed by hand, so it arrives shifted and
	// spaced. All of these have to be the same coupon.
	for _, in := range []string{
		"SYNCR-PILOT", "syncr-pilot", " Syncr-Pilot ", "SYNCR - PILOT", "syncr-pilot\n",
	} {
		if got := store.NormalizePromoCode(in); got != "SYNCR-PILOT" {
			t.Errorf("NormalizePromoCode(%q) = %q, want %q", in, got, "SYNCR-PILOT")
		}
	}
}

func TestRedeemGrantsThePlanAndOverridesTheSubscription(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	p := newPromo(t, s, store.PromoCode{Plan: "unlimited"})

	// Typed back in the sloppy form, to prove normalisation is on the redeem
	// path and not only on the write path.
	grant, err := s.RedeemPromoCode(ctx, " "+strings.ToLower(p.Code)+" ", u.ID, nil)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if grant.Plan != "unlimited" {
		t.Fatalf("granted plan %q, want unlimited", grant.Plan)
	}
	if grant.ExpiresAt != nil {
		t.Fatalf("grant with no grant_days expires at %v, want never", grant.ExpiresAt)
	}

	got, err := s.UserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("user by id: %v", err)
	}
	if !got.PromoActive() {
		t.Fatal("promotion is not active on the user after redeeming")
	}
	if got.PromoPlan != "unlimited" || got.PromoCode != p.Code {
		t.Fatalf("user has promo %q/%q, want %q/unlimited", got.PromoCode, got.PromoPlan, p.Code)
	}
	// The subscription is untouched: a promotion is an override, so when it
	// lapses somebody falls back to what they pay for rather than to Free.
	if got.Plan != "free" {
		t.Fatalf("subscription became %q, want it left as free", got.Plan)
	}
}

// TestSameCodeCannotBeRedeemedTwice pins the reason promo_redemptions exists:
// without it a dated grant could be re-redeemed every morning to push its
// expiry out forever, turning a 30-day trial into a permanent one.
func TestSameCodeCannotBeRedeemedTwice(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	p := newPromo(t, s, store.PromoCode{GrantDays: 30})

	first, err := s.RedeemPromoCode(ctx, p.Code, u.ID, nil)
	if err != nil {
		t.Fatalf("first redeem: %v", err)
	}
	if first.ExpiresAt == nil {
		t.Fatal("grant_days=30 produced a grant that never expires")
	}

	if _, err := s.RedeemPromoCode(ctx, p.Code, u.ID, nil); !errors.Is(err, store.ErrPromoAlreadyRedeemed) {
		t.Fatalf("second redeem: %v, want ErrPromoAlreadyRedeemed", err)
	}

	// And the expiry did not move.
	got, err := s.UserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("user by id: %v", err)
	}
	if got.PromoExpiresAt == nil || !got.PromoExpiresAt.Equal(*first.ExpiresAt) {
		t.Fatalf("expiry moved from %v to %v", first.ExpiresAt, got.PromoExpiresAt)
	}
}

func TestExpiredCodeIsRefused(t *testing.T) {
	s := open(t)
	past := time.Now().Add(-time.Hour)
	p := newPromo(t, s, store.PromoCode{ExpiresAt: &past})

	_, err := s.RedeemPromoCode(context.Background(), p.Code, newUser(t, s).ID, nil)
	if !errors.Is(err, store.ErrPromoExpired) {
		t.Fatalf("redeem: %v, want ErrPromoExpired", err)
	}
}

func TestUnknownCodeIsNotFound(t *testing.T) {
	s := open(t)
	_, err := s.RedeemPromoCode(context.Background(), uniqueCode(t), newUser(t, s).ID, nil)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("redeem: %v, want ErrNotFound", err)
	}
}

// TestRedemptionCeilingHoldsUnderConcurrentRedeems is the one worth having.
// Every interesting failure on this path is a race — the ceiling has to be a
// real ceiling, not a suggestion, when several people redeem the last seat at
// the same moment. SELECT ... FOR UPDATE is what makes it one.
func TestRedemptionCeilingHoldsUnderConcurrentRedeems(t *testing.T) {
	s := open(t)
	ctx := context.Background()

	const seats = 3
	const racers = 10
	p := newPromo(t, s, store.PromoCode{MaxRedemptions: seats})

	users := make([]*store.User, racers)
	for i := range users {
		users[i] = newUser(t, s)
	}

	var wg sync.WaitGroup
	granted := make(chan struct{}, racers)
	for _, u := range users {
		wg.Add(1)
		go func(u *store.User) {
			defer wg.Done()
			switch _, err := s.RedeemPromoCode(ctx, p.Code, u.ID, nil); {
			case err == nil:
				granted <- struct{}{}
			case errors.Is(err, store.ErrPromoExhausted):
				// The expected outcome for everyone past the ceiling.
			default:
				t.Errorf("redeem: %v, want nil or ErrPromoExhausted", err)
			}
		}(u)
	}
	wg.Wait()
	close(granted)

	if got := len(granted); got != seats {
		t.Fatalf("%d redemptions succeeded, want exactly %d", got, seats)
	}

	after, err := s.PromoCodeByCode(ctx, p.Code)
	if err != nil {
		t.Fatalf("promo by code: %v", err)
	}
	if after.Redemptions != seats {
		t.Fatalf("counter says %d, want %d", after.Redemptions, seats)
	}
}

func TestLapsedGrantStopsApplying(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)

	// One day, then wound back past its expiry. Setting promo_expires_at
	// directly is the only way to test the read-path check without waiting.
	p := newPromo(t, s, store.PromoCode{GrantDays: 1})
	if _, err := s.RedeemPromoCode(ctx, p.Code, u.ID, nil); err != nil {
		t.Fatalf("redeem: %v", err)
	}

	lapsed := time.Now().Add(-time.Minute)
	got, err := s.UserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("user by id: %v", err)
	}
	got.PromoExpiresAt = &lapsed
	if got.PromoActive() {
		t.Fatal("a grant whose expiry has passed still reports active")
	}
	// The plan name is still on the row — sweeping it would make every read a
	// write — so PromoActive is the only thing standing between a stale grant
	// and unmetered use. That is exactly why it is tested.
	if got.PromoPlan == "" {
		t.Fatal("promo_plan was cleared; the row is meant to be left in place")
	}
}

func TestDuplicatePromoCodeIsRejected(t *testing.T) {
	s := open(t)
	p := newPromo(t, s, store.PromoCode{})

	_, err := s.CreatePromoCode(context.Background(), store.PromoCode{Code: p.Code, Plan: "pro"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate code: %v, want ErrConflict", err)
	}
}

// TestMisconfiguredCodeCostsTheRedeemerNothing is here because an end-to-end
// run caught the opposite. The plan column is typed in by hand, so a typo is a
// question of when — and vetting it after the transaction committed meant a
// bad code burned the one redemption the account would ever get, overwrote any
// grant already on it, and then answered 500. The check belongs inside the
// transaction, before any write.
func TestMisconfiguredCodeCostsTheRedeemerNothing(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)

	good := newPromo(t, s, store.PromoCode{Plan: "unlimited"})
	if _, err := s.RedeemPromoCode(ctx, good.Code, u.ID, grantable); err != nil {
		t.Fatalf("redeem good code: %v", err)
	}

	bad := newPromo(t, s, store.PromoCode{Plan: "gold"})
	if _, err := s.RedeemPromoCode(ctx, bad.Code, u.ID, grantable); !errors.Is(err, store.ErrPromoMisconfigured) {
		t.Fatalf("redeem bad code: %v, want ErrPromoMisconfigured", err)
	}

	// The grant they already had must survive.
	got, err := s.UserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("user by id: %v", err)
	}
	if got.PromoPlan != "unlimited" || got.PromoCode != good.Code {
		t.Fatalf("grant became %q/%q, want %q/unlimited", got.PromoCode, got.PromoPlan, good.Code)
	}

	// Nothing was consumed, so the code still works once somebody fixes the
	// row — which is the whole point of rolling back.
	after, err := s.PromoCodeByCode(ctx, bad.Code)
	if err != nil {
		t.Fatalf("promo by code: %v", err)
	}
	if after.Redemptions != 0 {
		t.Fatalf("bad code counted %d redemptions, want 0", after.Redemptions)
	}
	if _, err := s.RedeemPromoCode(ctx, bad.Code, u.ID, nil); err != nil {
		t.Fatalf("bad code was consumed: re-redeeming gave %v", err)
	}
}

// grantable stands in for plan.Grantable. Spelled out rather than imported so
// the store's tests stay independent of what the tiers happen to be called.
func grantable(name string) bool {
	return name == "unlimited" || name == "pro" || name == "enterprise"
}

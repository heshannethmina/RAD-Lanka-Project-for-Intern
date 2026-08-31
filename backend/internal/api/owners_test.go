package api

import (
	"testing"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/plan"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// setOwners installs a list for one test and puts it back afterwards, so
// tests do not leak configuration into each other.
func setOwners(t *testing.T, list string) {
	t.Helper()
	previous := owners
	t.Cleanup(func() { owners = previous })
	SetOwners(list)
}

func TestNoOwnersConfiguredMeansNobodyIsPrivileged(t *testing.T) {
	setOwners(t, "")
	// A deployment that forgets the variable must have no privileged account
	// rather than an accidental one.
	for _, email := range []string{"", "anyone@example.com", "admin@example.com"} {
		u := &store.User{Email: email, Plan: "free"}
		if isOwner(email) || isAdmin(u) {
			t.Errorf("%q is privileged with no owner list configured", email)
		}
		if got := effectivePlan(u); got.Name != plan.Free {
			t.Errorf("%q resolved to %q, want free", email, got.Name)
		}
	}
}

func TestOwnerMatchIsCaseAndSpaceInsensitive(t *testing.T) {
	setOwners(t, "  Owner@Example.COM , second@example.com ")

	// The users table enforces uniqueness on lower(email), so Owner@ and
	// owner@ are one account and must resolve to one owner.
	for _, email := range []string{"owner@example.com", "OWNER@EXAMPLE.COM", " Owner@Example.com "} {
		if !isOwner(email) {
			t.Errorf("isOwner(%q) = false, want true", email)
		}
	}
	if !isOwner("second@example.com") {
		t.Error("second address in the list is not an owner")
	}
	if isOwner("owner@example.com.evil.test") {
		t.Error("a superstring of an owner address matched")
	}
	if isOwner("") {
		t.Error("the empty address matched; a blank entry must not grant anything")
	}
}

// TestOwnerNeedsNothingInTheDatabase is the property the whole mechanism
// exists for. An admin column can only be set by somebody who can already
// write to the database, which on a host with no shell and no SQL console is
// a cycle with no way in. Reading the environment breaks it — and survives
// the database being wiped, which the free Postgres tier does on a timer.
func TestOwnerNeedsNothingInTheDatabase(t *testing.T) {
	setOwners(t, "owner@example.com")

	// A brand-new row: free plan, no promotion, nothing written to it.
	u := &store.User{Email: "owner@example.com", Plan: "free"}

	if !isAdmin(u) {
		t.Fatal("owner is not an admin")
	}
	tier := effectivePlan(u)
	if tier.Name != plan.Unlimited {
		t.Fatalf("owner resolved to %q, want unlimited", tier.Name)
	}
	if !tier.UnlimitedInterviews() || !tier.UnlimitedDuration() {
		t.Fatalf("owner tier caps interviews=%d duration=%v", tier.MaxInterviews, tier.MaxDuration)
	}
}

// TestOwnerOutranksALapsedPromotion pins the precedence order. An owner whose
// promo grant has expired must not silently fall back to Free — that would
// lock the operator out of their own deployment at the moment the grant
// lapsed, which is exactly when they need to get in and fix it.
func TestOwnerOutranksALapsedPromotion(t *testing.T) {
	setOwners(t, "owner@example.com")
	lapsed := time.Now().Add(-time.Hour)

	u := &store.User{
		Email:          "owner@example.com",
		Plan:           "free",
		PromoPlan:      "pro",
		PromoExpiresAt: &lapsed,
	}
	if got := effectivePlan(u); got.Name != plan.Unlimited {
		t.Fatalf("owner with a lapsed promotion resolved to %q, want unlimited", got.Name)
	}

	// And a non-owner in the same state does fall back to the subscription.
	other := &store.User{
		Email:          "someone@example.com",
		Plan:           "free",
		PromoPlan:      "pro",
		PromoExpiresAt: &lapsed,
	}
	if got := effectivePlan(other); got.Name != plan.Free {
		t.Fatalf("non-owner with a lapsed promotion resolved to %q, want free", got.Name)
	}
}

func TestNonOwnerKeepsTheirOwnPlan(t *testing.T) {
	setOwners(t, "owner@example.com")

	paying := &store.User{Email: "customer@example.com", Plan: "pro"}
	if got := effectivePlan(paying); got.Name != plan.Pro {
		t.Fatalf("paying customer resolved to %q, want pro", got.Name)
	}
	if isAdmin(paying) {
		t.Fatal("a paying customer reached admin")
	}
}

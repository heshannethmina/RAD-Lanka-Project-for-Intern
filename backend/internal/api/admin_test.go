package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// withUser mounts a handler behind RequireAdmin with u already in the
// context, standing in for what RequireAuth does. The gate is the thing under
// test, not the session lookup, which has its own coverage.
func withUser(u *store.User) *httptest.ResponseRecorder {
	reached := false
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	if u != nil {
		r = r.WithContext(context.WithValue(r.Context(), userContextKey, u))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if reached && w.Code != http.StatusOK {
		panic("handler ran but the recorder disagrees")
	}
	return w
}

// TestAdminGateAnswers404ForEveryoneElse pins the shape of the refusal as
// well as the refusal. A 403 would confirm the endpoint is real and that
// somebody gets through it, which is exactly what an attacker probing for an
// admin API wants to learn.
func TestAdminGateAnswers404ForEveryoneElse(t *testing.T) {
	setOwners(t, "owner@example.com")

	cases := []struct {
		name string
		user *store.User
	}{
		{"no user in context at all", nil},
		{"an ordinary account", &store.User{Email: "someone@example.com", Plan: "free"}},
		{"a paying account", &store.User{Email: "customer@example.com", Plan: "pro"}},
		{"an account with an unlimited promotion", &store.User{
			Email: "comped@example.com", Plan: "free", PromoPlan: "unlimited",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := withUser(c.user).Code; got != http.StatusNotFound {
				t.Fatalf("status %d, want 404", got)
			}
		})
	}
}

// A comped account is the interesting one above: an unlimited *plan* must not
// imply administrative access. They are different questions and the code has
// to keep them apart.
func TestUnlimitedPlanDoesNotImplyAdmin(t *testing.T) {
	setOwners(t, "owner@example.com")
	u := &store.User{Email: "comped@example.com", Plan: "unlimited"}

	if isAdmin(u) {
		t.Fatal("an account on the unlimited plan reached admin")
	}
	if withUser(u).Code != http.StatusNotFound {
		t.Fatal("an account on the unlimited plan passed the admin gate")
	}
}

func TestOwnerPassesTheAdminGate(t *testing.T) {
	setOwners(t, "owner@example.com")

	if got := withUser(&store.User{Email: "owner@example.com", Plan: "free"}).Code; got != http.StatusOK {
		t.Fatalf("owner got status %d, want 200", got)
	}
	// Case-insensitively, because the users table treats the addresses as one
	// account and the gate must not disagree with it.
	if got := withUser(&store.User{Email: "Owner@Example.COM", Plan: "free"}).Code; got != http.StatusOK {
		t.Fatalf("owner in a different case got status %d, want 200", got)
	}
}

// TestNoOwnersMeansNoAdminAtAll is the fail-closed property. A deployment
// that forgets OWNER_EMAILS must have an admin API that nobody can reach,
// rather than one everybody can.
func TestNoOwnersMeansNoAdminAtAll(t *testing.T) {
	setOwners(t, "")
	for _, email := range []string{"owner@example.com", "anyone@example.com", ""} {
		u := &store.User{Email: email, Plan: "unlimited"}
		if withUser(u).Code != http.StatusNotFound {
			t.Fatalf("%q reached admin with no owner list configured", email)
		}
	}
}

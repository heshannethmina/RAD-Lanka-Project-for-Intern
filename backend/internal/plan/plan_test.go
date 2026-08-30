package plan_test

import (
	"testing"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/plan"
)

// TestUnknownPlanFallsBackToFree is the fail-closed property: a row written by
// a newer build, or edited by hand with a typo, must under-serve rather than
// silently hand out unlimited use.
func TestUnknownPlanFallsBackToFree(t *testing.T) {
	for _, name := range []string{"", "gold", "UNLIMITED", "unlimted"} {
		if got := plan.ByName(name); got.Name != plan.Free {
			t.Errorf("ByName(%q) = %q, want free", name, got.Name)
		}
	}
}

func TestUnlimitedGrantHasNoCeilings(t *testing.T) {
	p := plan.ByName(string(plan.Unlimited))
	if !p.UnlimitedInterviews() || !p.UnlimitedDuration() {
		t.Fatalf("unlimited caps interviews=%v duration=%v", p.MaxInterviews, p.MaxDuration)
	}
	// A promoted account asking for a four-hour interview gets four hours.
	if got := p.ClampDuration(4 * time.Hour); got != 4*time.Hour {
		t.Fatalf("ClampDuration(4h) = %v, want 4h", got)
	}
}

// TestOnlyRealTiersAreGrantable guards the hand-written promo_codes rows: a
// typo in the plan column would otherwise fall through ByName to Free, and the
// person redeeming would be congratulated on an upgrade that did nothing.
func TestOnlyRealTiersAreGrantable(t *testing.T) {
	for _, name := range []string{"unlimited", "pro", "enterprise"} {
		if !plan.Grantable(name) {
			t.Errorf("Grantable(%q) = false, want true", name)
		}
	}
	// Free is not grantable: a code that gives somebody Free is a code that
	// does nothing, and shipping that silently is the failure mode above.
	for _, name := range []string{"free", "", "Pro", "gold"} {
		if plan.Grantable(name) {
			t.Errorf("Grantable(%q) = true, want false", name)
		}
	}
}

func TestFreeClampsToItsCeiling(t *testing.T) {
	free := plan.ByName("free")
	if got := free.ClampDuration(time.Hour); got != 10*time.Minute {
		t.Fatalf("ClampDuration(1h) on free = %v, want 10m", got)
	}
	// Zero means "whatever the plan allows" rather than "no time at all".
	if got := free.ClampDuration(0); got != 10*time.Minute {
		t.Fatalf("ClampDuration(0) on free = %v, want 10m", got)
	}
}

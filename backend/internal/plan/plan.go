// Package plan holds what each pricing tier allows.
//
// One place, so the marketing page, the room-creation check and the timer
// cannot drift apart. If the site says 30 interviews and this says 20, the
// site is wrong and somebody finds out by being refused.
package plan

import "time"

// Name identifies a tier. Stored on the user row as plain text rather than an
// enum type, so adding a tier is a code change and not a migration.
type Name string

const (
	Free       Name = "free"
	Pro        Name = "pro"
	Enterprise Name = "enterprise"
	// Unlimited is what a promotion code grants. It is not for sale, which is
	// why it is not Enterprise: Enterprise is uncapped *and metered*, billed
	// per interview-hour after the fact, and an invoicing run has to be able
	// to tell a paying account from a comped one. Same limits, different
	// meaning.
	Unlimited Name = "unlimited"
)

// Plan is what a tier allows.
//
// Priced on interview time rather than seats: a live room holds a hub
// goroutine and a sandbox, and that is the thing that actually costs us
// something. A seat licence would punish a small team that interviews
// occasionally, which is exactly who this is for.
type Plan struct {
	Name Name
	// Label is what a person sees.
	Label string
	// MaxInterviews is how many rooms may be created. Zero means unlimited.
	MaxInterviews int
	// MaxDuration is the longest a single interview may run.
	MaxDuration time.Duration
	// Lifetime counts interviews for all time rather than per calendar month.
	// The free tier is a trial, not a recurring allowance.
	Lifetime bool
}

var plans = map[Name]Plan{
	Free: {
		Name:          Free,
		Label:         "Free",
		MaxInterviews: 2,
		MaxDuration:   10 * time.Minute,
		Lifetime:      true,
	},
	Pro: {
		Name:          Pro,
		Label:         "Pro",
		MaxInterviews: 30,
		MaxDuration:   time.Hour,
	},
	Enterprise: {
		Name:  Enterprise,
		Label: "Enterprise",
		// Unlimited both ways. Enterprise is metered after the fact rather
		// than capped in advance — that is what pay-as-you-go means.
		MaxInterviews: 0,
		MaxDuration:   0,
	},
	Unlimited: {
		Name:          Unlimited,
		Label:         "Unlimited",
		MaxInterviews: 0,
		MaxDuration:   0,
	},
}

// Grantable reports whether a promotion code may hand out this tier.
//
// Checked when a code is redeemed rather than when it is written, because
// codes are written by hand straight into the table. Without it a typo in the
// plan column would silently fall back to Free through ByName, and the person
// who redeemed it would be told they had been upgraded while nothing changed.
func Grantable(name string) bool {
	switch Name(name) {
	case Unlimited, Pro, Enterprise:
		return true
	}
	return false
}

// ByName returns the plan for a stored value, falling back to Free.
//
// An unrecognised value means the row was written by a newer build or edited
// by hand. Falling back to the most restrictive tier fails closed: the worst
// outcome is somebody being under-served and complaining, rather than an
// unknown string quietly granting unlimited use.
func ByName(name string) Plan {
	if p, ok := plans[Name(name)]; ok {
		return p
	}
	return plans[Free]
}

// UnlimitedInterviews reports whether the tier caps how many may be created.
func (p Plan) UnlimitedInterviews() bool { return p.MaxInterviews == 0 }

// UnlimitedDuration reports whether the tier caps how long one may run.
func (p Plan) UnlimitedDuration() bool { return p.MaxDuration == 0 }

// ClampDuration brings a requested length within what the tier allows.
//
// Clamped rather than rejected: somebody on Free who asks for an hour wants an
// interview, and giving them ten minutes with the limit shown is more useful
// than an error telling them to try again with a smaller number.
func (p Plan) ClampDuration(requested time.Duration) time.Duration {
	if requested <= 0 {
		if p.UnlimitedDuration() {
			return time.Hour
		}
		return p.MaxDuration
	}
	if p.UnlimitedDuration() || requested <= p.MaxDuration {
		return requested
	}
	return p.MaxDuration
}

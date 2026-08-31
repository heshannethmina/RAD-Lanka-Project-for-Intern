package api

import "strings"

// Owner accounts are the people running this deployment.
//
// They are configured by email in the environment rather than by a column,
// which is deliberate and solves a bootstrap problem: an admin flag in the
// database can only be set by somebody who is already able to write to the
// database. On a managed host with no shell and no SQL console that is a
// chicken-and-egg — the operator cannot reach the tool that would let them
// reach the tool. An environment variable is settable from the hosting
// dashboard, so it breaks the cycle.
//
// It also survives the database. The free Postgres tier here expires and
// takes every row with it; an owner defined in the environment is still an
// owner on the other side of that, without anybody having to remember to
// re-run an UPDATE.
//
// The trade is that changing the list needs a redeploy, which is the right
// shape for something that should change roughly never. Everything else —
// comping a customer, issuing a code — belongs in the admin UI, not here.
var owners map[string]bool

// SetOwners configures the owner list from a comma-separated set of email
// addresses. Empty means nobody, which is the correct default: a deployment
// that forgets this variable has no privileged account rather than an
// accidental one.
//
// Call it once at startup, before the server begins serving. It is written
// exactly once and read from every request goroutine after that, the same
// lifecycle as ws.AllowOrigins.
func SetOwners(list string) {
	set := make(map[string]bool)
	for _, email := range strings.Split(list, ",") {
		// Matched case-insensitively, because the users table enforces
		// uniqueness on lower(email) — so Alice@ and alice@ are one account
		// and must be one owner too.
		if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
			set[e] = true
		}
	}
	owners = set
}

// isOwner reports whether an address is on the owner list.
func isOwner(email string) bool {
	if len(owners) == 0 {
		return false
	}
	return owners[strings.ToLower(strings.TrimSpace(email))]
}

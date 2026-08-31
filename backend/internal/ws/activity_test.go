package ws

import (
	"context"
	"strings"
	"testing"
)

// byToken assigns a role from the query string, so one test room can hold both
// an interviewer and a candidate. The real authorizer reads a session or
// invite token; this only has to distinguish the two.
func byToken(_ context.Context, _, token string) (Grant, error) {
	if token == "interviewer" {
		return Grant{Role: RoleInterviewer}, nil
	}
	return Grant{Role: RoleCandidate}, nil
}

// The interviewer is told when the candidate leaves, and the running tally
// comes with it so the UI never has to accumulate anything itself.
func TestCandidateActivityReachesTheInterviewer(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("actroom")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	watcher := dial(t, room)
	next(t, watcher, TypeSnapshot)
	waitPresence(t, watcher, 2)
	waitPresence(t, candidate, 2)

	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityAway})

	got := next(t, watcher, TypeActivity)
	if got.Kind != ActivityAway {
		t.Fatalf("kind = %q", got.Kind)
	}
	if got.Activity == nil {
		t.Fatal("no summary attached")
	}
	if got.Activity.AwayCount != 1 || !got.Activity.Away {
		t.Fatalf("summary = %+v, want one away and currently away", *got.Activity)
	}
}

// visibilitychange and blur both fire for a single tab switch, so a repeated
// "away" must not inflate the count.
func TestDuplicateAwayIsCountedOnce(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("dupeaway")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	watcher := dial(t, room)
	next(t, watcher, TypeSnapshot)
	waitPresence(t, watcher, 2)
	waitPresence(t, candidate, 2)

	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityAway})
	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityAway})
	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityBack, Ms: 4000})

	var last *ActivitySummary
	for range 3 {
		last = next(t, watcher, TypeActivity).Activity
	}
	if last == nil {
		t.Fatal("no summary")
	}
	if last.AwayCount != 1 {
		t.Fatalf("away count = %d, want 1", last.AwayCount)
	}
	if last.AwayMs != 4000 {
		t.Fatalf("away ms = %d, want 4000", last.AwayMs)
	}
	if last.Away {
		t.Fatal("still marked away after coming back")
	}
}

func TestPasteIsCounted(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("pasteroom")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	watcher := dial(t, room)
	next(t, watcher, TypeSnapshot)
	waitPresence(t, watcher, 2)
	waitPresence(t, candidate, 2)

	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityPaste, Lines: 12})

	got := next(t, watcher, TypeActivity)
	if got.Lines != 12 {
		t.Fatalf("lines = %d, want 12", got.Lines)
	}
	if got.Activity.PasteCount != 1 {
		t.Fatalf("paste count = %d, want 1", got.Activity.PasteCount)
	}
}

// An interviewer's own tab switching is their business, and relaying it would
// put noise in front of the person meant to be reading the signal.
func TestInterviewerActivityIsNotRelayed(t *testing.T) {
	url := serverAs(t, AllowAll) // everyone is an interviewer here
	room := url("noiseroom")

	a := dial(t, room)
	next(t, a, TypeSnapshot)
	waitPresence(t, a, 1)

	b := dial(t, room)
	next(t, b, TypeSnapshot)
	waitPresence(t, b, 2)
	waitPresence(t, a, 2)

	send(t, a, Message{Type: TypeActivity, Kind: ActivityAway})
	// An edit afterwards must arrive with no activity frame before it, which
	// proves the activity was dropped rather than merely delayed.
	send(t, a, Message{Type: TypeEdit, Text: "still here"})

	got := next(t, b, TypeEdit)
	if got.Text != "still here" {
		t.Fatalf("got %q", got.Text)
	}
}

// The tally lives in the hub, so an interviewer who reloads mid-interview does
// not come back to a blank slate.
func TestTallySurvivesAReload(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("reloadroom")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	// A witness already in the room, to order the send against the join.
	//
	// Writing to one socket and immediately dialling another guarantees
	// nothing: the frames are still in flight while the new connection does
	// its handshake, so the hub can build the joiner's snapshot first. There
	// is no ordering between two independent connections, and the hub is
	// right not to invent one. The hub mutates its state and only then
	// relays, so a witness that has *received* the relay proves the state is
	// already applied — and the hub is one goroutine, so any join it handles
	// afterwards must see it.
	witness := dial(t, room)
	next(t, witness, TypeSnapshot)
	waitPresence(t, witness, 2)

	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityPaste, Lines: 3})
	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityAway})

	next(t, witness, TypeActivity)
	next(t, witness, TypeActivity)

	// A fresh connection stands in for the interviewer reloading the page.
	joiner := dial(t, room)
	snap := next(t, joiner, TypeSnapshot)
	if snap.Activity == nil {
		t.Fatal("snapshot carried no tally")
	}
	if snap.Activity.PasteCount != 1 || snap.Activity.AwayCount != 1 {
		t.Fatalf("tally = %+v", *snap.Activity)
	}
	if !snap.Activity.Away {
		t.Fatal("candidate was away; the snapshot should say so")
	}
}

// The interviewer needs the pasted text, not just a count — "pasted 12 lines"
// does not tell them whether it was a solution or a test case.
func TestPasteContentReachesTheInterviewer(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("contentroom")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	watcher := dial(t, room)
	next(t, watcher, TypeSnapshot)
	waitPresence(t, watcher, 2)
	waitPresence(t, candidate, 2)

	send(t, candidate, Message{
		Type:  TypeActivity,
		Kind:  ActivityPaste,
		Lines: 2,
		Text:  "def solve(xs):\n    return max(xs)",
	})

	got := next(t, watcher, TypeActivity)
	if got.Event == nil {
		t.Fatal("no event attached")
	}
	if got.Event.Text != "def solve(xs):\n    return max(xs)" {
		t.Fatalf("pasted text = %q", got.Event.Text)
	}
	if got.Event.At == 0 {
		t.Error("event carried no server timestamp")
	}
	if got.Event.Truncated {
		t.Error("a short paste was marked truncated")
	}
}

// A huge paste must not be relayed whole or kept whole — and the interviewer
// has to be told it was cut, or they will read a fragment as the full thing.
func TestOversizedPasteIsTruncatedAndFlagged(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("bigpaste")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	watcher := dial(t, room)
	next(t, watcher, TypeSnapshot)
	waitPresence(t, watcher, 2)
	waitPresence(t, candidate, 2)

	huge := strings.Repeat("x", MaxPasteChars*2)
	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityPaste, Text: huge})

	got := next(t, watcher, TypeActivity)
	if len(got.Event.Text) != MaxPasteChars {
		t.Fatalf("kept %d chars, want %d", len(got.Event.Text), MaxPasteChars)
	}
	if !got.Event.Truncated {
		t.Fatal("truncation was not flagged")
	}
}

// The interviewer gets the timeline on join, so reloading does not lose what
// happened before it.
func TestEventLogIsSentToInterviewersOnly(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("logroom")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	// An observer already in the room, so the assertion below cannot pass for
	// the wrong reason.
	//
	// Sending and then immediately dialling proves nothing: if the hub has not
	// read the activity yet, the log is empty and "the joiner got no events"
	// is true even when the code is leaking the log to candidates. Activity is
	// relayed to everyone but its author, so waiting for the observer to see
	// both frames guarantees both are recorded before anybody else joins.
	observer := dial(t, room)
	next(t, observer, TypeSnapshot)
	waitPresence(t, observer, 2)

	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityPaste, Lines: 1, Text: "secret"})
	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityAway})

	next(t, observer, TypeActivity)
	next(t, observer, TypeActivity)

	// A candidate joining must not be handed the record kept about them —
	// and by now there is definitely a record to withhold.
	peer := dial(t, room)
	peerSnap := next(t, peer, TypeSnapshot)
	if len(peerSnap.Events) != 0 {
		t.Fatalf("a candidate received %d events", len(peerSnap.Events))
	}
}

// The other half: an interviewer joining mid-interview is handed everything
// that happened before they arrived, so a reload does not lose the record.
func TestInterviewerReceivesTheEventLogOnJoin(t *testing.T) {
	url := serverAs(t, byToken)
	room := url("mixedroom")

	candidate := dial(t, room+"?token=candidate")
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	// An interviewer already in the room, purely as a barrier.
	//
	// Writing to one socket and immediately dialling another guarantees
	// nothing about the order the hub reads them: the activity frames are in
	// flight while the second dial is still doing its TCP handshake and
	// upgrade, so on a loaded machine the join can be processed first and the
	// snapshot is built before anything has been logged. That is not a bug in
	// the hub — two independent connections have no ordering between them —
	// but it made this test fail roughly one CI run in three.
	//
	// The hub appends to the log and only then relays, so an observer that has
	// *received* both frames proves both are already recorded. The hub is a
	// single goroutine, so any join it processes afterwards must see them.
	observer := dial(t, room+"?token=interviewer")
	next(t, observer, TypeSnapshot)
	waitPresence(t, observer, 2)

	send(t, candidate, Message{
		Type: TypeActivity, Kind: ActivityPaste, Lines: 1, Text: "pasted thing",
	})
	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityAway})

	next(t, observer, TypeActivity)
	next(t, observer, TypeActivity)

	interviewer := dial(t, room+"?token=interviewer")
	snap := next(t, interviewer, TypeSnapshot)

	if len(snap.Events) != 2 {
		t.Fatalf("interviewer got %d events, want 2", len(snap.Events))
	}
	if snap.Events[0].Kind != ActivityPaste || snap.Events[0].Text != "pasted thing" {
		t.Fatalf("first event = %+v", snap.Events[0])
	}
	if snap.Events[1].Kind != ActivityAway {
		t.Fatalf("second event = %+v", snap.Events[1])
	}
	if snap.Events[0].At == 0 {
		t.Error("logged event carried no timestamp")
	}
}

// The log is bounded, or a long interview would grow the room without limit.
func TestEventLogIsBounded(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("boundedroom")

	candidate := dial(t, room)
	next(t, candidate, TypeSnapshot)
	waitPresence(t, candidate, 1)

	watcher := dial(t, room)
	next(t, watcher, TypeSnapshot)
	waitPresence(t, watcher, 2)
	waitPresence(t, candidate, 2)

	for i := range MaxActivityEvents + 10 {
		send(t, candidate, Message{Type: TypeActivity, Kind: ActivityPaste, Lines: i})
	}
	for range MaxActivityEvents + 10 {
		next(t, watcher, TypeActivity)
	}

	joiner := dial(t, room)
	snap := next(t, joiner, TypeSnapshot)
	if len(snap.Events) > MaxActivityEvents {
		t.Fatalf("log grew to %d, cap is %d", len(snap.Events), MaxActivityEvents)
	}
}

package ws

import (
	"testing"
)

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

	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityPaste, Lines: 3})
	send(t, candidate, Message{Type: TypeActivity, Kind: ActivityAway})

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

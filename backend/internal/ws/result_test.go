package ws

import (
	"testing"
)

// A run result must reach the other side of the interview. Before this frame
// existed, only the person who pressed Run saw the output, which meant an
// interviewer could not see what their candidate's code actually did.
func TestResultReachesTheOtherClient(t *testing.T) {
	_, url := newTestServer(t)
	room := url("resultroom")

	author := dial(t, room)
	next(t, author, TypeSnapshot)
	waitPresence(t, author, 1)

	peer := dial(t, room)
	next(t, peer, TypeSnapshot)
	waitPresence(t, peer, 2)
	waitPresence(t, author, 2)

	send(t, author, Message{Type: TypeResult, Text: "22\n// Accepted in 0.03s"})

	got := next(t, peer, TypeResult)
	if got.Text != "22\n// Accepted in 0.03s" {
		t.Fatalf("peer got %q", got.Text)
	}
	if got.Failed {
		t.Error("a successful run arrived marked failed")
	}
}

// Failure has to survive the relay, or the other side renders a red error as
// though it were normal output.
func TestResultCarriesTheFailureFlag(t *testing.T) {
	_, url := newTestServer(t)
	room := url("failroom")

	author := dial(t, room)
	next(t, author, TypeSnapshot)
	waitPresence(t, author, 1)

	peer := dial(t, room)
	next(t, peer, TypeSnapshot)
	waitPresence(t, peer, 2)
	waitPresence(t, author, 2)

	send(t, author, Message{Type: TypeResult, Text: "NameError: x", Failed: true})

	got := next(t, peer, TypeResult)
	if !got.Failed {
		t.Fatal("failure flag was lost in the relay")
	}
}

// The result must not become part of the shared document. If it did, pressing
// Run would overwrite whatever the other person was typing.
func TestResultDoesNotTouchTheDocument(t *testing.T) {
	_, url := newTestServer(t)
	room := url("docroom")

	author := dial(t, room)
	next(t, author, TypeSnapshot)
	waitPresence(t, author, 1)

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

	send(t, author, Message{Type: TypeEdit, Text: "print('hello')"})
	send(t, author, Message{Type: TypeResult, Text: "hello"})

	next(t, witness, TypeEdit)
	next(t, witness, TypeResult)

	// A fresh joiner's snapshot is the document, and it must be the code —
	// not the output, and not a mixture.
	joiner := dial(t, room)
	snap := next(t, joiner, TypeSnapshot)
	if snap.Text != "print('hello')" {
		t.Fatalf("snapshot = %q, want the code only", snap.Text)
	}
}

// A result is a moment, not state. Someone joining afterwards should not be
// shown output from a run they were not present for.
func TestResultIsNotReplayedToLateJoiners(t *testing.T) {
	_, url := newTestServer(t)
	room := url("replayroom")

	author := dial(t, room)
	next(t, author, TypeSnapshot)
	waitPresence(t, author, 1)

	// A witness, so this cannot pass for the wrong reason: without one, an
	// unprocessed result means there is nothing to replay and the assertion
	// holds even if results were being stored.
	witness := dial(t, room)
	next(t, witness, TypeSnapshot)
	waitPresence(t, witness, 2)

	send(t, author, Message{Type: TypeResult, Text: "stale output"})
	next(t, witness, TypeResult)

	joiner := dial(t, room)
	// The snapshot is the first thing a joiner gets; if a result were stored
	// it would have to arrive around here. Reading presence proves the
	// connection is live and working without a result having shown up.
	//
	// Three, not two: the author, the witness and this joiner. next() skips
	// frames until it finds the type it wants, so a count that never occurs
	// makes this block until the read deadline rather than failing usefully.
	next(t, joiner, TypeSnapshot)
	waitPresence(t, joiner, 3)
}

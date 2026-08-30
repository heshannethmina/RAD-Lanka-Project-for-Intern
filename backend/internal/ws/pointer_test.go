package ws

import "testing"

// "This line here" is most of what gets said in an interview. Without a shared
// pointer it has to be typed out instead.
func TestPointerReachesTheOtherPerson(t *testing.T) {
	url := serverAs(t, byToken)
	room := url("pointroom")

	a := dial(t, room+"?token=interviewer")
	next(t, a, TypeSnapshot)
	waitPresence(t, a, 1)

	b := dial(t, room+"?token=candidate")
	next(t, b, TypeSnapshot)
	waitPresence(t, b, 2)
	waitPresence(t, a, 2)

	send(t, a, Message{Type: TypePointer, X: 0.25, Y: 0.75})

	got := next(t, b, TypePointer)
	if got.X != 0.25 || got.Y != 0.75 {
		t.Fatalf("got (%v, %v), want (0.25, 0.75)", got.X, got.Y)
	}
	// The role travels so the receiver can colour and label the cursor.
	if got.Role != string(RoleInterviewer) {
		t.Fatalf("role = %q, want interviewer", got.Role)
	}
}

// Out of range is dropped rather than clamped: a bad client should not be able
// to park a cursor off-screen where the other person cannot see it move.
func TestOutOfRangePointerIsDropped(t *testing.T) {
	url := serverAs(t, byToken)
	room := url("badpoint")

	a := dial(t, room+"?token=candidate")
	next(t, a, TypeSnapshot)
	waitPresence(t, a, 1)

	b := dial(t, room+"?token=interviewer")
	next(t, b, TypeSnapshot)
	waitPresence(t, b, 2)
	waitPresence(t, a, 2)

	send(t, a, Message{Type: TypePointer, X: 5, Y: -2})
	// A frame that does arrive proves the bad one was dropped, not delayed.
	send(t, a, Message{Type: TypeEdit, Text: "after"})

	got := next(t, b, TypeEdit)
	if got.Text != "after" {
		t.Fatalf("got %q", got.Text)
	}
}

// A pointer is stale the moment it arrives, so it must not be replayed to
// somebody who joins later.
func TestPointerIsNotStored(t *testing.T) {
	url := serverAs(t, byToken)
	room := url("stalepoint")

	a := dial(t, room+"?token=candidate")
	next(t, a, TypeSnapshot)
	waitPresence(t, a, 1)

	send(t, a, Message{Type: TypePointer, X: 0.5, Y: 0.5})

	joiner := dial(t, room+"?token=interviewer")
	snap := next(t, joiner, TypeSnapshot)
	if snap.X != 0 || snap.Y != 0 {
		t.Fatalf("snapshot carried a stale pointer (%v, %v)", snap.X, snap.Y)
	}
}

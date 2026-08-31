package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serverAs builds a room server where every client gets the given role, so a
// test can prove what that role may and may not do.
func serverAs(t *testing.T, auth Authorizer) func(room string) string {
	t.Helper()

	reg := NewRegistry(nil)
	go reg.Run()

	mux := http.NewServeMux()
	mux.Handle("GET /ws/{roomID}", Handler(reg, auth))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	base := "ws" + strings.TrimPrefix(srv.URL, "http")
	return func(room string) string { return base + "/ws/" + room }
}

// The interviewer sets the question and the candidate sees it live, without
// either of them reloading.
func TestInterviewerPromptReachesTheRoom(t *testing.T) {
	url := serverAs(t, AllowAll)
	room := url("promptroom")

	owner := dial(t, room)
	next(t, owner, TypeSnapshot)
	waitPresence(t, owner, 1)

	peer := dial(t, room)
	next(t, peer, TypeSnapshot)
	waitPresence(t, peer, 2)
	waitPresence(t, owner, 2)

	send(t, owner, Message{Type: TypePrompt, Prompt: "Reverse a linked list."})

	got := next(t, peer, TypePrompt)
	if got.Prompt != "Reverse a linked list." {
		t.Fatalf("peer got prompt %q", got.Prompt)
	}
}

// The question is room state, so somebody joining later must be told it —
// otherwise a candidate who reloads sees an empty panel.
func TestPromptIsInTheSnapshotForLateJoiners(t *testing.T) {
	url := serverAs(t, AllowAll)
	room := url("lateroom")

	owner := dial(t, room)
	next(t, owner, TypeSnapshot)
	waitPresence(t, owner, 1)

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

	send(t, owner, Message{Type: TypePrompt, Prompt: "Largest value in a list."})
	send(t, owner, Message{Type: TypeEdit, Text: "x = 1"})

	next(t, witness, TypePrompt)
	next(t, witness, TypeEdit)

	joiner := dial(t, room)
	snap := next(t, joiner, TypeSnapshot)
	if snap.Prompt != "Largest value in a list." {
		t.Fatalf("late joiner's snapshot prompt = %q", snap.Prompt)
	}
	if snap.Text != "x = 1" {
		t.Fatalf("late joiner's snapshot text = %q", snap.Text)
	}
}

// The snapshot tells a client what it is, so the UI can show an editable
// question to one side and a read-only one to the other.
func TestSnapshotCarriesTheRole(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth Authorizer
		want string
	}{
		{"interviewer", AllowAll, string(RoleInterviewer)},
		{"candidate", AllowAllAsCandidate, string(RoleCandidate)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			url := serverAs(t, tc.auth)
			c := dial(t, url("roleroom"))
			snap := next(t, c, TypeSnapshot)
			if snap.Role != tc.want {
				t.Fatalf("role = %q, want %q", snap.Role, tc.want)
			}
		})
	}
}

// A candidate must not be able to rewrite the question they are being asked.
// They are ignored rather than disconnected: a buggy client should not be able
// to end somebody's interview.
func TestCandidateCannotChangeThePrompt(t *testing.T) {
	url := serverAs(t, AllowAllAsCandidate)
	room := url("guardroom")

	a := dial(t, room)
	next(t, a, TypeSnapshot)
	waitPresence(t, a, 1)

	b := dial(t, room)
	next(t, b, TypeSnapshot)
	waitPresence(t, b, 2)
	waitPresence(t, a, 2)

	send(t, a, Message{Type: TypePrompt, Prompt: "I rewrote the question."})
	// The prompt must not have been relayed. An edit sent afterwards will
	// arrive, and since the hub processes frames in order, seeing the edit
	// without a prompt first proves the prompt was dropped.
	send(t, a, Message{Type: TypeEdit, Text: "still here"})

	got := next(t, b, TypeEdit)
	if got.Text != "still here" {
		t.Fatalf("expected the edit through, got %q", got.Text)
	}

	// And it must not have been stored either.
	joiner := dial(t, room)
	snap := next(t, joiner, TypeSnapshot)
	if snap.Prompt != "" {
		t.Fatalf("a candidate changed the stored prompt to %q", snap.Prompt)
	}
}

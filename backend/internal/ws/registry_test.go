package ws

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// waitForRooms polls the registry until it reports want live rooms. Client
// teardown is asynchronous, so the test cannot assert immediately after a
// Close.
func waitForRooms(t *testing.T, reg *Registry, want int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		got := reg.Rooms()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("registry reports %d rooms, want %d", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRoomsAreIsolated(t *testing.T) {
	_, url := newTestServer(t)

	alice := dial(t, url("room-a"))
	next(t, alice, TypeSnapshot)
	bob := dial(t, url("room-b"))
	next(t, bob, TypeSnapshot)

	send(t, alice, Message{Type: TypeEdit, Text: "only for room a"})

	// Bob is in a different room and must never see that edit. He does get
	// presence frames for his own room, so check the type rather than just
	// asserting silence.
	if err := bob.SetReadDeadline(time.Now().Add(400 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	for {
		_, data, err := bob.ReadMessage()
		if err != nil {
			break // timed out, as expected
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Type == TypeEdit {
			t.Fatalf("edit leaked from room-a into room-b: %q", msg.Text)
		}
	}

	// And room-a's own document is intact.
	observer := dial(t, url("room-a"))
	if got := next(t, observer, TypeSnapshot); got.Text != "only for room a" {
		t.Fatalf("room-a document is %q, want %q", got.Text, "only for room a")
	}
}

func TestRoomOpensOnJoinAndClosesWhenEmpty(t *testing.T) {
	reg, url := newTestServer(t)

	waitForRooms(t, reg, 0)

	first := dial(t, url("standup"))
	next(t, first, TypeSnapshot)
	second := dial(t, url("standup"))
	next(t, second, TypeSnapshot)
	waitForRooms(t, reg, 1)

	// One leaving is not enough to close the room.
	second.Close()
	waitForRooms(t, reg, 1)

	first.Close()
	waitForRooms(t, reg, 0)
}

// A closed room keeps nothing: the document lives only in the hub goroutine,
// so reopening the same ID must start empty. Worth pinning down, because it
// is the behaviour that changes once there is a database.
func TestReopenedRoomStartsEmpty(t *testing.T) {
	reg, url := newTestServer(t)

	first := dial(t, url("ephemeral"))
	next(t, first, TypeSnapshot)
	send(t, first, Message{Type: TypeEdit, Text: "written before everyone left"})

	// Make sure the hub applied the edit before we tear the room down.
	witness := dial(t, url("ephemeral"))
	next(t, witness, TypeSnapshot)

	first.Close()
	witness.Close()
	waitForRooms(t, reg, 0)

	rejoin := dial(t, url("ephemeral"))
	if got := next(t, rejoin, TypeSnapshot); got.Text != "" {
		t.Fatalf("reopened room has document %q, want empty", got.Text)
	}
}

func TestInvalidRoomIDIsRejected(t *testing.T) {
	_, url := newTestServer(t)

	for _, room := range []string{
		"has%20space",
		"slash%2Fpath",
		"waaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaay-too-long",
	} {
		conn, resp, err := websocket.DefaultDialer.Dial(url(room), nil)
		if err == nil {
			conn.Close()
			t.Fatalf("room %q was accepted, want rejection", room)
		}
		if resp == nil {
			t.Fatalf("room %q: no response: %v", room, err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("room %q: status %d, want %d", room, resp.StatusCode, http.StatusBadRequest)
		}
		resp.Body.Close()
	}
}

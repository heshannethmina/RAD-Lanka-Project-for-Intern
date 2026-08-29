package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestRoom spins up a real HTTP server backed by a running hub.
func newTestRoom(t *testing.T) string {
	t.Helper()

	h := NewHub()
	go h.Run()

	mux := http.NewServeMux()
	mux.Handle("GET /ws", Handler(h))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// next reads frames until one of the wanted type arrives, so a test can
// ignore presence chatter it does not care about.
func next(t *testing.T, conn *websocket.Conn, want MessageType) Message {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %q: %v", want, err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Type == want {
			return msg
		}
	}
}

func send(t *testing.T, conn *websocket.Conn, msg Message) {
	t.Helper()

	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestJoinerReceivesSnapshot(t *testing.T) {
	url := newTestRoom(t)

	first := dial(t, url)
	if got := next(t, first, TypeSnapshot); got.Text != "" {
		t.Fatalf("first joiner got document %q, want empty", got.Text)
	}

	send(t, first, Message{Type: TypeEdit, Text: "package main"})

	// A client joining after the edit must be handed the current document,
	// not an empty one.
	second := dial(t, url)
	if got := next(t, second, TypeSnapshot); got.Text != "package main" {
		t.Fatalf("late joiner got document %q, want %q", got.Text, "package main")
	}
}

func TestEditBroadcastsToOthersButNotAuthor(t *testing.T) {
	url := newTestRoom(t)

	author := dial(t, url)
	next(t, author, TypeSnapshot)
	peer := dial(t, url)
	next(t, peer, TypeSnapshot)

	send(t, author, Message{Type: TypeEdit, Text: "func main() {}"})

	if got := next(t, peer, TypeEdit); got.Text != "func main() {}" {
		t.Fatalf("peer got %q, want %q", got.Text, "func main() {}")
	}

	// The author must not be echoed its own edit: that would fight with the
	// local cursor. Give the hub a beat, then assert nothing arrives.
	if err := author.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	for {
		_, data, err := author.ReadMessage()
		if err != nil {
			break // timed out, which is the expected outcome
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Type == TypeEdit {
			t.Fatal("author was echoed its own edit")
		}
	}
}

func TestPresenceTracksRoomSize(t *testing.T) {
	url := newTestRoom(t)

	first := dial(t, url)
	next(t, first, TypeSnapshot)
	// Its own join produced a presence frame; consume it so the assertions
	// below read the frames caused by the second client.
	if got := next(t, first, TypePresence); got.Clients != 1 {
		t.Fatalf("on join, presence reported %d clients, want 1", got.Clients)
	}

	second := dial(t, url)
	if got := next(t, first, TypePresence); got.Clients != 2 {
		t.Fatalf("presence reported %d clients, want 2", got.Clients)
	}

	second.Close()
	if got := next(t, first, TypePresence); got.Clients != 1 {
		t.Fatalf("after leave, presence reported %d clients, want 1", got.Clients)
	}
}

// TestConcurrentEditsSerialise is the one that matters for the design: many
// clients hammering the hub at once must leave it with a document equal to
// some edit that was actually sent, never a torn mix, and must not race.
func TestConcurrentEditsSerialise(t *testing.T) {
	url := newTestRoom(t)

	const writers = 8
	const edits = 25

	conns := make([]*websocket.Conn, writers)
	for i := range conns {
		conns[i] = dial(t, url)
		next(t, conns[i], TypeSnapshot)
	}

	done := make(chan struct{})
	for i, conn := range conns {
		go func(id int, conn *websocket.Conn) {
			defer func() { done <- struct{}{} }()
			for n := 0; n < edits; n++ {
				_ = conn.WriteJSON(Message{
					Type: TypeEdit,
					Text: strings.Repeat("x", id+1),
				})
			}
		}(i, conn)
	}
	for range conns {
		<-done
	}

	// Drain, then confirm the room converged on one of the sent documents.
	observer := dial(t, url)
	got := next(t, observer, TypeSnapshot)
	if len(got.Text) < 1 || len(got.Text) > writers ||
		strings.Trim(got.Text, "x") != "" {
		t.Fatalf("document %q is not a whole edit from any writer", got.Text)
	}
}

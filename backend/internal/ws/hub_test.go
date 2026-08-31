package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestServer spins up a real HTTP server backed by a running registry,
// and returns a function that builds the URL for a given room.
func newTestServer(t *testing.T) (*Registry, func(room string) string) {
	t.Helper()

	reg := NewRegistry(nil)
	go reg.Run()

	mux := http.NewServeMux()
	// AllowAll: these tests are about the hub and the registry, not about who
	// is allowed in. Authorization has its own tests in package api.
	mux.Handle("GET /ws/{roomID}", Handler(reg, AllowAll))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	base := "ws" + strings.TrimPrefix(srv.URL, "http")
	return reg, func(room string) string { return base + "/ws/" + room }
}

// newTestRoom is the common case: one room on a fresh server.
func newTestRoom(t *testing.T) string {
	t.Helper()

	_, url := newTestServer(t)
	return url("demo")
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

// waitPresence reads until presence reports want clients. Presence frames
// queue up (a client sees its own join too), so tests must match on the
// count rather than on frame position.
func waitPresence(t *testing.T, conn *websocket.Conn, want int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for presence of %d", want)
		}
		got := next(t, conn, TypePresence)
		if got.Clients == want {
			return
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
	witness := dial(t, url)
	next(t, witness, TypeSnapshot)

	send(t, first, Message{Type: TypeEdit, Text: "package main"})
	next(t, witness, TypeEdit)

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
	waitPresence(t, first, 1)

	second := dial(t, url)
	waitPresence(t, first, 2)

	second.Close()
	waitPresence(t, first, 1)
}

// TestConcurrentEditsSerialise is the one that matters for the design: many
// clients hammering the hub at once must leave it with a document equal to
// some edit that was actually sent, never a torn mix of several.
//
// This drives h.inbound directly rather than going through sockets. Over real
// connections the writers generate enough traffic to overrun each other's send
// buffers, so the hub correctly drops them and the room empties — which tests
// backpressure (TestSlowClientIsDropped) instead of serialisation, and does it
// flakily. The mechanism under test is the channel plus the single Run
// goroutine, and that is exercised fully here.
func TestConcurrentEditsSerialise(t *testing.T) {
	h := NewHub("unit", nil)
	go h.Run()
	defer h.Stop()

	const writers = 8
	const edits = 25

	// Buffered generously so the observer is never dropped for lagging.
	observer := &Client{hub: h, send: make(chan []byte, writers*edits+16)}
	h.register <- observer

	var wg sync.WaitGroup
	for id := 1; id <= writers; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			payload, err := json.Marshal(Message{
				Type: TypeEdit,
				Text: strings.Repeat("x", id),
			})
			if err != nil {
				t.Errorf("marshal: %v", err)
				return
			}
			for range edits {
				h.inbound <- inbound{data: payload}
			}
		}(id)
	}
	wg.Wait()

	// Every edit the observer saw must be one writer's whole document.
	timeout := time.After(3 * time.Second)
	seen := 0
	for seen < writers*edits {
		select {
		case data := <-observer.send:
			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if msg.Type != TypeEdit {
				continue
			}
			seen++
			if len(msg.Text) < 1 || len(msg.Text) > writers ||
				strings.Trim(msg.Text, "x") != "" {
				t.Fatalf("relayed a torn document: %q", msg.Text)
			}
		case <-timeout:
			t.Fatalf("saw %d of %d edits", seen, writers*edits)
		}
	}

	// And the document the hub settled on is likewise a whole edit. Read it
	// through a fresh joiner's snapshot rather than touching h.document, which
	// belongs to the Run goroutine.
	joiner := &Client{hub: h, send: make(chan []byte, 4)}
	h.register <- joiner

	var snap Message
	if err := json.Unmarshal(<-joiner.send, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if snap.Type != TypeSnapshot {
		t.Fatalf("first frame was %q, want snapshot", snap.Type)
	}
	if len(snap.Text) < 1 || len(snap.Text) > writers ||
		strings.Trim(snap.Text, "x") != "" {
		t.Fatalf("document %q is not a whole edit from any writer", snap.Text)
	}
}

// TestSlowClientIsDropped pins the hub's central safety property: it is the
// room's only thread of control, so a client that stops draining must be
// dropped rather than allowed to wedge everyone else.
//
// This drives the hub's logic directly instead of going through a socket.
// Doing it over a real connection would mean pushing megabytes to defeat the
// OS socket buffer, which is slow and flaky; and calling into the hub from
// the test goroutine is safe precisely because Run is not started, so nothing
// else owns the state.
func TestSlowClientIsDropped(t *testing.T) {
	h := NewHub("unit", nil)

	stuck := &Client{hub: h, send: make(chan []byte, 1)}
	healthy := &Client{hub: h, send: make(chan []byte, 4)}
	h.clients[stuck] = struct{}{}
	h.clients[healthy] = struct{}{}

	// Two frames, one more than stuck can hold.
	h.broadcast(Message{Type: TypeEdit, Text: "one"}, nil)
	h.broadcast(Message{Type: TypeEdit, Text: "two"}, nil)

	if _, ok := h.clients[stuck]; ok {
		t.Fatal("client that overran its buffer is still in the room")
	}
	if _, ok := h.clients[healthy]; !ok {
		t.Fatal("healthy client was dropped alongside the slow one")
	}
	if len(healthy.send) != 2 {
		t.Fatalf("healthy client received %d frames, want 2", len(healthy.send))
	}

	// Its send channel must be closed, which is how the write pump learns to
	// shut the connection down.
	<-stuck.send
	if _, open := <-stuck.send; open {
		t.Fatal("dropped client's send channel was not closed")
	}

	// And the room must be told it shrank, otherwise presence stays stale
	// forever: the slow client's later unregister is a no-op.
	if !h.presenceDirty {
		t.Fatal("dropping a client did not mark presence dirty")
	}
	h.flushPresence()

	var got Message
	// Past the two edits already queued, the next frame is the presence update.
	for range 2 {
		<-healthy.send
	}
	if err := json.Unmarshal(<-healthy.send, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != TypePresence || got.Clients != 1 {
		t.Fatalf("got %q with %d clients, want presence with 1", got.Type, got.Clients)
	}
}

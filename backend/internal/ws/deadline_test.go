package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// endingIn builds an authorizer that grants everyone an interview ending after
// d, so the timer can be exercised without waiting for a real hour.
func endingIn(d time.Duration) Authorizer {
	return func(context.Context, string, string) (Grant, error) {
		return Grant{Role: RoleInterviewer, EndsAt: time.Now().Add(d)}, nil
	}
}

func serverWith(t *testing.T, auth Authorizer) (func(string) string, chan string) {
	t.Helper()

	ended := make(chan string, 4)
	reg := NewRegistry(func(roomID string) { ended <- roomID })
	go reg.Run()

	mux := http.NewServeMux()
	mux.Handle("GET /ws/{roomID}", Handler(reg, auth))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	base := "ws" + strings.TrimPrefix(srv.URL, "http")
	return func(room string) string { return base + "/ws/" + room }, ended
}

// The interview stops on its own when the plan's time runs out, and both
// people are told rather than being cut off.
func TestInterviewEndsWhenTimeRunsOut(t *testing.T) {
	url, ended := serverWith(t, endingIn(150*time.Millisecond))

	c := dial(t, url("timedroom"))
	snap := next(t, c, TypeSnapshot)
	if snap.EndsAt == nil {
		t.Fatal("snapshot carried no deadline")
	}
	if snap.Ended {
		t.Fatal("a fresh room reported itself already ended")
	}

	// The frame is what lets the UI explain itself; a dropped socket would
	// leave both people looking at a reconnect spinner.
	next(t, c, TypeEnded)

	select {
	case room := <-ended:
		if room != "timedroom" {
			t.Fatalf("recorded %q", room)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the store was never told the interview expired")
	}
}

// After the deadline the room is read-only. Edits are dropped rather than
// answered with an error per keystroke.
func TestEditsAreIgnoredAfterTheDeadline(t *testing.T) {
	url, _ := serverWith(t, endingIn(120*time.Millisecond))
	room := url("frozenroom")

	author := dial(t, room)
	next(t, author, TypeSnapshot)
	waitPresence(t, author, 1)

	peer := dial(t, room)
	next(t, peer, TypeSnapshot)
	waitPresence(t, peer, 2)
	waitPresence(t, author, 2)

	next(t, author, TypeEnded)
	next(t, peer, TypeEnded)

	send(t, author, Message{Type: TypeEdit, Text: "sneaking this in"})

	// Nothing should reach the peer. A short read window is enough: the hub
	// relays within microseconds when it relays at all.
	if err := peer.SetReadDeadline(time.Now().Add(400 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	for {
		var msg Message
		if err := peer.ReadJSON(&msg); err != nil {
			return // timed out, which is the pass
		}
		if msg.Type == TypeEdit {
			t.Fatal("an edit was relayed after the interview ended")
		}
	}
}

// Somebody reopening a finished interview is told immediately, rather than
// being left in a room that silently ignores them.
func TestRejoiningAnExpiredInterviewIsToldAtOnce(t *testing.T) {
	url, _ := serverWith(t, endingIn(-time.Minute))

	c := dial(t, url("overroom"))
	snap := next(t, c, TypeSnapshot)
	if !snap.Ended {
		t.Fatal("snapshot did not report the interview as over")
	}
}

// A second joiner must not be able to extend the interview by reporting a
// later deadline than the one already set.
func TestLaterJoinCannotExtendTheDeadline(t *testing.T) {
	first := true
	auth := func(context.Context, string, string) (Grant, error) {
		if first {
			first = false
			return Grant{Role: RoleInterviewer, EndsAt: time.Now().Add(150 * time.Millisecond)}, nil
		}
		return Grant{Role: RoleCandidate, EndsAt: time.Now().Add(time.Hour)}, nil
	}

	url, _ := serverWith(t, auth)
	room := url("extendroom")

	a := dial(t, room)
	next(t, a, TypeSnapshot)
	waitPresence(t, a, 1)

	b := dial(t, room)
	next(t, b, TypeSnapshot)
	waitPresence(t, b, 2)

	// The original short deadline must still apply.
	next(t, a, TypeEnded)
	next(t, b, TypeEnded)
}

// No deadline means no timer: an unmetered plan runs until somebody ends it.
func TestNoDeadlineNeverExpires(t *testing.T) {
	url, _ := serverWith(t, AllowAll)

	c := dial(t, url("unmetered"))
	snap := next(t, c, TypeSnapshot)
	if snap.EndsAt != nil {
		t.Fatalf("unmetered room reported a deadline: %v", *snap.EndsAt)
	}

	if err := c.SetReadDeadline(time.Now().Add(400 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	for {
		var msg Message
		if err := c.ReadJSON(&msg); err != nil {
			return
		}
		if msg.Type == TypeEnded {
			t.Fatal("an unmetered interview ended on its own")
		}
	}
}

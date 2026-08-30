package ws

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// ErrUnauthorized is what an Authorizer returns when the bearer of a token
// may not join. It is the only failure the handler turns into a 401; anything
// else is treated as the authorizer itself being broken.
var ErrUnauthorized = errors.New("ws: unauthorized")

// Role is what a connected client is allowed to do.
//
// The hub needs this because the two participants are not equivalent: an
// interviewer sets the question, a candidate answers it. Without a role, a
// candidate could rewrite the question they are being asked.
type Role string

const (
	// RoleInterviewer owns the room. May change the prompt.
	RoleInterviewer Role = "interviewer"
	// RoleCandidate arrived through an invite link. May edit code and run it,
	// but not change the question.
	RoleCandidate Role = "candidate"
)

// Authorizer decides whether a connection may join a room, and as what.
//
// It is a function rather than a concrete type so this package stays ignorant
// of Postgres and of what a token means: the hub's job is to move text between
// sockets, and it should not grow a dependency on the store to do it. The
// server wires in an implementation that accepts either an interviewer's
// session token or a room's invite token.
type Authorizer func(ctx context.Context, roomID, token string) (Grant, error)

// Grant is what an authorizer decided: who this connection is, and how long
// the interview has left.
//
// The deadline arrives with the authorization because the authorizer is
// already looking the room up. Asking separately would be a second query on
// every join, and the two answers could disagree.
type Grant struct {
	Role Role
	// EndsAt is when the interview must stop. Zero means no limit, which is
	// what an unmetered plan looks like from here.
	EndsAt time.Time
}

// AllowAll is an Authorizer that admits everyone as an interviewer. It exists
// for tests, and is named so that using it in the server is obviously wrong at
// the call site.
func AllowAll(context.Context, string, string) (Grant, error) {
	return Grant{Role: RoleInterviewer}, nil
}

// AllowAllAsCandidate is AllowAll with the lesser role, for tests that need to
// prove a candidate is refused something.
func AllowAllAsCandidate(context.Context, string, string) (Grant, error) {
	return Grant{Role: RoleCandidate}, nil
}

// Upgrader is exported so the server can tighten CheckOrigin at startup, where
// the allowed origins are known.
//
// WebSocket handshakes are not covered by CORS — the browser sends them
// regardless of origin — so CheckOrigin is the *only* thing standing between a
// hostile page and a socket opened with a victim's token in the query string.
// Leaving it open is a real hole once tokens exist, not a formality.
// The default allows nothing from a browser: with an empty allowlist, only
// requests carrying no Origin at all get through. A server that forgets to
// call AllowOrigins is therefore closed to browsers rather than open to them.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     originChecker(""),
}

// AllowOrigins configures Upgrader.CheckOrigin from a comma-separated
// allowlist. "*" allows any origin and is for local development only.
func AllowOrigins(origins string) {
	Upgrader.CheckOrigin = originChecker(origins)
}

// originChecker builds the CheckOrigin policy.
//
// A request with no Origin header is allowed. Those come from non-browser
// clients — the Go test suite, websocat, a health check — and they are not
// what this defends against: the attack it stops is a *browser* being told by
// an untrusted page to open a socket, and a browser always sends Origin. A
// caller that can omit the header can equally forge one, so refusing here
// would cost real clients something and buy nothing.
func originChecker(origins string) func(*http.Request) bool {
	var allowed []string
	wildcard := false
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o == "*" {
			wildcard = true
		} else if o != "" {
			allowed = append(allowed, o)
		}
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || wildcard {
			return true
		}
		for _, a := range allowed {
			if strings.EqualFold(a, origin) {
				return true
			}
		}
		log.Printf("ws: refused upgrade from origin %q", origin)
		return false
	}
}

// Handler upgrades an HTTP request into a client of the room named by the
// {roomID} path value. It blocks for the lifetime of the connection, on the
// HTTP server's own goroutine.
func Handler(reg *Registry, authorize Authorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.PathValue("roomID")
		if !ValidRoomID(roomID) {
			// Reject before upgrading, so the client gets a real status code
			// instead of an immediately-closed socket.
			http.Error(w, "invalid room id", http.StatusBadRequest)
			return
		}

		// The token rides in the query string because browsers cannot set
		// headers on a WebSocket handshake. That puts it in this server's
		// access log if one is ever enabled, which is the accepted cost; the
		// alternative, Sec-WebSocket-Protocol smuggling, is worse to read and
		// no more secret.
		grant, err := authorize(r.Context(), roomID, r.URL.Query().Get("token"))
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				// Again before the upgrade: a 401 is something the client can
				// act on, where a socket that opens and closes is not.
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			log.Printf("ws: authorize room %q: %v", roomID, err)
			http.Error(w, "could not verify access", http.StatusInternalServerError)
			return
		}

		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade already wrote an error response.
			log.Printf("ws: upgrade: %v", err)
			return
		}

		hub := reg.Join(roomID)
		// Leave must run after Run returns. By then the client's read pump has
		// already handed its unregister to the hub, so nothing is in flight
		// when the registry decides whether to close the room.
		defer reg.Leave(roomID)

		// The first client through sets the room's deadline. Later joins carry
		// the same value and the hub ignores the repeats.
		hub.SetDeadline(grant.EndsAt)

		c := NewClient(hub, conn, grant.Role)
		hub.Register(c)
		c.Run()
	}
}

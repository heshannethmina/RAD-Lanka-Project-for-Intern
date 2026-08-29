package ws

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// No auth yet and nothing worth protecting, and the Next.js dev server is
	// a different origin. Tighten this to an allowlist before real sessions.
	CheckOrigin: func(*http.Request) bool { return true },
}

// Handler upgrades an HTTP request into a client of the room named by the
// {roomID} path value. It blocks for the lifetime of the connection, on the
// HTTP server's own goroutine.
func Handler(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.PathValue("roomID")
		if !ValidRoomID(roomID) {
			// Reject before upgrading, so the client gets a real status code
			// instead of an immediately-closed socket.
			http.Error(w, "invalid room id", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
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

		c := NewClient(hub, conn)
		hub.Register(c)
		c.Run()
	}
}

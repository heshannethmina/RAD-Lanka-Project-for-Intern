package ws

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Step 1 has no auth and nothing worth protecting, and the Next.js dev
	// server is a different origin. Tighten this to an allowlist before
	// real sessions exist.
	CheckOrigin: func(*http.Request) bool { return true },
}

// Handler upgrades an HTTP request into a client on h. It blocks for the
// lifetime of the connection, on the HTTP server's own goroutine.
func Handler(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade already wrote an error response.
			log.Printf("ws: upgrade: %v", err)
			return
		}

		c := NewClient(h, conn)
		h.Register(c)
		c.Run()
	}
}

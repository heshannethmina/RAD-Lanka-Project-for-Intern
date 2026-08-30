package ws

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is how long a single write may take before we give up on
	// the peer.
	writeWait = 10 * time.Second
	// pongWait is how long we tolerate silence from a peer. It must be
	// comfortably larger than pingPeriod.
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize caps a single frame. Edits carry the whole document,
	// so this is generous by design.
	maxMessageSize       = 128 * 1024
	maxMessagesPerSecond = 60

	// sendBuffer absorbs a short burst of broadcasts. A client that fills
	// it is too slow and gets dropped rather than stalling the hub.
	sendBuffer = 32
)

// Client is one WebSocket connection. It owns exactly two goroutines: a
// read pump that funnels inbound edits into the hub, and a write pump that
// is the sole writer to the socket. gorilla/websocket permits one
// concurrent reader and one concurrent writer, and this split is what
// keeps us inside that rule.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	// role is set once at construction and never written again, so the hub
	// goroutine may read it without coordination.
	role Role
	// send is written to only by the hub goroutine and closed only by the
	// hub goroutine, so there is no race on close.
	send           chan []byte
	windowStarted  time.Time
	windowMessages int
}

// NewClient wraps an upgraded connection. It does not start any goroutines;
// call Run for that.
func NewClient(hub *Hub, conn *websocket.Conn, role Role) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		role: role,
		send: make(chan []byte, sendBuffer),
	}
}

// Run starts the write pump and then blocks on the read pump until the
// connection closes. Callers run it on the HTTP handler's own goroutine.
func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}

// readPump reads frames off the socket and hands them to the hub. It is
// the only reader of the connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		now := time.Now()
		if c.windowStarted.IsZero() || now.Sub(c.windowStarted) >= time.Second {
			c.windowStarted = now
			c.windowMessages = 0
		}
		c.windowMessages++
		if c.windowMessages > maxMessagesPerSecond {
			return
		}
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("ws: read: %v", err)
			}
			return
		}
		c.hub.inbound <- inbound{client: c, data: data}
	}
}

// writePump drains the send channel and keeps the connection alive with
// pings. It is the only writer of the connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				// The hub closed the channel: this client is being dropped.
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

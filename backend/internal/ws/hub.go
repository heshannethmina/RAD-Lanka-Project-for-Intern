package ws

import (
	"encoding/json"
	"log"
)

// inbound pairs a raw frame with the client that sent it, so the hub can
// echo an edit to everyone except its author.
type inbound struct {
	client *Client
	data   []byte
}

// Hub is the sole owner of one room's document and client set.
//
// Nothing outside the Run goroutine reads or writes those two fields.
// Every mutation arrives as a channel message and is applied in arrival
// order by a single goroutine, which is what makes whole-document sync
// correct without operational transformation. That is also why there is
// no mutex in this file, and why adding one would defeat the design:
// the state is not shared, it is owned.
type Hub struct {
	// Owned by Run. Do not touch from any other goroutine.
	clients  map[*Client]struct{}
	document string

	register   chan *Client
	unregister chan *Client
	inbound    chan inbound
}

// NewHub builds a hub with an empty document. Call Run in its own
// goroutine before registering anyone.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		inbound:    make(chan inbound),
	}
}

// Register hands a new client to the hub goroutine and blocks until it is
// accepted, so the caller knows the client is in the room before it starts
// pumping.
func (h *Hub) Register(c *Client) { h.register <- c }

// Run owns the room. It never returns.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = struct{}{}
			// A late joiner needs the current document before anything else,
			// otherwise it would sit on an empty buffer until someone types.
			h.sendTo(c, Message{Type: TypeSnapshot, Text: h.document})
			h.broadcastPresence()
			log.Printf("ws: client joined (%d in room)", len(h.clients))

		case c := <-h.unregister:
			if _, ok := h.clients[c]; !ok {
				// Already dropped for being slow; its read pump is just
				// catching up. Nothing to do, and importantly do not close
				// send twice.
				continue
			}
			h.drop(c)
			h.broadcastPresence()
			log.Printf("ws: client left (%d in room)", len(h.clients))

		case in := <-h.inbound:
			h.apply(in)
		}
	}
}

// apply folds one inbound frame into the document.
func (h *Hub) apply(in inbound) {
	var msg Message
	if err := json.Unmarshal(in.data, &msg); err != nil {
		log.Printf("ws: discarding malformed frame: %v", err)
		return
	}
	if msg.Type != TypeEdit {
		// Clients have nothing else to say yet. Ignore rather than error:
		// an older or newer client should not be able to kill the room.
		return
	}

	h.document = msg.Text
	// The author already has this text locally; echoing it back would fight
	// with their cursor.
	h.broadcast(Message{Type: TypeEdit, Text: msg.Text}, in.client)
}

// broadcast sends msg to every client except skip, which may be nil.
func (h *Hub) broadcast(msg Message, skip *Client) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws: marshal: %v", err)
		return
	}
	for c := range h.clients {
		if c == skip {
			continue
		}
		h.deliver(c, data)
	}
}

func (h *Hub) sendTo(c *Client, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws: marshal: %v", err)
		return
	}
	h.deliver(c, data)
}

func (h *Hub) broadcastPresence() {
	h.broadcast(Message{Type: TypePresence, Clients: len(h.clients)}, nil)
}

// deliver must never block: the hub is the whole room's single thread of
// control, so one wedged client cannot be allowed to freeze everyone else.
// A client whose buffer is full is dropped instead.
func (h *Hub) deliver(c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
		log.Print("ws: dropping client that fell behind")
		h.drop(c)
	}
}

// drop removes a client and closes its send channel, which is the signal
// its write pump uses to shut the connection down. Deleting during a range
// over h.clients is safe, and deliver relies on that.
func (h *Hub) drop(c *Client) {
	delete(h.clients, c)
	close(c.send)
}

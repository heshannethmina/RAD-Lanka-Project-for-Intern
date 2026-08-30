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
	// roomID is set once at construction and never written again.
	roomID string

	// Owned by Run. Do not touch from any other goroutine.
	clients  map[*Client]struct{}
	document string
	// presenceDirty means the client set changed and the room has not been
	// told yet. Set it instead of broadcasting inline: a drop can happen
	// deep inside a broadcast's range loop, and re-entering broadcast from
	// there would iterate the map while it is being mutated.
	presenceDirty bool

	register   chan *Client
	unregister chan *Client
	inbound    chan inbound
	// quit is closed by the registry when the last client has left.
	quit chan struct{}
}

// NewHub builds a hub for roomID with an empty document. Call Run in its own
// goroutine before registering anyone.
func NewHub(roomID string) *Hub {
	return &Hub{
		roomID:     roomID,
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		inbound:    make(chan inbound),
		quit:       make(chan struct{}),
	}
}

// Stop shuts the hub's goroutine down. Only the registry calls it, exactly
// once, after the room is empty and unreachable.
func (h *Hub) Stop() { close(h.quit) }

// Register hands a new client to the hub goroutine and blocks until it is
// accepted, so the caller knows the client is in the room before it starts
// pumping.
func (h *Hub) Register(c *Client) { h.register <- c }

// Run owns the room. It returns when the registry stops the hub.
func (h *Hub) Run() {
	for {
		select {
		case <-h.quit:
			return

		case c := <-h.register:
			h.clients[c] = struct{}{}
			// A late joiner needs the current document before anything else,
			// otherwise it would sit on an empty buffer until someone types.
			h.sendTo(c, Message{Type: TypeSnapshot, Text: h.document})
			h.presenceDirty = true
			log.Printf("ws: %s: client joined (%d in room)", h.roomID, len(h.clients))

		case c := <-h.unregister:
			// It may already be gone, dropped for being slow while its read
			// pump was still catching up. Then there is nothing to do, and
			// importantly its send channel must not be closed twice.
			if _, ok := h.clients[c]; ok {
				h.drop(c)
				log.Printf("ws: %s: client left (%d in room)", h.roomID, len(h.clients))
			}

		case in := <-h.inbound:
			h.apply(in)
		}

		h.flushPresence()
	}
}

// flushPresence tells the room its size, if anything changed. It loops
// because sending presence can itself drop a client and change the count
// again; that terminates because every extra pass removes at least one
// client.
func (h *Hub) flushPresence() {
	for h.presenceDirty {
		h.presenceDirty = false
		h.broadcast(Message{Type: TypePresence, Clients: len(h.clients)}, nil)
	}
}

// apply folds one inbound frame into the document.
func (h *Hub) apply(in inbound) {
	var msg Message
	if err := json.Unmarshal(in.data, &msg); err != nil {
		log.Printf("ws: discarding malformed frame: %v", err)
		return
	}
	switch msg.Type {
	case TypeEdit:
		h.document = msg.Text
		// The author already has this text locally; echoing it back would
		// fight with their cursor.
		h.broadcast(Message{Type: TypeEdit, Text: msg.Text}, in.client)

	case TypeResult:
		// Relayed, not stored. A run result is a moment, not part of the
		// document, so a client joining later gets no snapshot of it — it
		// would be stale by then anyway.
		//
		// The author already rendered its own output, so it is skipped for
		// the same reason as an edit.
		h.broadcast(Message{Type: TypeResult, Text: msg.Text, Failed: msg.Failed}, in.client)

	default:
		// Ignore rather than error: an older or newer client should not be
		// able to kill the room by saying something this build has never
		// heard of.
	}
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

// deliver must never block: the hub is the whole room's single thread of
// control, so one wedged client cannot be allowed to freeze everyone else.
// A client whose buffer is full is dropped instead.
func (h *Hub) deliver(c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
		log.Printf("ws: %s: dropping client that fell behind", h.roomID)
		h.drop(c)
	}
}

// drop removes a client and closes its send channel, which is the signal
// its write pump uses to shut the connection down. Deleting during a range
// over h.clients is safe, and deliver relies on that.
func (h *Hub) drop(c *Client) {
	delete(h.clients, c)
	close(c.send)
	h.presenceDirty = true
}

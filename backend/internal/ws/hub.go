package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"
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
	// prompt is the interview question. Unlike a run result it is room state,
	// so a late joiner gets it in their snapshot rather than missing it.
	prompt string
	// activity is the candidate's running tally. Kept here rather than in the
	// interviewer's browser so that reloading the page does not reset it, and
	// so a second interviewer joining sees the same numbers.
	activity ActivitySummary
	// activityLog is the bounded timeline behind the tally. Ephemeral, like
	// the document: it lives as long as the room and is never written down.
	activityLog []ActivityEvent
	// endsAt is when the interview must stop. Zero means no limit.
	endsAt time.Time
	// ended is set once the deadline has passed, after which the room is
	// read-only: both people can still see it, but nothing more is accepted.
	ended bool

	deadline chan time.Time
	// presenceDirty means the client set changed and the room has not been
	// told yet. Set it instead of broadcasting inline: a drop can happen
	// deep inside a broadcast's range loop, and re-entering broadcast from
	// there would iterate the map while it is being mutated.
	presenceDirty bool

	register     chan *Client
	registerAcks sync.Map
	unregister   chan *Client
	inbound      chan inbound
	// quit is closed by the registry when the last client has left.
	quit chan struct{}

	// onEnded is called once when the interview runs out of time, so the
	// store can record it. A function so this package stays ignorant of
	// Postgres, the same way Authorizer keeps it ignorant of what a token is.
	onEnded func(roomID string)
}

const MaxClientsPerRoom = 100

// NewHub builds a hub for roomID with an empty document. Call Run in its own
// goroutine before registering anyone.
// onEnded may be nil, which is what the tests use.
func NewHub(roomID string, onEnded func(roomID string)) *Hub {
	return &Hub{
		roomID:     roomID,
		onEnded:    onEnded,
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		deadline:   make(chan time.Time, 1),
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
func (h *Hub) Register(c *Client) bool {
	accepted := make(chan bool, 1)
	h.registerAcks.Store(c, accepted)
	h.register <- c
	return <-accepted
}

// SetDeadline tells the hub when the interview must stop.
//
// Non-blocking, because it runs on the HTTP handler's goroutine before the
// client is registered: blocking here would wedge a join behind whatever the
// hub happens to be doing. A dropped repeat is harmless — every client
// carries the same deadline, and the hub keeps the first it is given.
func (h *Hub) SetDeadline(endsAt time.Time) {
	if endsAt.IsZero() {
		return
	}
	select {
	case h.deadline <- endsAt:
	default:
	}
}

// Run owns the room. It returns when the registry stops the hub.
func (h *Hub) Run() {
	// Armed only once a deadline arrives. Stopped rather than left nil so the
	// select below always has a channel to read from.
	expiry := time.NewTimer(time.Hour)
	if !expiry.Stop() {
		<-expiry.C
	}
	defer expiry.Stop()

	for {
		select {
		case <-h.quit:
			return

		case at := <-h.deadline:
			// First one wins. A later join must not be able to extend an
			// interview by reporting a deadline further out.
			if !h.endsAt.IsZero() {
				continue
			}
			h.endsAt = at
			if remaining := time.Until(at); remaining > 0 {
				expiry.Reset(remaining)
			} else {
				// Already over: somebody reopening a finished interview.
				h.expire()
			}

		case <-expiry.C:
			h.expire()

		case c := <-h.register:
			if len(h.clients) >= MaxClientsPerRoom {
				if ack, ok := h.registerAcks.LoadAndDelete(c); ok {
					ack.(chan bool) <- false
				}
				continue
			}
			if ack, ok := h.registerAcks.LoadAndDelete(c); ok {
				ack.(chan bool) <- true
			}
			h.clients[c] = struct{}{}
			// A late joiner needs the current document before anything else,
			// otherwise it would sit on an empty buffer until someone types.
			summary := h.activity
			snap := Message{
				Type:     TypeSnapshot,
				Text:     h.document,
				Prompt:   h.prompt,
				Role:     string(c.role),
				Activity: &summary,
				Ended:    h.ended,
			}
			if !h.endsAt.IsZero() {
				ends := h.endsAt.UnixMilli()
				snap.EndsAt = &ends
			}
			// The log goes to interviewers only. A candidate has no business
			// reading the record being kept about them mid-interview, and it
			// would only distract from the question.
			if c.role == RoleInterviewer {
				snap.Events = append([]ActivityEvent(nil), h.activityLog...)
			}
			h.sendTo(c, snap)
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

// expire ends the interview. The room stays readable, but nothing further is
// accepted and everyone is told.
//
// The clients are deliberately not dropped. Cutting the sockets would leave
// both people staring at a reconnect spinner with no idea why, where a frame
// lets the UI say the interview finished. The room closes on its own once
// they leave.
func (h *Hub) expire() {
	if h.ended {
		return
	}
	h.ended = true
	log.Printf("ws: %s: interview time expired", h.roomID)
	h.broadcast(Message{Type: TypeEnded}, nil)
	if h.onEnded != nil {
		// Off the hub goroutine: this writes to Postgres, and the room must
		// not stop relaying while that happens.
		go h.onEnded(h.roomID)
	}
}

// apply folds one inbound frame into the document.
func (h *Hub) apply(in inbound) {
	var msg Message
	if err := json.Unmarshal(in.data, &msg); err != nil {
		log.Printf("ws: discarding malformed frame: %v", err)
		return
	}

	// Past the deadline the room is read-only. Dropped silently: the client
	// has already been told the interview ended, and an error per keystroke
	// would be noise on top of that.
	if h.ended {
		return
	}
	if len(msg.Text) > maxMessageSize {
		return
	}
	switch msg.Type {
	case TypeEdit:
		h.document = msg.Text
		// The author already has this text locally; echoing it back would
		// fight with their cursor.
		h.broadcast(Message{Type: TypeEdit, Text: msg.Text}, in.client)

	case TypePrompt:
		// Only the interviewer sets the question. A candidate sending this is
		// ignored rather than disconnected: a buggy or malicious client should
		// not be able to end someone's interview, and there is nothing useful
		// the candidate could do about an error anyway.
		if in.client.role != RoleInterviewer {
			log.Printf("ws: %s: ignoring prompt from a %s", h.roomID, in.client.role)
			return
		}
		h.prompt = msg.Prompt
		// The author already has the text they just typed; echoing it back
		// would fight with their cursor, exactly as with an edit.
		h.broadcast(Message{Type: TypePrompt, Prompt: msg.Prompt}, in.client)

	case TypeActivity:
		// Only the candidate is observed. An interviewer switching tabs is
		// their own business, and relaying it would put noise in front of the
		// person who is supposed to be reading the signal.
		if in.client.role != RoleCandidate {
			return
		}
		if msg.Kind != ActivityAway && msg.Kind != ActivityBack && msg.Kind != ActivityPaste {
			return
		}
		if msg.Ms < 0 || msg.Ms > 24*60*60*1000 || msg.Lines < 0 || msg.Lines > 10000 {
			return
		}
		event := h.recordActivity(msg)
		summary := h.activity
		h.broadcast(Message{
			Type:     TypeActivity,
			Kind:     msg.Kind,
			Lines:    msg.Lines,
			Ms:       msg.Ms,
			Activity: &summary,
			Event:    &event,
		}, in.client)

	case TypePointer:
		// Relayed untouched and never stored. Both roles may point; it is a
		// gesture, not a permission. Out-of-range values are dropped rather
		// than clamped, so a bad client cannot park a cursor off-screen where
		// the other person cannot see what is happening.
		if msg.X < 0 || msg.X > 1 || msg.Y < 0 || msg.Y > 1 {
			return
		}
		h.broadcast(Message{Type: TypePointer, X: msg.X, Y: msg.Y, Role: string(in.client.role)}, in.client)

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

// recordActivity folds one reported event into the room's tally.
//
// The client reports how long it was away rather than the hub timing it: the
// hub would have to time from its own clock, and a reconnect in between would
// make that measurement meaningless. A candidate could of course under-report,
// but a candidate who is editing the payload is past what this feature claims
// to catch.
func (h *Hub) recordActivity(msg Message) ActivityEvent {
	event := ActivityEvent{
		Kind: msg.Kind,
		// Stamped here rather than trusting the client: this is a record
		// somebody may rely on, and the browser's clock is not ours.
		At:    time.Now().UnixMilli(),
		Lines: msg.Lines,
		Ms:    msg.Ms,
	}

	if msg.Kind == ActivityPaste {
		event.Text = msg.Text
		if len(event.Text) > MaxPasteChars {
			event.Text = event.Text[:MaxPasteChars]
			event.Truncated = true
		}
	}

	// Oldest out first. Kept as a plain slice rather than a ring buffer
	// because fifty entries copied on overflow is nothing, and a ring would
	// have to be unrolled every time a snapshot is built.
	h.activityLog = append(h.activityLog, event)
	if len(h.activityLog) > MaxActivityEvents {
		h.activityLog = h.activityLog[len(h.activityLog)-MaxActivityEvents:]
	}

	switch msg.Kind {
	case ActivityAway:
		// Guarded so a duplicate "away" — visibilitychange and blur can both
		// fire for one switch — is not counted twice.
		if !h.activity.Away {
			h.activity.Away = true
			h.activity.AwayCount++
		}
	case ActivityBack:
		if h.activity.Away {
			h.activity.Away = false
			if msg.Ms > 0 {
				h.activity.AwayMs += msg.Ms
			}
		}
	case ActivityPaste:
		h.activity.PasteCount++
	}
	return event
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

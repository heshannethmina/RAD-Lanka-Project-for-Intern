package ws

// MessageType tags a frame on the wire. Both directions use the same
// envelope so the client only ever parses one shape.
type MessageType string

const (
	// TypeSnapshot carries the full document. The hub sends one to a client
	// the moment it joins, so a late joiner starts in sync with the room.
	TypeSnapshot MessageType = "snapshot"

	// TypeEdit carries the full document text after an edit. Step 1 of the
	// roadmap syncs raw text rather than operations: the hub goroutine
	// serialises every edit, so "last write wins" has a well-defined meaning
	// without any transformation.
	TypeEdit MessageType = "edit"

	// TypePresence reports how many clients are currently in the room.
	TypePresence MessageType = "presence"
)

// Message is the single frame type exchanged over the socket.
type Message struct {
	Type MessageType `json:"type"`
	Text string      `json:"text,omitempty"`
	// Clients is set on TypePresence only.
	Clients int `json:"clients,omitempty"`
}

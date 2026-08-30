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

	// TypeResult carries the output of a run so the other side of the
	// interview sees it too.
	//
	// The hub relays this without understanding it. Where the output came
	// from — a sandbox, or Python compiled to WebAssembly in the browser —
	// is the client's business, and deliberately not encoded here: the room
	// should not need a new frame type every time execution moves.
	//
	// It is not folded into the document. Run output is not something either
	// person edits, and writing it into the shared text would fight with
	// whoever is typing.
	TypeResult MessageType = "result"
)

// Message is the single frame type exchanged over the socket.
type Message struct {
	Type MessageType `json:"type"`
	Text string      `json:"text,omitempty"`
	// Clients is set on TypePresence only.
	Clients int `json:"clients,omitempty"`
	// Failed marks a run that did not succeed, so the other side can show it
	// the same way the person who pressed Run sees it. Set on TypeResult only.
	Failed bool `json:"failed,omitempty"`
}

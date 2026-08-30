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

	// TypePrompt carries the interview question.
	//
	// Accepted from an interviewer only. A candidate that sends one is
	// ignored: they may answer the question, not rewrite it. Unlike a result,
	// this *is* room state, so the hub keeps it and hands it to late joiners
	// in their snapshot.
	TypePrompt MessageType = "prompt"

	// TypeActivity reports that the candidate left the tab, came back, or
	// pasted into the editor.
	//
	// Detection only. A browser cannot stop someone opening another tab —
	// there is no API for it, by design — and anyone determined can use a
	// phone or a second machine. So this is a signal that somebody stepped
	// away, never proof of anything, and the UI must not present it as more
	// than that.
	//
	// The running totals live in the hub rather than in the interviewer's
	// browser, so reloading the page does not lose the tally.
	TypeActivity MessageType = "activity"
)

// Activity kinds, sent by the candidate's client.
const (
	// ActivityAway means the tab was hidden or the window lost focus.
	ActivityAway = "away"
	// ActivityBack means focus returned. Carries how long they were gone.
	ActivityBack = "back"
	// ActivityPaste means text was pasted into the editor. The single most
	// useful signal here: people switch tabs to read documentation, but a
	// pasted block of code is worth an interviewer's attention.
	ActivityPaste = "paste"
)

// ActivitySummary is the running tally an interviewer sees.
//
// Aggregates, deliberately, rather than a keystroke or mouse-movement stream.
// "Left the tab four times, two minutes total, pasted twice" tells an
// interviewer everything a raw log would, is defensible if a candidate asks
// what was collected, and does not bury the signals that matter in noise.
type ActivitySummary struct {
	AwayCount  int `json:"away_count"`
	AwayMs     int `json:"away_ms"`
	PasteCount int `json:"paste_count"`
	// Away is true while the candidate is currently out of the tab.
	Away bool `json:"away"`
}

// Message is the single frame type exchanged over the socket.
type Message struct {
	Type MessageType `json:"type"`
	Text string      `json:"text,omitempty"`
	// Clients is set on TypePresence only.
	Clients int `json:"clients,omitempty"`
	// Failed marks a run that did not succeed, so the other side can show it
	// the same way the person who pressed Run sees it. Set on TypeResult only.
	Failed bool `json:"failed,omitempty"`
	// Prompt is the interview question. Sent on TypePrompt, and on the
	// TypeSnapshot a client receives when it joins.
	//
	// No omitempty: clearing the question is a real edit, and omitempty would
	// drop the empty string and leave the other side showing the old one.
	Prompt string `json:"prompt"`
	// Role tells a client what it is allowed to do, so the UI can offer an
	// editable question to an interviewer and a read-only one to a candidate.
	// Sent on TypeSnapshot only.
	Role string `json:"role,omitempty"`

	// Kind is which activity happened. Set on TypeActivity only.
	Kind string `json:"kind,omitempty"`
	// Lines is how many lines were pasted, on Kind == ActivityPaste.
	Lines int `json:"lines,omitempty"`
	// Ms is how long the candidate was away, on Kind == ActivityBack.
	Ms int `json:"ms,omitempty"`
	// Activity is the running tally, attached to every TypeActivity the hub
	// sends out and to the snapshot a joiner receives. A pointer so a frame
	// that carries no tally omits the field rather than sending zeroes that
	// would read as "nothing has happened".
	Activity *ActivitySummary `json:"activity,omitempty"`
}

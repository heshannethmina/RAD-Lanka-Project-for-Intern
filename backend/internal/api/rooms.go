package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/auth"
	"github.com/heshannethmina/interview-platform/backend/internal/judge0"
	"github.com/heshannethmina/interview-platform/backend/internal/plan"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// maxRoomBodyBytes bounds a room payload: a title and a language name.
const maxRoomBodyBytes = 4 * 1024

// maxTitleLen keeps a title to something that fits a dashboard row.
const maxTitleLen = 200

// roomListLimit caps a dashboard listing. Pagination can come when somebody
// actually has more than this many interviews.
const roomListLimit = 200

// maxPromptLen bounds the question. Generous — an interviewer may paste a
// whole problem statement — but not unbounded, since it is relayed to everyone
// in the room on every keystroke.
const maxPromptLen = 20_000

type roomJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Language string `json:"language"`
	Prompt   string `json:"prompt"`
	// DurationMinutes is how long it may run once started.
	DurationMinutes int        `json:"duration_minutes"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
	StartedAt       *time.Time `json:"started_at"`
	// EndsAt is when the timer runs out, absolute so the client does not have
	// to agree with us about the current time — only about the deadline.
	EndsAt    *time.Time `json:"ends_at"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	Open      bool       `json:"open"`
}

// endsAt returns the deadline, or nil for a room nobody has opened yet.
func endsAt(r *store.Room) *time.Time {
	e := r.EndsAt()
	if e.IsZero() {
		return nil
	}
	return &e
}

func toRoomJSON(r *store.Room) roomJSON {
	// OwnerID is deliberately absent: it is an internal key, and every route
	// that returns a room has already checked ownership.
	return roomJSON{
		ID:              r.ID,
		Title:           r.Title,
		Language:        r.Language,
		Prompt:          r.Prompt,
		DurationMinutes: int(r.Duration / time.Minute),
		ScheduledAt:     r.ScheduledAt,
		StartedAt:       r.StartedAt,
		EndsAt:          endsAt(r),
		CreatedAt:       r.CreatedAt,
		ClosedAt:        r.ClosedAt,
		Open:            r.Open(),
	}
}

// createdRoomJSON carries the invite token, which is returned exactly once.
//
// Only the hash is stored, so this response is the only time the token
// exists in readable form. That is a deliberate trade: an interviewer who
// loses the link has to rotate it rather than re-read it, and in exchange a
// leaked database grants nobody entry to a live interview. The dashboard
// should make the copy action prominent for exactly that reason.
type createdRoomJSON struct {
	roomJSON
	InviteToken string `json:"invite_token"`
}

// CreateRoom opens a new interview owned by the signed-in interviewer.
func CreateRoom(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		var req struct {
			Title    string `json:"title"`
			Language string `json:"language"`
			// Minutes the interview may run. Clamped to the plan; zero means
			// "give me whatever the plan allows".
			DurationMinutes int `json:"duration_minutes"`
			// When the interviewer means to hold it. Advisory — the timer runs
			// from the first join, not from this.
			ScheduledAt *time.Time `json:"scheduled_at"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRoomBodyBytes)
		// An empty body is a valid "create me a room with the defaults", so
		// only a malformed one is an error.
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}

		req.Title = strings.TrimSpace(req.Title)
		if len(req.Title) > maxTitleLen {
			writeError(w, http.StatusBadRequest, "title is too long")
			return
		}
		if req.Language == "" {
			req.Language = "python"
		}
		// Validate against our own list, for the same reason /api/run does:
		// the room's language decides what gets submitted to the sandbox.
		if !judge0.Supported(req.Language) {
			writeError(w, http.StatusBadRequest, "unsupported language: "+req.Language)
			return
		}

		// The allowance check. Counted from the rooms table rather than a
		// separate counter, because the rooms are the record.
		tier := plan.ByName(u.Plan)
		if !tier.UnlimitedInterviews() {
			used, err := s.CountRooms(r.Context(), u.ID, tier.Lifetime)
			if err != nil {
				log.Printf("api: count rooms: %v", err)
				writeError(w, http.StatusInternalServerError, "could not check your plan")
				return
			}
			if used >= tier.MaxInterviews {
				// 402 rather than 403: this is not a permission problem, it is
				// an exhausted allowance, and the difference tells the client
				// whether to offer an upgrade or an apology.
				writeError(w, http.StatusPaymentRequired, planLimitMessage(tier))
				return
			}
		}

		// Clamped rather than rejected: somebody on Free who asks for an hour
		// wants an interview, and ten minutes with the limit shown beats an
		// error telling them to try again with a smaller number.
		duration := tier.ClampDuration(time.Duration(req.DurationMinutes) * time.Minute)

		id, err := auth.NewRoomID()
		if err != nil {
			log.Printf("api: room id: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create the room")
			return
		}
		invite, inviteHash, err := auth.NewToken()
		if err != nil {
			log.Printf("api: invite token: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create the room")
			return
		}

		room, err := s.CreateRoom(r.Context(), store.NewRoom{
			ID:              id,
			OwnerID:         u.ID,
			Title:           req.Title,
			Language:        req.Language,
			ScheduledAt:     req.ScheduledAt,
			Duration:        duration,
			InviteTokenHash: inviteHash,
		})
		if err != nil {
			log.Printf("api: create room: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create the room")
			return
		}

		writeJSON(w, http.StatusCreated, createdRoomJSON{
			roomJSON:    toRoomJSON(room),
			InviteToken: invite,
		})
	}
}

// ListRooms returns the signed-in interviewer's rooms, newest first.
func ListRooms(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		rooms, err := s.RoomsByOwner(r.Context(), u.ID, roomListLimit)
		if err != nil {
			log.Printf("api: list rooms: %v", err)
			writeError(w, http.StatusInternalServerError, "could not list your rooms")
			return
		}

		out := make([]roomJSON, 0, len(rooms))
		for i := range rooms {
			out = append(out, toRoomJSON(&rooms[i]))
		}
		writeJSON(w, http.StatusOK, map[string]any{"rooms": out})
	}
}

// GetRoom returns one of the interviewer's own rooms.
func GetRoom(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		room, ok := ownedRoom(w, r, s)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, toRoomJSON(room))
	}
}

// CloseRoom ends an interview. The room is kept — it is the record that the
// interview happened — but it stops accepting joins.
func CloseRoom(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		err := s.CloseRoom(r.Context(), r.PathValue("roomID"), u.ID)
		if errors.Is(err, store.ErrNotFound) {
			// Covers "no such room", "not yours" and "already closed" with
			// one answer, so this cannot be used to probe for rooms.
			writeError(w, http.StatusNotFound, "no such open room")
			return
		}
		if err != nil {
			log.Printf("api: close room: %v", err)
			writeError(w, http.StatusInternalServerError, "could not close the room")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RotateInvite issues a new candidate link and revokes the previous one.
func RotateInvite(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		room, ok := ownedRoom(w, r, s)
		if !ok {
			return
		}
		u, _ := UserFrom(r.Context())

		invite, inviteHash, err := auth.NewToken()
		if err != nil {
			log.Printf("api: invite token: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create a link")
			return
		}
		if err := s.RotateInvite(r.Context(), room.ID, u.ID, inviteHash); err != nil {
			log.Printf("api: rotate invite: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create a link")
			return
		}

		writeJSON(w, http.StatusOK, createdRoomJSON{
			roomJSON:    toRoomJSON(room),
			InviteToken: invite,
		})
	}
}

// JoinRoom is the candidate's side of a shareable link, and the only room
// route that does not require an account.
//
// It reports whether a link is good and what the room is set up to run, so the
// page can render the right editor before opening a socket. The socket
// re-checks the token itself: this endpoint is a courtesy, not the gate.
func JoinRoom(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.PathValue("roomID")
		token := r.URL.Query().Get("token")
		if token == "" {
			writeError(w, http.StatusUnauthorized, "this link is missing its token")
			return
		}

		room, err := s.RoomByInvite(r.Context(), roomID, auth.HashToken(token))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "this link is not valid, or the interview has ended")
			return
		}
		if err != nil {
			log.Printf("api: join room: %v", err)
			writeError(w, http.StatusInternalServerError, "could not open the room")
			return
		}
		writeJSON(w, http.StatusOK, toRoomJSON(room))
	}
}

// UpdatePrompt saves the interview question.
//
// The WebSocket already relays prompt edits live, so this is the durable half:
// the relay is what the candidate sees immediately, this is what survives a
// reload. Both enforce the same rule — only the owner may change it.
func UpdatePrompt(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		var req struct {
			Prompt string `json:"prompt"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxPromptLen+4*1024)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}
		if len(req.Prompt) > maxPromptLen {
			writeError(w, http.StatusBadRequest, "the question is too long")
			return
		}

		err := s.UpdatePrompt(r.Context(), r.PathValue("roomID"), u.ID, req.Prompt)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no such room")
			return
		}
		if err != nil {
			log.Printf("api: update prompt: %v", err)
			writeError(w, http.StatusInternalServerError, "could not save the question")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// planLimitMessage says what ran out and what to do, rather than "limit
// reached" — somebody who has just been refused needs the number and the way
// forward in the same sentence.
func planLimitMessage(tier plan.Plan) string {
	if tier.Lifetime {
		return fmt.Sprintf(
			"The %s plan includes %d interviews in total, and you have used them. Upgrade to run more.",
			tier.Label, tier.MaxInterviews,
		)
	}
	return fmt.Sprintf(
		"The %s plan includes %d interviews a month, and you have used them. They reset at the start of next month.",
		tier.Label, tier.MaxInterviews,
	)
}

// ownedRoom resolves {roomID} and confirms the signed-in interviewer owns it.
//
// "Not yours" and "does not exist" both answer 404 on purpose: a 403 would
// confirm that a room ID is real, which is exactly what someone guessing IDs
// wants to learn.
func ownedRoom(w http.ResponseWriter, r *http.Request, s *store.Store) (*store.Room, bool) {
	u, ok := UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return nil, false
	}

	room, err := s.RoomByID(r.Context(), r.PathValue("roomID"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such room")
		return nil, false
	}
	if err != nil {
		log.Printf("api: room by id: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load the room")
		return nil, false
	}
	if room.OwnerID != u.ID {
		writeError(w, http.StatusNotFound, "no such room")
		return nil, false
	}
	return room, true
}

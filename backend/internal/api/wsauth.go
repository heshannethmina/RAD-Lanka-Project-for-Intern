package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/heshannethmina/interview-platform/backend/internal/auth"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
	"github.com/heshannethmina/interview-platform/backend/internal/ws"
)

// RoomAuthorizer admits the two kinds of participant an interview has.
//
// There is no third kind, and in particular no unauthenticated join: before
// this existed, anyone who guessed a room ID was in.
//
//	interviewer — a live session token, and they must own the room
//	candidate   — the room's invite token, and the room must still be open
//
// The two are tried in that order against one opaque token, because the client
// has no reason to tell us which it holds: the room page passes whatever it
// has, and the candidate page only ever has an invite.
func RoomAuthorizer(s *store.Store) ws.Authorizer {
	return func(ctx context.Context, roomID, token string) (ws.Grant, error) {
		if token == "" {
			return ws.Grant{}, ws.ErrUnauthorized
		}
		hash := auth.HashToken(token)

		// Interviewer. A session token is not a skeleton key: it admits its
		// holder to their own rooms only, so one interviewer cannot open
		// another's interview by knowing its ID.
		u, err := s.UserBySessionToken(ctx, hash)
		switch {
		case err == nil:
			room, err := s.RoomByID(ctx, roomID)
			if errors.Is(err, store.ErrNotFound) {
				return ws.Grant{}, ws.ErrUnauthorized
			}
			if err != nil {
				return ws.Grant{}, fmt.Errorf("authorize: room by id: %w", err)
			}
			if room.OwnerID != u.ID {
				return ws.Grant{}, ws.ErrUnauthorized
			}
			// An owner may rejoin a room they have closed — to re-read it, or
			// because they closed it by accident. A candidate may not.
			//
			// The clock is not started for them: an interviewer opening the
			// room early to set the question should not burn the candidate's
			// time. It starts when the candidate arrives.
			return ws.Grant{Role: ws.RoleInterviewer, EndsAt: room.EndsAt()}, nil

		case errors.Is(err, store.ErrNotFound):
			// Not a session token; fall through and try it as an invite.

		default:
			return ws.Grant{}, fmt.Errorf("authorize: session lookup: %w", err)
		}

		// Candidate. RoomByInvite checks the token against this room and that
		// the room is open, in one query, so a token for another room cannot
		// be replayed here.
		room, err := s.RoomByInvite(ctx, roomID, hash)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return ws.Grant{}, ws.ErrUnauthorized
			}
			return ws.Grant{}, fmt.Errorf("authorize: invite lookup: %w", err)
		}

		// The candidate arriving is what starts the interview. Stamped here
		// rather than at creation so a room booked on Monday and held on
		// Friday gets its full time, and an interviewer setting up early does
		// not eat into it.
		started, err := s.StartRoom(ctx, roomID)
		if err != nil {
			return ws.Grant{}, fmt.Errorf("authorize: start room: %w", err)
		}
		_ = room

		// A candidate cannot rejoin an interview whose time has run out.
		if started.Expired() {
			return ws.Grant{}, ws.ErrUnauthorized
		}
		return ws.Grant{Role: ws.RoleCandidate, EndsAt: started.EndsAt()}, nil
	}
}

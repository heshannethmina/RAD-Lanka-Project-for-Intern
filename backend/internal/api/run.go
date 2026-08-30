// Package api holds the REST handlers. Real-time editor traffic lives in
// package ws; this is everything else.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/heshannethmina/interview-platform/backend/internal/judge0"
	"github.com/heshannethmina/interview-platform/backend/internal/ws"
)

// runRequest is what the room page posts when someone presses Run.
type runRequest struct {
	Language string `json:"language"`
	Source   string `json:"source"`
	RoomID   string `json:"room_id"`
	Token    string `json:"token"`
}

// errorResponse keeps failures in the same shape as successes, so the client
// has one thing to parse.
type errorResponse struct {
	Error string `json:"error"`
}

// maxBodyBytes bounds the request itself. judge0.MaxSourceBytes bounds the
// source within it; this leaves room for the JSON envelope around it.
const maxBodyBytes = judge0.MaxSourceBytes + 4*1024

// Run proxies a run request to Judge0.
//
// The Go process never executes anything itself — it forwards to a sandbox
// that runs in a separate container, which is the whole reason Judge0 is
// here rather than an exec.Command.
func Run(client *judge0.Client, authorizers ...ws.Authorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

		var req runRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}

		// Check the language here rather than letting Judge0 reject it, so a
		// client can never pick a language ID we did not intend to expose.
		if !judge0.Supported(req.Language) {
			writeError(w, http.StatusBadRequest, "unsupported language: "+req.Language)
			return
		}
		if req.Source == "" {
			writeError(w, http.StatusBadRequest, "no source to run")
			return
		}
		if len(req.Source) > judge0.MaxSourceBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "source too large")
			return
		}
		if len(authorizers) > 0 {
			grant, err := authorizers[0](r.Context(), req.RoomID, req.Token)
			if err != nil || !grant.CanRun {
				writeError(w, http.StatusUnauthorized, "room access required")
				return
			}
			if grant.OnAccepted != nil {
				if err := grant.OnAccepted(r.Context()); err != nil {
					writeError(w, http.StatusForbidden, "room is not available")
					return
				}
			}
		}

		result, err := client.Run(r.Context(), req.Language, req.Source)
		if err != nil {
			switch {
			case errors.Is(err, judge0.ErrUnsupportedLanguage):
				writeError(w, http.StatusBadRequest, "unsupported language")
			case errors.Is(err, judge0.ErrSourceTooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, "source too large")
			case errors.Is(err, r.Context().Err()) && r.Context().Err() != nil:
				// The client navigated away mid-run; nothing to report to.
				log.Printf("api: run cancelled: %v", err)
			default:
				// Do not hand the caller the internals of an execution
				// service failure; log it and say something useful instead.
				log.Printf("api: run failed: %v", err)
				writeError(w, http.StatusBadGateway, "could not reach the execution service")
			}
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("api: write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

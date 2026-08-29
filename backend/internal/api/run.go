// Package api holds the REST handlers. Real-time editor traffic lives in
// package ws; this is everything else.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/heshannethmina/interview-platform/backend/internal/judge0"
)

// runRequest is what the room page posts when someone presses Run.
type runRequest struct {
	Language string `json:"language"`
	Source   string `json:"source"`
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
func Run(client *judge0.Client) http.HandlerFunc {
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

// Package judge0 talks to a Judge0 instance, which is where all untrusted
// code runs. Nothing in this package executes anything itself: it submits
// source to a separate sandboxed service and reads the verdict back.
package judge0

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Judge0 language IDs, as shipped by judge0 1.13.0. They are pinned to the
// image tag in docker-compose.yml; if that tag moves, check these against
// `GET /languages` before trusting them.
var languageIDs = map[string]int{
	"python":     71, // Python 3.8.1
	"go":         60, // Go 1.13.5
	"javascript": 63, // JavaScript (Node.js 12.14.0)
}

// MaxSourceBytes bounds what we are willing to forward. An interview answer
// is a few hundred lines; anything larger is a mistake or an attack.
const MaxSourceBytes = 64 * 1024

var (
	ErrUnsupportedLanguage = errors.New("judge0: unsupported language")
	ErrSourceTooLarge      = errors.New("judge0: source too large")
)

// Supported reports whether a language can be run, without exposing the
// Judge0 ID space to callers.
func Supported(language string) bool {
	_, ok := languageIDs[language]
	return ok
}

// Result is the normalised outcome of one run. Judge0's own shape leaks
// nulls and stringly-typed numbers; this does not.
type Result struct {
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	CompileOutput string `json:"compile_output"`
	// Status is Judge0's human-readable verdict: "Accepted", "Runtime Error
	// (NZEC)", "Time Limit Exceeded", and so on.
	Status string `json:"status"`
	Time   string `json:"time"`
	Memory int    `json:"memory"`
}

// Client is a Judge0 HTTP client. The zero value is not usable; use New.
type Client struct {
	baseURL string
	http    *http.Client

	// pollInterval is how often we ask whether a submission has finished, and
	// maxWait caps the whole run. Both are fields so tests can shrink them.
	pollInterval time.Duration
	maxWait      time.Duration
}

func New(baseURL string) *Client {
	return &Client{
		baseURL:      baseURL,
		http:         &http.Client{Timeout: 15 * time.Second},
		pollInterval: 250 * time.Millisecond,
		maxWait:      20 * time.Second,
	}
}

type submitRequest struct {
	SourceCode    string  `json:"source_code"`
	LanguageID    int     `json:"language_id"`
	Stdin         string  `json:"stdin,omitempty"`
	CPUTimeLimit  float64 `json:"cpu_time_limit,omitempty"`
	WallTimeLimit float64 `json:"wall_time_limit,omitempty"`
}

type submitResponse struct {
	Token string `json:"token"`
}

// submission is Judge0's representation. The pointers matter: it returns
// JSON null for every field that does not apply to a given verdict.
type submission struct {
	Stdout        *string `json:"stdout"`
	Stderr        *string `json:"stderr"`
	CompileOutput *string `json:"compile_output"`
	Message       *string `json:"message"`
	Time          *string `json:"time"`
	Memory        *int    `json:"memory"`
	Status        struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"status"`
}

// Judge0 status IDs 1 and 2 mean queued and running; everything above is a
// final verdict.
const (
	statusInQueue    = 1
	statusProcessing = 2
)

// Run submits source and blocks until Judge0 reaches a verdict, the context
// is cancelled, or maxWait elapses.
func (c *Client) Run(ctx context.Context, language, source string) (*Result, error) {
	id, ok := languageIDs[language]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, language)
	}
	if len(source) > MaxSourceBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrSourceTooLarge, len(source))
	}

	ctx, cancel := context.WithTimeout(ctx, c.maxWait)
	defer cancel()

	token, err := c.submit(ctx, id, source)
	if err != nil {
		return nil, err
	}
	return c.poll(ctx, token)
}

func (c *Client) submit(ctx context.Context, languageID int, source string) (string, error) {
	body, err := json.Marshal(submitRequest{
		SourceCode: source,
		LanguageID: languageID,
		// Judge0 enforces these inside the sandbox. They are a second line of
		// defence; the sandbox itself is the first.
		CPUTimeLimit:  5,
		WallTimeLimit: 10,
	})
	if err != nil {
		return "", fmt.Errorf("judge0: encode submission: %w", err)
	}

	// wait=false: Judge0's synchronous mode is documented as unreliable under
	// load, so we submit and poll instead.
	url := c.baseURL + "/submissions?base64_encoded=false&wait=false"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("judge0: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("judge0: submit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("judge0: submit: %s: %s", resp.Status, peek(resp.Body))
	}

	var out submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("judge0: decode submission: %w", err)
	}
	if out.Token == "" {
		return "", errors.New("judge0: submission returned no token")
	}
	return out.Token, nil
}

func (c *Client) poll(ctx context.Context, token string) (*Result, error) {
	url := c.baseURL + "/submissions/" + token + "?base64_encoded=false"

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		sub, err := c.fetch(ctx, url)
		if err != nil {
			return nil, err
		}
		if sub.Status.ID != statusInQueue && sub.Status.ID != statusProcessing {
			return toResult(sub), nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("judge0: waiting for verdict: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *Client) fetch(ctx context.Context, url string) (*submission, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("judge0: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("judge0: fetch submission: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("judge0: fetch submission: %s: %s", resp.Status, peek(resp.Body))
	}

	var sub submission
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return nil, fmt.Errorf("judge0: decode submission: %w", err)
	}
	return &sub, nil
}

func toResult(sub *submission) *Result {
	r := &Result{
		Stdout:        deref(sub.Stdout),
		Stderr:        deref(sub.Stderr),
		CompileOutput: deref(sub.CompileOutput),
		Status:        sub.Status.Description,
		Time:          deref(sub.Time),
	}
	if sub.Memory != nil {
		r.Memory = *sub.Memory
	}
	// Judge0 puts sandbox-level failures (out of memory, killed) in Message
	// rather than Stderr, and a run that says nothing at all is baffling.
	if r.Stderr == "" && sub.Message != nil {
		r.Stderr = *sub.Message
	}
	return r
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// peek reads a bounded prefix of an error body, for log and error messages.
func peek(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 512))
	return string(bytes.TrimSpace(b))
}

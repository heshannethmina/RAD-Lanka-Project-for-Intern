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
	"strings"
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

	// headers is sent with every request. A self-hosted Judge0 on localhost
	// needs none; a hosted one needs whatever it uses for auth.
	headers map[string]string

	// pollInterval is how often we ask whether a submission has finished, and
	// maxWait caps the whole run. Both are fields so tests can shrink them.
	pollInterval time.Duration
	maxWait      time.Duration
}

// Option configures a Client. Functional options rather than a wider New, so
// that adding one later does not break every call site again.
type Option func(*Client)

// WithHeader adds a header sent on every request to Judge0.
//
// This is how a hosted instance is authenticated, and which header to use
// depends on who is hosting it:
//
//	RapidAPI      X-RapidAPI-Key plus X-RapidAPI-Host
//	Judge0 Cloud  X-Auth-Token
//	self-hosted   none, unless AUTHN_TOKEN is set in judge0.conf
//
// Nothing here knows or cares which; the header names come from configuration
// so a new provider needs no code change.
func WithHeader(name, value string) Option {
	return func(c *Client) {
		if name == "" || value == "" {
			// Silently dropping an empty pair keeps callers from having to
			// guard every optional variable before passing it in.
			return
		}
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers[name] = value
	}
}

func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:      baseURL,
		http:         &http.Client{Timeout: 15 * time.Second},
		pollInterval: 250 * time.Millisecond,
		maxWait:      20 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ParseHeaders reads the JUDGE0_HEADERS configuration format: comma-separated
// "Name: Value" pairs.
//
//	X-RapidAPI-Key: abc123, X-RapidAPI-Host: judge0-ce.p.rapidapi.com
//
// One variable rather than a pair per provider, because the set of headers
// differs between them and hard-coding vendor names here would mean editing Go
// to change hosting.
//
// Only the first colon splits, so a value may contain one; a value may not
// contain a comma. No token format in use does, and the alternative — a
// quoting scheme in an environment variable — is worse than the limitation.
//
// The returned error never quotes the value, because it holds a secret.
func ParseHeaders(s string) ([]Option, error) {
	var opts []Option
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, value, found := strings.Cut(pair, ":")
		if !found {
			return nil, fmt.Errorf("judge0: header %d is missing a colon", len(opts)+1)
		}
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if name == "" || value == "" {
			return nil, fmt.Errorf("judge0: header %q has an empty name or value", name)
		}
		opts = append(opts, WithHeader(name, value))
	}
	return opts, nil
}

// newRequest builds a request carrying the configured headers.
//
// Both the submit and the poll path go through here. Authenticating only the
// submit would fail confusingly: the submission would be accepted and every
// poll for its verdict rejected.
func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("judge0: build request: %w", err)
	}
	for name, value := range c.headers {
		req.Header.Set(name, value)
	}
	return req, nil
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
	req, err := c.newRequest(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
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
	req, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
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

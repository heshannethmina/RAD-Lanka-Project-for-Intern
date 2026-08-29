package judge0

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeJudge0 stands in for the real service. It hands out a token, then
// reports "Processing" for the first `queued` polls before settling on the
// given verdict — which is the sequence the client must cope with.
type fakeJudge0 struct {
	queued  int32
	final   submission
	submits int32
	polls   int32
	// lastBody captures what the client actually sent.
	lastBody submitRequest
}

func (f *fakeJudge0) server(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /submissions", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&f.submits, 1)
		if err := json.NewDecoder(r.Body).Decode(&f.lastBody); err != nil {
			t.Errorf("fake judge0: decode: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(submitResponse{Token: "tok-123"})
	})
	mux.HandleFunc("GET /submissions/{token}", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&f.polls, 1)
		if n <= atomic.LoadInt32(&f.queued) {
			var pending submission
			pending.Status.ID = statusProcessing
			pending.Status.Description = "Processing"
			_ = json.NewEncoder(w).Encode(pending)
			return
		}
		_ = json.NewEncoder(w).Encode(f.final)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, baseURL string) *Client {
	t.Helper()

	c := New(baseURL)
	// Keep the tests fast; the production defaults are much longer.
	c.pollInterval = time.Millisecond
	c.maxWait = 2 * time.Second
	return c
}

func str(s string) *string { return &s }
func num(n int) *int       { return &n }

func TestRunReturnsOutput(t *testing.T) {
	fake := &fakeJudge0{queued: 2}
	fake.final.Stdout = str("22\n")
	fake.final.Time = str("0.012")
	fake.final.Memory = num(3456)
	fake.final.Status.ID = 3
	fake.final.Status.Description = "Accepted"

	client := newClient(t, fake.server(t).URL)

	got, err := client.Run(context.Background(), "python", "print(22)")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Stdout != "22\n" {
		t.Errorf("stdout = %q, want %q", got.Stdout, "22\n")
	}
	if got.Status != "Accepted" {
		t.Errorf("status = %q, want Accepted", got.Status)
	}
	if got.Memory != 3456 {
		t.Errorf("memory = %d, want 3456", got.Memory)
	}
	// It must have kept polling past the queued responses rather than
	// reporting "Processing" as a verdict.
	if fake.polls < 3 {
		t.Errorf("polled %d times, want at least 3", fake.polls)
	}
}

// The language name must be translated to Judge0's numeric ID, and the
// sandbox limits must actually be sent.
func TestRunSendsLanguageIDAndLimits(t *testing.T) {
	fake := &fakeJudge0{}
	fake.final.Status.ID = 3
	fake.final.Status.Description = "Accepted"

	client := newClient(t, fake.server(t).URL)
	if _, err := client.Run(context.Background(), "go", "package main"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if fake.lastBody.LanguageID != languageIDs["go"] {
		t.Errorf("language_id = %d, want %d", fake.lastBody.LanguageID, languageIDs["go"])
	}
	if fake.lastBody.SourceCode != "package main" {
		t.Errorf("source_code = %q", fake.lastBody.SourceCode)
	}
	if fake.lastBody.CPUTimeLimit == 0 || fake.lastBody.WallTimeLimit == 0 {
		t.Error("submission carried no cpu/wall time limit")
	}
}

func TestRunSurfacesCompileError(t *testing.T) {
	fake := &fakeJudge0{}
	fake.final.CompileOutput = str("./main.go:3:1: syntax error")
	fake.final.Status.ID = 6
	fake.final.Status.Description = "Compilation Error"

	client := newClient(t, fake.server(t).URL)

	got, err := client.Run(context.Background(), "go", "func main( {")
	if err != nil {
		t.Fatalf("a compile error is a verdict, not a transport failure: %v", err)
	}
	if !strings.Contains(got.CompileOutput, "syntax error") {
		t.Errorf("compile_output = %q", got.CompileOutput)
	}
	if got.Status != "Compilation Error" {
		t.Errorf("status = %q", got.Status)
	}
}

// Judge0 reports sandbox-level failures in Message, not Stderr. A run that
// says nothing at all would be baffling to whoever pressed Run.
func TestRunFallsBackToMessageWhenStderrEmpty(t *testing.T) {
	fake := &fakeJudge0{}
	fake.final.Message = str("Exited with error status 137")
	fake.final.Status.ID = 11
	fake.final.Status.Description = "Runtime Error (SIGKILL)"

	client := newClient(t, fake.server(t).URL)

	got, err := client.Run(context.Background(), "python", "x=1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Stderr != "Exited with error status 137" {
		t.Errorf("stderr = %q, want the message to be surfaced", got.Stderr)
	}
}

func TestRunRejectsUnsupportedLanguage(t *testing.T) {
	fake := &fakeJudge0{}
	client := newClient(t, fake.server(t).URL)

	_, err := client.Run(context.Background(), "brainfuck", "+++")
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Fatalf("err = %v, want ErrUnsupportedLanguage", err)
	}
	if fake.submits != 0 {
		t.Error("an unsupported language must be rejected before hitting judge0")
	}
}

func TestRunRejectsOversizedSource(t *testing.T) {
	fake := &fakeJudge0{}
	client := newClient(t, fake.server(t).URL)

	_, err := client.Run(context.Background(), "python", strings.Repeat("x", MaxSourceBytes+1))
	if !errors.Is(err, ErrSourceTooLarge) {
		t.Fatalf("err = %v, want ErrSourceTooLarge", err)
	}
	if fake.submits != 0 {
		t.Error("oversized source must be rejected before hitting judge0")
	}
}

func TestRunReportsSubmitFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "queue is full", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := newClient(t, srv.URL)

	_, err := client.Run(context.Background(), "python", "print(1)")
	if err == nil {
		t.Fatal("a 503 from judge0 must be an error")
	}
	if !strings.Contains(err.Error(), "queue is full") {
		t.Errorf("err = %v, want it to carry judge0's reason", err)
	}
}

// A submission that never leaves the queue must time out rather than poll
// forever.
func TestRunGivesUpOnStuckSubmission(t *testing.T) {
	fake := &fakeJudge0{queued: 1 << 30}
	client := newClient(t, fake.server(t).URL)
	client.maxWait = 150 * time.Millisecond

	start := time.Now()
	_, err := client.Run(context.Background(), "python", "while True: pass")
	if err == nil {
		t.Fatal("a stuck submission must fail, not hang")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want a deadline error", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("gave up after %v, want roughly maxWait", elapsed)
	}
}

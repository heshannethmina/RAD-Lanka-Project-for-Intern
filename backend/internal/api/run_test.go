package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heshannethmina/interview-platform/backend/internal/judge0"
)

// fakeJudge0 answers the two endpoints the client uses, settling immediately
// on the supplied verdict.
func fakeJudge0(t *testing.T, stdout, status string) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /submissions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"tok"}`))
	})
	mux.HandleFunc("GET /submissions/{token}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"stdout":` + jsonString(stdout) +
			`,"stderr":null,"compile_output":null,"time":"0.01","memory":2048` +
			`,"status":{"id":3,"description":` + jsonString(status) + `}}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRunReturnsResult(t *testing.T) {
	h := Run(judge0.New(fakeJudge0(t, "22\n", "Accepted")))

	rec := post(t, h, `{"language":"python","source":"print(22)"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	var got judge0.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Stdout != "22\n" || got.Status != "Accepted" {
		t.Fatalf("got %+v", got)
	}
}

// The set of runnable languages is decided here, not by Judge0 — a client
// must not be able to reach a language ID we did not choose to expose.
func TestRunRejectsUnsupportedLanguage(t *testing.T) {
	h := Run(judge0.New(fakeJudge0(t, "", "Accepted")))

	rec := post(t, h, `{"language":"ruby","source":"puts 1"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unsupported language") {
		t.Errorf("body = %s", rec.Body)
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	h := Run(judge0.New(fakeJudge0(t, "", "Accepted")))

	for name, body := range map[string]string{
		"malformed json": `{"language":`,
		"empty source":   `{"language":"python","source":""}`,
		"no language":    `{"source":"print(1)"}`,
	} {
		rec := post(t, h, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
	}
}

// Judge0 being down is not the caller's fault, and its internals are not the
// caller's business.
func TestRunReportsExecutionServiceFailureAsBadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "judge0 internal detail", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	h := Run(judge0.New(srv.URL))
	rec := post(t, h, `{"language":"python","source":"print(1)"}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "judge0 internal detail") {
		t.Error("the execution service's internals leaked to the client")
	}
}

func TestRunRejectsOversizedBody(t *testing.T) {
	h := Run(judge0.New(fakeJudge0(t, "", "Accepted")))

	huge := strings.Repeat("x", judge0.MaxSourceBytes+8*1024)
	rec := post(t, h, `{"language":"python","source":"`+huge+`"}`)

	// MaxBytesReader trips first and surfaces as a decode failure; either way
	// the oversized request must not reach Judge0.
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 400 or 413", rec.Code)
	}
}

func TestCORSAnswersPreflight(t *testing.T) {
	h := CORS("*", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("preflight must not reach the wrapped handler")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/run", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin = %q", got)
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type") {
		t.Error("preflight did not allow the Content-Type header")
	}
}

func TestCORSPassesThroughRealRequests(t *testing.T) {
	called := false
	h := CORS("http://localhost:3000", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("request did not reach the wrapped handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("allow-origin = %q", got)
	}
}

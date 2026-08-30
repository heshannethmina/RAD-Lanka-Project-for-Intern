package judge0

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"only whitespace", "   ", map[string]string{}},
		{"single", "X-Auth-Token: secret", map[string]string{"X-Auth-Token": "secret"}},
		{"no space after colon", "X-Auth-Token:secret", map[string]string{"X-Auth-Token": "secret"}},
		{
			// The RapidAPI case, which is the reason this takes a list rather
			// than one name and one value.
			"two pairs",
			"X-RapidAPI-Key: abc123, X-RapidAPI-Host: judge0-ce.p.rapidapi.com",
			map[string]string{
				"X-RapidAPI-Key":  "abc123",
				"X-RapidAPI-Host": "judge0-ce.p.rapidapi.com",
			},
		},
		{"trailing comma", "X-Auth-Token: secret,", map[string]string{"X-Auth-Token": "secret"}},
		{
			// Only the first colon splits, so a value may contain one.
			"value containing a colon",
			"X-Origin: https://example.com:8443",
			map[string]string{"X-Origin": "https://example.com:8443"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := ParseHeaders(tt.input)
			if err != nil {
				t.Fatalf("ParseHeaders(%q): %v", tt.input, err)
			}

			c := New("http://example.invalid", opts...)
			if len(c.headers) != len(tt.want) {
				t.Fatalf("got %d headers, want %d: %v", len(c.headers), len(tt.want), c.headers)
			}
			for name, want := range tt.want {
				if got := c.headers[name]; got != want {
					t.Errorf("header %q = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestParseHeadersRejectsMalformed(t *testing.T) {
	for _, input := range []string{
		"no-colon-here",
		": empty-name",
		"Empty-Value:",
		"X-Ok: fine, broken",
	} {
		if _, err := ParseHeaders(input); err == nil {
			t.Errorf("ParseHeaders(%q) accepted a malformed value", input)
		}
	}
}

// A malformed JUDGE0_HEADERS is a configuration mistake, and the operator has
// to see which pair is wrong — but the value is a credential, so the message
// must not repeat it.
func TestParseHeadersErrorDoesNotLeakTheSecret(t *testing.T) {
	const secret = "sk-do-not-log-me"
	_, err := ParseHeaders("X-Ok: fine, " + secret)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error message leaked the credential: %v", err)
	}
}

// The submit and the poll must both be authenticated. Sending the header on
// only one of them fails in a way that is very hard to read: the submission is
// accepted, then every poll for its verdict is rejected.
func TestConfiguredHeadersAreSentOnEveryRequest(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]string{}

	record := func(path string, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		seen[path] = append(seen[path], r.Header.Get("X-RapidAPI-Key"))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /submissions", func(w http.ResponseWriter, r *http.Request) {
		record("submit", r)
		if r.Header.Get("X-RapidAPI-Host") == "" {
			t.Error("submit: X-RapidAPI-Host was not sent")
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(submitResponse{Token: "tok-123"})
	})
	mux.HandleFunc("GET /submissions/{token}", func(w http.ResponseWriter, r *http.Request) {
		record("poll", r)
		var done submission
		done.Status.ID = 3
		done.Status.Description = "Accepted"
		done.Stdout = str("22\n")
		_ = json.NewEncoder(w).Encode(done)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	opts, err := ParseHeaders("X-RapidAPI-Key: abc123, X-RapidAPI-Host: judge0-ce.p.rapidapi.com")
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}
	c := New(srv.URL, opts...)
	c.pollInterval = time.Millisecond
	c.maxWait = 2 * time.Second

	if _, err := c.Run(context.Background(), "python", "print(1)"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"submit", "poll"} {
		got := seen[path]
		if len(got) == 0 {
			t.Fatalf("%s was never called", path)
		}
		for i, key := range got {
			if key != "abc123" {
				t.Errorf("%s request %d sent X-RapidAPI-Key %q, want %q", path, i, key, "abc123")
			}
		}
	}
}

// A local instance needs no auth, and configuring none must not add stray
// headers — an unexpected header is a way to get rejected by a proxy.
func TestNoHeadersConfiguredSendsNone(t *testing.T) {
	c := New("http://example.invalid")
	if len(c.headers) != 0 {
		t.Fatalf("got %d headers on a bare client, want 0: %v", len(c.headers), c.headers)
	}
}

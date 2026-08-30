package api

import (
	"net/http"
	"strings"
)

// CORS allows the browser app, which is served from a different origin, to
// call this API.
//
// WebSocket upgrades are not subject to CORS, which is why the editor worked
// without this; fetch() from the room page is.
//
// origins is a comma-separated allowlist. "*" is still accepted and still
// means "anyone", which is only defensible in local development — the API now
// carries bearer tokens, so a wildcard in production would let any page a
// logged-in interviewer visits call this API with their token if it ever
// obtained one. Set CORS_ORIGIN to the deployed web origin, and narrow
// ws.CheckOrigin at the same time.
//
// Credentials are deliberately not enabled: the session token travels in an
// Authorization header, not a cookie, so the browser never needs to attach
// ambient credentials and CSRF does not arise.
func CORS(origins string, next http.Handler) http.Handler {
	allowed := parseOrigins(origins)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vary regardless of outcome: the response body may be identical for
		// two origins while the headers differ, and a cache that missed this
		// would serve one origin's headers to another.
		w.Header().Add("Vary", "Origin")

		if allow, ok := allowedOrigin(allowed, r.Header.Get("Origin")); ok {
			w.Header().Set("Access-Control-Allow-Origin", allow)
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			// Authorization is not a CORS-safelisted header, so without it
			// every authenticated call fails at the preflight.
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseOrigins(origins string) []string {
	var out []string
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// allowedOrigin returns the value to echo in Access-Control-Allow-Origin.
//
// The request's own origin is echoed rather than the configured list, because
// the header takes exactly one value; echoing is why the Vary above matters.
func allowedOrigin(allowed []string, origin string) (string, bool) {
	// A request with no Origin is not a CORS request — same-origin, curl, or
	// a test — so strictly there is nothing to allow. Answer with the first
	// configured origin anyway: a browser ignores the header on a same-origin
	// response, and staying quiet here would make the middleware look broken
	// to anyone probing it with curl.
	if origin == "" {
		if len(allowed) == 0 {
			return "", false
		}
		return allowed[0], true
	}

	for _, a := range allowed {
		if a == "*" {
			// Echo rather than reply "*", so the header stays meaningful and
			// the same code path works if credentials are ever enabled.
			return origin, true
		}
		if strings.EqualFold(a, origin) {
			return origin, true
		}
	}
	return "", false
}

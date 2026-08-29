package api

import "net/http"

// CORS allows the browser app, which is served from a different origin, to
// call this API.
//
// WebSocket upgrades are not subject to CORS, which is why the editor worked
// without this; fetch() from the room page is.
//
// origin of "*" is fine only while there is nothing to protect: no auth, no
// cookies, no credentials. Narrow it to the deployed web origin at the same
// time as ws.CheckOrigin.
func CORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

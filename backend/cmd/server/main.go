// Command server runs the interview platform backend.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/api"
	"github.com/heshannethmina/interview-platform/backend/internal/judge0"
	"github.com/heshannethmina/interview-platform/backend/internal/ws"
)

// env returns the value of key, or fallback when it is unset.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	addr := env("ADDR", ":8080")
	judgeURL := env("JUDGE0_URL", "http://localhost:2358")
	// "*" is acceptable only while the API carries no credentials. Narrow it
	// alongside ws.CheckOrigin when auth lands.
	corsOrigin := env("CORS_ORIGIN", "*")

	// One hub goroutine per room, created on first join and shut down when
	// the last client leaves. Still no auth: anyone with a room ID is in.
	rooms := ws.NewRegistry()
	go rooms.Run()

	// Code execution goes to Judge0, in its own container. The Go process
	// never runs submitted code itself.
	judge := judge0.New(judgeURL)

	apiMux := http.NewServeMux()
	apiMux.Handle("POST /api/run", api.Run(judge))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /ws/{roomID}", ws.Handler(rooms))
	// Wrapped rather than mounted directly so the CORS preflight is answered
	// for every /api/ route, including ones that do not exist yet.
	mux.Handle("/api/", api.CORS(corsOrigin, apiMux))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
		// No WriteTimeout: it would kill long-lived WebSocket connections.
		// The per-write deadline in the client's write pump covers us instead.
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server: listening on %s (judge0 at %s)", addr, judgeURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("server: shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server: shutdown: %v", err)
	}
}

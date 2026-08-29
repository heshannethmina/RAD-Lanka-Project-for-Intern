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

	"github.com/heshannethmina/interview-platform/backend/internal/ws"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// Roadmap step 1: exactly one room, no auth. Step 2 replaces this single
	// hub with a map of room ID -> hub, one goroutine each.
	hub := ws.NewHub()
	go hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /ws", ws.Handler(hub))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
		// No WriteTimeout: it would kill long-lived WebSocket connections.
		// The per-write deadline in the client's write pump covers us instead.
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server: listening on %s", addr)
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

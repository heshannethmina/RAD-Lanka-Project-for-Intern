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
	"github.com/heshannethmina/interview-platform/backend/internal/store"
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
	// Render and most other PaaS hosts pick the port themselves, inject it as
	// PORT, and route to it — a service listening anywhere else looks dead to
	// their health check. It wins over ADDR because it is not a preference,
	// it is where the platform is already sending traffic. ":port" binds
	// 0.0.0.0, which Render requires; binding localhost is not reachable.
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	judgeURL := env("JUDGE0_URL", "http://localhost:2358")
	// A comma-separated allowlist. "*" is local development only: the API now
	// carries bearer tokens, so a deployment must name its web origin here.
	// The same value gates WebSocket upgrades, which CORS does not cover.
	corsOrigin := env("CORS_ORIGIN", "http://localhost:3000")
	databaseURL := env("DATABASE_URL", "postgres://syncr:syncrdev@localhost:5433/syncr")

	// The application database — users, sessions, rooms. Not Judge0's, which
	// is reachable from the sandbox and holds only submissions.
	ctx := context.Background()
	db, err := store.Open(ctx, databaseURL)
	if err != nil {
		log.Fatalf("server: database: %v", err)
	}
	defer db.Close()

	// Migrations run at startup rather than as a deploy step: the schema is
	// small, they are idempotent, and it removes an ordering mistake that
	// would otherwise be possible on every deploy.
	migrateCtx, cancelMigrate := context.WithTimeout(ctx, 30*time.Second)
	if err := db.Migrate(migrateCtx); err != nil {
		cancelMigrate()
		log.Fatalf("server: migrate: %v", err)
	}
	cancelMigrate()

	// One hub goroutine per room, created on first join and shut down when
	// the last client leaves.
	//
	// The callback records an interview that ran out of time. It is a function
	// rather than the store itself so package ws stays ignorant of Postgres —
	// the same reason authorization arrives as a function.
	rooms := ws.NewRegistry(func(roomID string) {
		// Its own context: the room's is already gone by the time this runs.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.CloseExpiredRoom(ctx, roomID); err != nil {
			log.Printf("server: closing expired room %q: %v", roomID, err)
		}
	})
	go rooms.Run()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if _, err := db.DeleteExpiredSessions(cleanupCtx); err != nil { log.Printf("server: session cleanup: %v", err) }
			cancel()
		}
	}()

	// Code execution goes to Judge0. The Go process never runs submitted code
	// itself, whether Judge0 is a container next door or somebody else's API.
	//
	// JUDGE0_HEADERS carries whatever a hosted instance needs for auth, as
	// comma-separated "Name: Value" pairs — see judge0.ParseHeaders. A local
	// instance needs none. It holds a secret, so it is never logged.
	judgeHeaders, err := judge0.ParseHeaders(os.Getenv("JUDGE0_HEADERS"))
	if err != nil {
		log.Fatalf("server: JUDGE0_HEADERS: %v", err)
	}
	judge := judge0.New(judgeURL, judgeHeaders...)

	// Public: no session required.
	apiMux := http.NewServeMux()
	apiMux.Handle("POST /api/auth/register", api.RateLimit(api.Register(db), 5, 10*time.Minute))
	apiMux.Handle("POST /api/auth/login", api.RateLimit(api.Login(db), 10, 5*time.Minute))
	apiMux.Handle("POST /api/auth/logout", api.Logout(db))
	// The candidate's half of a shareable link: an invite token, no account.
	apiMux.Handle("GET /api/rooms/{roomID}/join", api.JoinRoom(db))
	apiMux.Handle("POST /api/run", api.RateLimit(api.Run(judge, api.RoomAuthorizer(db)), 30, time.Minute))

	// Everything an interviewer does with their own rooms.
	authed := http.NewServeMux()
	authed.Handle("GET /api/me", api.Me(db))
	authed.Handle("POST /api/rooms", api.CreateRoom(db))
	authed.Handle("GET /api/rooms", api.ListRooms(db))
	authed.Handle("GET /api/rooms/{roomID}", api.GetRoom(db))
	authed.Handle("DELETE /api/rooms/{roomID}", api.CloseRoom(db))
	authed.Handle("POST /api/rooms/{roomID}/invite", api.RotateInvite(db))
	authed.Handle("PUT /api/rooms/{roomID}/prompt", api.UpdatePrompt(db))
	// Redeeming a promotion needs an account: the grant is applied to a user
	// row, so there is nowhere to put it before somebody has registered.
	authed.Handle("POST /api/promo/redeem", api.RedeemPromo(db))
	// ServeMux prefers the more specific pattern, so the public
	// /api/rooms/{roomID}/join above still wins over this catch-all.
	apiMux.Handle("/api/me", api.RequireAuth(db, authed))
	apiMux.Handle("/api/rooms", api.RequireAuth(db, authed))
	apiMux.Handle("/api/rooms/{roomID}", api.RequireAuth(db, authed))
	apiMux.Handle("/api/rooms/{roomID}/invite", api.RequireAuth(db, authed))
	apiMux.Handle("/api/rooms/{roomID}/prompt", api.RequireAuth(db, authed))
	apiMux.Handle("/api/promo/redeem", api.RequireAuth(db, authed))

	// WebSocket upgrades ignore CORS entirely, so this is the only thing
	// stopping a hostile page from opening a socket in a victim's browser.
	ws.AllowOrigins(corsOrigin)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /ws/{roomID}", ws.Handler(rooms, api.RoomAuthorizer(db)))
	// Wrapped rather than mounted directly so the CORS preflight is answered
	// for every /api/ route, including ones that do not exist yet.
	mux.Handle("/api/", api.SecurityHeaders(api.CORS(corsOrigin, apiMux)))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
		// No WriteTimeout: it would kill long-lived WebSocket connections.
		// The per-write deadline in the client's write pump covers us instead.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 * 1024,
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

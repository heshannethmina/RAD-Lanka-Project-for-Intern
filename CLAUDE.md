# CLAUDE.md

Guidance for Claude Code (and future-me) when working in this repository.

## Project Overview

**Product:** A technical interview platform — a collaborative code editor with live code execution, built as a leaner, cheaper alternative to CoderPad/HackerRank/Codility, initially targeted at startups, small companies, and university placement/recruitment programs.

**Core value prop:** Fast, low-friction, affordable technical interviews with a shared editor and real code execution. No bloat, no enterprise pricing.

**Primary technical goal (secondary but explicit):** This project is also how I'm learning Go properly — specifically its concurrency model (goroutines + channels). The Go backend is not incidental; the WebSocket hub is intentionally hand-built rather than outsourced to a managed real-time service, because building it is the point.

## Tech Stack

| Layer | Choice | Why |
|---|---|---|
| Frontend | Next.js (App Router) + TypeScript | SSR for landing/marketing pages, file-based routing for auth/dashboard, easy deploy |
| Editor | Monaco Editor (`@monaco-editor/react`) | Same editor as VS Code, syntax highlighting/autocomplete built in |
| Styling | Tailwind CSS | Fast to ship |
| Backend | Go | WebSocket hub, REST API, session orchestration. Chosen specifically for goroutine/channel-based concurrency |
| Real-time | Native WebSockets via `gorilla/websocket` | Editor sync, cursor/presence broadcasting |
| Database | PostgreSQL | Users, sessions, questions, interview records — durable, relational |
| Cache/pubsub | Redis | Presence state, and pub/sub for scaling WS rooms across multiple Go instances (not needed for single-instance MVP, but designed in from the start) |
| Code execution | Judge0 (self-hosted) | Sandboxed multi-language execution via Docker isolation. Do NOT hand-roll code execution security — this is a solved problem, use Judge0 |
| Infra (local) | Docker Compose | Go, Postgres, Redis, Judge0 all run locally via compose |
| Reverse proxy | Nginx or Caddy | TLS termination, routing |

**Explicitly deferred / out of scope for v1:** video/audio (LiveKit or Daily.co if/when added — do not build WebRTC signaling by hand), session recording/playback, auto-grading, ATS integrations, more than 2-3 languages.

## Architecture

```
Next.js Frontend (landing, auth, dashboard, interview room page)
    │ HTTPS (REST)          │ WebSocket (editor sync, presence)
    ▼                       ▼
Go Backend
  ├── REST API        → auth, room CRUD, question bank
  ├── WS Hub          → 1 goroutine per connection, channel-based
  │                      broadcast within room; one hub goroutine
  │                      per room owns document state exclusively
  └── Judge0 Proxy    → forwards "Run Code" requests
    │
    ├── PostgreSQL (source of truth: users, sessions, questions, records)
    └── Redis (ephemeral: presence, pub/sub for multi-instance scaling)
         │
         Judge0 workers (Docker fleet, sandboxed execution)
```

**Key architectural decision — conflict resolution without OT:**
Each active interview room has exactly one hub goroutine that owns the document state. All client edits flow through a channel into this goroutine and are processed strictly in arrival order. This avoids the need for Operational Transformation while still being correct, because there's no shared mutable state — the hub goroutine is the sole writer. This is intentional and should not be "simplified" into a shared-mutex model; the whole point is idiomatic Go concurrency (share memory by communicating, not the other way around).

**The Next.js interview room page connects directly to the Go WebSocket server — this connection does NOT proxy through Next.js.** Next.js only renders the Monaco editor shell and UI; live sync bypasses Next.js entirely.

**Judge0 runs as a fully separate service from the Go app**, specifically so an execution sandbox escape doesn't touch the main application process.

## Build Order / Roadmap

1. Go WebSocket hub — single room, no auth, broadcast raw text edits
2. Multi-room support (map of room ID → hub goroutine)
3. Monaco editor integrated in Next.js, wired to the WS connection
4. Judge0 integration — "Run Code" button, proxied through Go
5. Auth (interviewer accounts) + session links (candidate joins via shareable link)
6. Question bank / templates
7. Polish, deploy, get 2-3 real interviewers to pilot it
8. (Later, only if validated) video/audio, recording/playback, auto-grading

**Principle:** ship the editor + execution first and validate real usage before adding features. Don't add video or scaling infrastructure (Redis pub/sub across instances) until there's an actual reason to.

## Repository Structure

```
/interview-platform
  /web              → Next.js app
  /backend          → Go backend
    /cmd/server     → main entrypoint
    /internal
      /ws           → WebSocket hub, room, client connection logic
      /api          → REST handlers (auth, rooms, questions)
      /judge0        → Judge0 client/proxy
      /store         → Postgres access layer
    /migrations     → SQL migrations
  /docker-compose.yml
  CLAUDE.md
```

## Conventions

- **Go:** standard project layout (`cmd/`, `internal/`), idiomatic error handling (no panics for expected errors), goroutines communicate via channels, not shared mutexes, unless there's a specific documented reason
- **Go concurrency rule:** if you're reaching for a `sync.Mutex` to protect room/document state, stop — that state should be owned by a single goroutine and mutated only via channel messages instead
- **Frontend:** TypeScript strict mode, Tailwind for styling, no CSS-in-JS
- **API:** REST for CRUD (auth, rooms, questions), WebSocket only for real-time editor/presence sync
- **Security:** never execute arbitrary code directly in the Go process — always through Judge0
- **Commits:** conventional commits style if possible (`feat:`, `fix:`, `chore:`)

## Current Status

Project scaffolding stage. Nothing built yet — starting with step 1 of the build order (Go WebSocket hub, single room).

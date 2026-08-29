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

Go module path: `github.com/heshannethmina/interview-platform/backend`

```
/interview-platform
  /web              → Next.js app                          [exists]
    /app            → routes: landing, /room/[roomId]      [exists]
    /components     → UI, incl. RoomEditor (Monaco)        [exists]
    /lib            → useRoomSocket, applyRemoteText       [exists]
  /backend          → Go backend
    /cmd/server     → main entrypoint                      [exists]
    /internal
      /ws           → hub, registry, client, protocol      [exists]
      /api          → REST handlers (auth, rooms, questions)   [planned]
      /judge0       → Judge0 client/proxy                      [planned]
      /store        → Postgres access layer                    [planned]
    /migrations     → SQL migrations                           [planned]
  /docker-compose.yml                                          [planned]
  CLAUDE.md
```

## Running Locally

```bash
# backend — WebSocket hub on :8080 (override with ADDR)
cd backend && go run ./cmd/server
go test ./...            # -race needs CGO_ENABLED=1 and gcc on PATH

# frontend
cd web && npm run dev
```

Health check: `GET /healthz`. WebSocket: `GET /ws/{roomID}`.

The frontend reads `NEXT_PUBLIC_WS_URL` (see `web/.env.example`), defaulting
to `ws://localhost:8080`. Room IDs must match `^[A-Za-z0-9_-]{1,64}$`;
anything else is rejected with 400 before the upgrade.

## WebSocket Protocol

One JSON envelope in both directions, so the client only ever parses one shape:

| Type | Direction | Payload | Meaning |
|---|---|---|---|
| `snapshot` | server → client | `text` | Full document, sent once on join so late joiners start in sync |
| `edit` | both | `text` | Full document after an edit. Server relays to everyone **except** the author |
| `presence` | server → client | `clients` | Number of clients currently in the room |

Edits carry the **whole document**, not a diff. That is deliberate: the hub
goroutine serialises them, so last-write-wins is well defined without OT.
Revisit only if document size actually becomes a problem — not before.

Two things that bite if forgotten:

- Go's `omitempty` **drops `text` entirely when the document is empty**, so
  the client must read it as `text ?? ""`. A first client's snapshot arrives
  as literally `{"type":"snapshot"}`.
- An empty snapshot means "you are the first one here". The client seeds the
  room by sending its local buffer as an edit, rather than blanking itself.

## Conventions

- **Go:** standard project layout (`cmd/`, `internal/`), idiomatic error handling (no panics for expected errors), goroutines communicate via channels, not shared mutexes, unless there's a specific documented reason
- **Go concurrency rule:** if you're reaching for a `sync.Mutex` to protect room/document state, stop — that state should be owned by a single goroutine and mutated only via channel messages instead
- **Frontend:** TypeScript strict mode, Tailwind for styling, no CSS-in-JS
- **API:** REST for CRUD (auth, rooms, questions), WebSocket only for real-time editor/presence sync
- **Security:** never execute arbitrary code directly in the Go process — always through Judge0
- **Commits:** conventional commits style if possible (`feat:`, `fix:`, `chore:`)

## Current Status

**Build order steps 1-3 are done: rooms sync end to end.** Two browsers on the
same `/room/<id>` see each other's keystrokes.

### Backend

- `internal/ws/hub.go` — one hub goroutine per room, sole owner of `clients`
  and `document`. No mutex, by design. A client that fills its send buffer is
  dropped rather than allowed to block the room.
- `internal/ws/registry.go` — room ID → hub. Both join and leave pass through
  the registry goroutine, which is what makes teardown safe: a room cannot be
  handed to a new client while it is being closed. Rooms open on first join
  and close when the last client leaves.
- `internal/ws/client.go` — one read pump and one write pump per connection,
  keeping inside gorilla's one-reader/one-writer rule.
- `internal/ws/handler.go` — validates the room ID, then upgrades. Every
  `Join` is paired with a deferred `Leave`.

### Frontend

- `lib/useRoomSocket.ts` — connects straight to the Go server, reconnects with
  exponential backoff, tracks presence.
- `lib/applyRemoteText.ts` — applies a remote document by diffing to the
  changed span. `setValue` would work but resets the local cursor on every
  keystroke the other person makes, which makes the editor unusable for
  whoever is not typing.
- `components/RoomEditor.tsx` — Monaco wired to the socket, with an echo guard
  so an applied remote edit is not bounced back. Top bar shows real connection
  status and live peer avatars driven by presence frames.

### Design system

The UI follows a supplied mockup: blue (`--accent-a #5B8CFF`) to violet
(`--accent-b #8B5CF6`) on deep navy (`--bg #070A18`), with a violet bloom
top-centre. Everything is built from three utilities in `app/globals.css` —
`.glass` (floating cards), `.inset` (recessed panels: code body, output box,
FAQ rows) and `.btn-accent` (primary action). Product name is **Interview
Platform**; `components/Logo.tsx` is the single source for the mark.

Monaco runs a custom `interview-dark` theme registered in `beforeMount`, with
`editor.background` set to `#00000000` so the glass panel shows through. If
the editor ever renders on an opaque slab, that theme failed to register.

The room defaults to **Python**, with a starter and prompt that match
("return the largest value", input `[10, 5, 22, 11]` → `22`).

### Still stubbed

- **`handleRun` is a `setTimeout`** returning fake output. No Judge0, no proxy
  (step 4).
- No auth: anyone with a room ID is in (step 5).
- Nothing persists. A room's document lives only in its hub goroutine and dies
  when the last client leaves — pinned by `TestReopenedRoomStartsEmpty`.
- Language selection is per-client, not synced.
- `CheckOrigin` allows every origin.

### Testing note

`go test ./...` covers snapshot-on-join, author exclusion, presence, room
isolation, room lifecycle, edit serialisation under concurrent writers, and
slow-client eviction.

Two tests (`TestConcurrentEditsSerialise`, `TestSlowClientIsDropped`) drive the
hub's channels directly instead of going through sockets, and that is
deliberate. Over real connections, enough writers overrun each other's send
buffers, so the hub correctly drops them and the room empties — which turns a
serialisation test into a flaky backpressure test. Keep them white-box.

**The race detector has never been run** — it needs `CGO_ENABLED=1` and gcc on
PATH. On a project whose point is concurrency, that is the highest-value gap
to close.

**Next:** step 4, Judge0 execution proxied through Go.

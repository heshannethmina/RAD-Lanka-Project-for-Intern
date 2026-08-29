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
      /api          → REST handlers (run; auth/rooms later)   [exists]
      /judge0       → Judge0 client/proxy                      [exists]
      /store        → Postgres access layer                    [planned]
    /migrations     → SQL migrations                           [planned]
  /docker-compose.yml → Judge0 stack                           [exists]
  /judge0.conf                                                 [exists]
  CLAUDE.md
```

## Running Locally

```bash
# Judge0 (sandboxed execution) — must be up before the Run button works
docker compose up -d
curl http://localhost:2358/about

# backend — WS hub + REST on :8080
cd backend && go run ./cmd/server
go test ./...            # -race needs CGO_ENABLED=1 and gcc on PATH

# frontend
cd web && npm run dev
```

Backend env: `ADDR` (`:8080`), `JUDGE0_URL` (`http://localhost:2358`),
`CORS_ORIGIN` (`*`).

Health check: `GET /healthz`. WebSocket: `GET /ws/{roomID}`.
Execution: `POST /api/run` with `{"language","source"}`.

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

**Build order steps 1-4 are done.** Two browsers on the same `/room/<id>` see
each other's keystrokes, and Run executes the code for real via Judge0.

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

### Execution (step 4)

- `internal/judge0/client.go` — submits source and polls for a verdict.
  `wait=false` deliberately: Judge0's synchronous mode is documented as
  unreliable under load. Language names map to Judge0 numeric IDs here, and
  those IDs are pinned to the `judge0/judge0:1.13.0` image tag — if that tag
  moves, re-check them against `GET /languages`.
- `internal/api/run.go` — `POST /api/run`. Validates the language against our
  own list rather than letting Judge0 reject it, so a client can never select
  a language ID we did not intend to expose. Judge0 failures become a plain
  502; its internals are logged, never returned.
- `internal/api/cors.go` — the room page fetches cross-origin. WebSocket
  upgrades are exempt from CORS, which is why the editor worked without this
  and the Run button would not have.
- `web/lib/runCode.ts` — posts to the Go API. The browser never talks to
  Judge0 directly.

Judge0 runs in its own containers with `ENABLE_NETWORK=false`, and brings its
own Postgres and Redis — unrelated to the application database that steps 5-6
will add. The Go process never executes submitted code; if a change ever makes
it tempting to shell out, that is the wrong turn.

### Still stubbed

- No auth: anyone with a room ID is in (step 5).
- Nothing persists. A room's document lives only in its hub goroutine and dies
  when the last client leaves — pinned by `TestReopenedRoomStartsEmpty`.
- **Run output is not shared.** Only the person who pressed Run sees the
  result; the other side of the interview sees nothing. The document syncs but
  the output panel does not. Fixing it means a new `result` frame relayed by
  the hub — the natural next increment on top of step 4.
- Language selection is per-client, not synced, so two people in one room can
  submit the same source under different languages.
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

**Judge0 has never been exercised against the real service.** The client and
proxy are covered against a fake Judge0, and the full chain (browser payload →
Go → Judge0 → response) was verified against a stub, confirming the language-ID
mapping, sandbox limits, CORS preflight and error paths. But
`judge0/judge0:1.13.0` would not finish downloading here. First person with a
working connection should run `docker compose up -d`, wait for
`curl localhost:2358/about`, and press Run. The likeliest thing to be wrong is
a language ID: verify against `GET /languages`.

**The race detector has never been run** — it needs `CGO_ENABLED=1` and gcc on
PATH. On a project whose point is concurrency, that is the highest-value gap
to close.

**Next:** step 5, auth for interviewers plus shareable candidate links.

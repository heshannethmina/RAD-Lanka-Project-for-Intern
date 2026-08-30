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
  /web              → Next.js app                                    [exists]
    /app            → landing, /login, /register, /dashboard,
                      /room/[roomId]                                [exists]
    /components     → UI, incl. RoomEditor (Monaco), RoomGate,
                      AuthForm, Dashboard                           [exists]
    /lib            → api, useAuth, useRoomSocket, applyRemoteText  [exists]
  /backend          → Go backend
    /cmd/server     → main entrypoint                               [exists]
    /internal
      /ws           → hub, registry, client, protocol, authorizer   [exists]
      /api          → REST: run, auth, rooms, ws authorizer         [exists]
      /judge0       → Judge0 client/proxy                           [exists]
      /store        → Postgres access layer                         [exists]
      /auth         → password + token primitives                   [exists]
    /migrations     → SQL migrations (embedded via go:embed)        [exists]
  /docker-compose.yml     → Judge0 stack                            [exists]
  /docker-compose.app.yml → application Postgres                    [exists]
  /judge0.conf                                                      [exists]
  CLAUDE.md
```

## Running Locally

```bash
# Judge0 (sandboxed execution) — must be up before the Run button works.
# NOTE: on Docker Desktop for Windows every run fails with Internal Error;
# see the Judge0 section under "Testing note". Judge0 needs a Linux host.
docker compose up -d
curl http://localhost:2358/about

# application database — users, sessions, rooms. Separate file, separate
# stack, deliberately not Judge0's Postgres.
docker compose -f docker-compose.app.yml up -d --wait

# backend — WS hub + REST on :8080. Migrations run automatically at startup.
cd backend && go run ./cmd/server
go test ./...

# the store tests need a real Postgres and skip without this
TEST_DATABASE_URL='postgres://syncr:syncrdev@localhost:5433/syncr' \
  go test ./internal/store/
# -race needs CGO_ENABLED=1 and a gcc. On this machine gcc is at
# C:\Users\hesha\mingw64\bin and is NOT on PATH; see "Testing note" below.
PATH="/c/Users/hesha/mingw64/bin:$PATH" CGO_ENABLED=1 \
  CC="C:/Users/hesha/mingw64/bin/gcc.exe" go test -race ./...

# frontend
cd web && npm run dev
```

Backend env: `ADDR` (`:8080`), `JUDGE0_URL` (`http://localhost:2358`),
`DATABASE_URL` (`postgres://syncr:syncrdev@localhost:5433/syncr`), and
`CORS_ORIGIN` (`*`) — a comma-separated allowlist that gates **both** the REST
CORS headers and WebSocket `CheckOrigin`. `*` is local development only.

`PORT` overrides `ADDR` when set, because Render and every other PaaS picks the
port itself and routes to it — a service listening anywhere else looks dead to
their health check.

`JUDGE0_HEADERS` authenticates a **hosted** Judge0, as comma-separated
`Name: Value` pairs. One variable rather than a pair per provider, because each
wants different headers and hard-coding vendor names would mean editing Go to
change hosting:

```
# RapidAPI
JUDGE0_URL=https://judge0-ce.p.rapidapi.com
JUDGE0_HEADERS=X-RapidAPI-Key: <key>, X-RapidAPI-Host: judge0-ce.p.rapidapi.com

# Judge0 Cloud, or self-hosted with AUTHN_TOKEN set
JUDGE0_HEADERS=X-Auth-Token: <token>
```

Parsing splits on the first colon only, so a value may contain one; it may not
contain a comma. The headers go on the poll as well as the submit —
authenticating only the submit fails confusingly, with the submission accepted
and every poll for its verdict rejected. Malformed values are fatal at startup
rather than at first Run, and the error never quotes the value, which is a
credential.

Going hosted means **candidate code leaves your infrastructure**, which is a
product decision as much as a technical one. It is also the only way to see the
Run button work without a Linux host — and the language IDs must be re-checked
against `GET /languages` first, since ours are pinned to the `1.13.0` image and
a hosted instance runs something newer.

| Route | Auth | Purpose |
|---|---|---|
| `GET /healthz` | — | liveness |
| `POST /api/auth/register` | — | create an interviewer, returns a token |
| `POST /api/auth/login` | — | exchange credentials for a token |
| `POST /api/auth/logout` | bearer | revoke the presented token |
| `GET /api/me` | bearer | who am I |
| `POST /api/rooms` | bearer | create a room, returns the invite **once** |
| `GET /api/rooms` | bearer | list my rooms, newest first |
| `GET /api/rooms/{id}` | bearer | one of my rooms |
| `DELETE /api/rooms/{id}` | bearer | close it; keeps the record |
| `POST /api/rooms/{id}/invite` | bearer | rotate the link, revoking the old |
| `GET /api/rooms/{id}/join?token=` | invite | candidate's link check |
| `POST /api/run` | — | `{"language","source"}` → Judge0 |
| `GET /ws/{roomID}?token=` | session **or** invite | the editor socket |

The socket takes its token in the query string because browsers cannot set
headers on a WebSocket handshake. Room IDs must match `^[A-Za-z0-9_-]{1,64}$`;
anything else is rejected with 400 before the upgrade, and the same pattern is
a CHECK constraint on `rooms.id`.

The frontend reads `NEXT_PUBLIC_WS_URL` (see `web/.env.example`), defaulting
to `ws://localhost:8080`.

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

**Product name: SyncR.** The design direction is light, elegant and
restrained — closer to Google's own product sites than to a typical SaaS
landing page. The rules, in the order they matter:

- White and near-white surfaces, graphite text, generous whitespace.
- **One accent**, `--accent #005DED`, used sparingly. It is sampled from the
  logo, so the mark and the interface are literally the same blue.
- **Flat surfaces separated by hairlines** (`--line`), not by shadows or
  glows. Cards get a 1px border, not a card shadow.
- Hierarchy comes from type — size, weight, tracking — not decoration. One
  typeface (Inter) throughout, plus JetBrains Mono for code.
- Motion is a subtle scroll reveal (fade plus a small upward slide). No
  pulsing, floating, or glowing.

Explicitly avoided, because they read as generic AI-generated design:
purple-to-blue gradients, backdrop-blur glass panels, neon glow shadows,
floating badges with ping animations.

Utilities in `app/globals.css`:

| Utility | Used for |
|---|---|
| `.nav-bar` | the sticky bar; `data-scrolled` drives its settled state |
| `.nav-link` | nav link with a centre-out underline on hover |
| `.btn-primary` | flat, solid-accent, pill-shaped primary action |
| `.btn-secondary` | hairline-outlined action on white |
| `.eyebrow` | section label; the one accent-coloured item per section |
| `.card-tag` | card label, muted so a grid of them stays quiet |
| `.card` | flat card: hairline border, no shadow, no gradient edge |
| `.card--pick` | the recommended plan: accent hairline plus an inset ring |
| `.panel` | raised surface: hairline plus a single soft shadow |

Two things here are load-bearing and easy to undo by accident:

- **`backdrop-filter` values must go through a `var()`.** Given a literal,
  Lightning CSS rewrites the declaration to `-webkit-backdrop-filter` *only*
  and drops the standard property. Chrome does not support the prefixed
  alias, so the blur silently does nothing. That is why the nav's frost lives
  in `--nav-filter-*`. Check with `getComputedStyle($0).backdropFilter` —
  `none` means it regressed.
- **The nav declares its blur at rest too** (`blur(0px)`), so the property
  has something to animate from. Setting it only in the scrolled state makes
  the frost snap on instead of easing in.

`components/Logo.tsx` is the single source for the mark. It serves the
supplied artwork from `public/syncr-logo.png` (full lockup) and
`public/syncr-mark.png` (mark only) — both cropped to the ink and re-encoded
as flat `#005DED` over the alpha channel, which cut them to roughly a third
of their original weight. They are 256px tall, so they stay sharp at 3x.

### Motion and the loader

Two pieces, both driven from CSS so they work without waiting on React.

`components/Reveal.tsx` wraps a group and fades it in while sliding it up as
it enters the viewport. The observer **disconnects after the first
intersection** — a reveal that replays on every scroll past becomes a
distraction. Groups already on screen fire immediately, so the hero reads as
a load-in. Stagger sibling groups with `delay` (the hero uses 0 for the copy
and 140ms for the editor).

`components/SyncLoader.tsx` is the logo taken apart and set moving: the cycle
turns between the two brackets while the brackets ease out and back, both on
the same 1.15s period so they read as one gesture. The three pieces live in
`public/syncr-cycle.png`, `syncr-chev-left.png` and `syncr-chev-right.png`.
Sizes are computed from each piece's aspect ratio in the component rather
than set in CSS, so `next/image` gets real dimensions and the row cannot
reflow as the images arrive.

It is used in two places:

- `components/Splash.tsx` — the first-paint cover. It server-renders, so the
  mark is already turning in the first frame instead of appearing at
  hydration. Clears on `window.load` with a `MIN_MS` floor so a fast load is
  a deliberate beat rather than a flicker.
- `app/loading.tsx` — the same cover for route transitions.

**Anything that can hide content needs a `<noscript>` escape.** The layout
carries one that disables both the splash and the reveal's hidden state;
without it, a browser with JS off gets a blank page behind a cover with
nothing to dismiss it.

**Migration complete.** Nav, hero, features, pricing, FAQ, closing CTA and
the interview room are all on the light system. The compatibility layer that
mapped the old dark class names (`.glass`, `.inset`, `.btn-accent`,
`.tab-active`, `.chip`, `.ring-accent`, `.panel-deep`, `.text-gradient`,
`.lift`) and the `--ink-dim` / `--ink-faint` aliases has been deleted — 52
lines of CSS with no callers left.

The FAQ accordion animates height with `grid-template-rows: 0fr -> 1fr`
rather than by measuring `scrollHeight`, so it animates to the content's
natural height without JS knowing what that height is. Two things it depends
on: the inner wrapper needs `overflow: hidden` or the content simply ignores
the `0fr` row, and a collapsed panel needs `inert` — the content stays in the
DOM for the animation, so it has to be pulled out of the a11y tree and tab
order explicitly.

### The 3D mark

`components/Logo3D.tsx` renders `public/syncr-mark.glb` turning slowly in the
hero, behind `components/HeroVisual.tsx`, which code-splits it
(`dynamic(..., { ssr: false })`) and holds the space with the flat PNG until
the model resolves.

The source export was 3.2MB for only 31k triangles — unwelded float32, no
compression. It is now 402KB:

```bash
npx @gltf-transform/cli optimize in.glb out.glb --compress meshopt
```

Re-run that after any re-export; do not ship the raw file. **Meshopt, not
Draco** — drei bundles the meshopt decoder from three-stdlib, whereas the
Draco path fetches its decoder from gstatic at runtime. Quantize-only was
also measured, at 1.58MB, so meshopt is worth the decoder.

Two things keep it from being a battery drain: the frame loop stops when the
hero scrolls out of view, and the lighting environment is built in-memory
from `<Lightformer>` shapes with `frames={1}` rather than loaded as an HDR —
which also means no CDN round trip. `prefers-reduced-motion` stops the spin.

Note when debugging this in a headless or backgrounded browser: rAF is
throttled to almost nothing when the page is not compositing, so the model
looks frozen and screenshots come back stale. Count WebGL draw calls rather
than trusting a screenshot.

### Footer contact details are placeholders

`CONTACT_PLACEHOLDER` in `components/Footer.tsx` holds an invented email,
phone and city. **Replace them before the site goes anywhere public.** They
are grouped in one named constant rather than scattered through the markup
precisely so they are hard to miss.

Monaco runs a `syncr-light` theme registered in `beforeMount`, using the
same GitHub-light token palette as the marketing editor mockup so the
product and the page advertising it agree.

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

### Auth and shareable links (step 5, backend done)

An interview has exactly two kinds of participant, and there is no third:

| | Holds | Gets in when |
|---|---|---|
| Interviewer | session token, from login | they **own** the room |
| Candidate | the room's invite token | the room is still **open** |

Candidates are deliberately not users. They never register, never have a
password, and hold nothing but a link — that is the whole point of the
shareable link, and it is why `users` has no candidate rows.

**Tokens are opaque random bytes in Postgres, not JWTs.** 32 bytes from
`crypto/rand`, stored as a SHA-256 hash. That buys real revocation: logout
deletes a row and the token is dead immediately, where a JWT would stay valid
until it expired. It costs one indexed lookup per request, which is nothing
beside the socket it guards. There is no signing key to rotate or leak.

Passwords go through bcrypt; tokens through SHA-256. Different jobs: a
password is low-entropy and needs a slow hash, a 256-bit random token has
nothing to brute-force and a fast hash keeps it off the hot path.

**The session token travels in `Authorization: Bearer`, not a cookie.** A
cookie would be HttpOnly and so safer against XSS, but the web app and the API
are different origins in every environment — Vercel and a VPS in production,
`:3000` and `:8080` locally — so it would need `SameSite=None; Secure`, which
browsers refuse to send over plain http. That breaks local development
outright. The trade is written out at the top of `internal/api/auth.go`; keep
the XSS surface small rather than reversing it.

**The invite token is shown exactly once**, at create and at rotate, because
only its hash is stored. An interviewer who loses a link rotates it rather
than re-reading it, and a leaked database grants entry to nothing. If that
proves too annoying in the pilot, storing it in plaintext is a one-line schema
change — but do it deliberately, not by accident.

**404, never 403.** "Not yours" and "does not exist" answer identically, so
someone guessing room IDs learns nothing from the difference.

Login hashes against `auth.DummyPasswordHash` when the email is unknown, so a
missing account costs the same bcrypt work as a wrong password. Without it the
endpoint is a timing oracle over the user list.

Two things that are easy to get wrong here:

- **WebSocket upgrades ignore CORS entirely.** The browser sends the handshake
  whatever the origin, so `CheckOrigin` is the only thing between a hostile
  page and a socket opened with a victim's token. It now defaults to *deny*
  for anything carrying an `Origin`; a server that forgets `AllowOrigins` is
  closed to browsers rather than open to them. Requests with no `Origin` at
  all are allowed — those are curl and the Go test suite, not the attack.
- **`internal/ws` must not import `store`.** Authorization arrives as a
  `ws.Authorizer` func so the hub stays a thing that moves text between
  sockets. `api.RoomAuthorizer` is the real implementation; `ws.AllowAll` is
  for tests and is named to look wrong in production code.

Verified end to end against the live database and server: register, duplicate
rejection (409), wrong password (401), `/api/me` with and without a token,
room create/list/get/close, cross-account isolation (404 both ways, empty
listing), candidate join by invite, and the socket itself — no token 401, bad
token 401, invite 101, owner 101, other interviewer 401. Closing a room drops
the candidate to 401 while the owner keeps 101; rotating kills the old link;
logout kills both `/api/me` and the socket.

### The web side of step 5

`lib/api.ts` is the one place that talks to the REST API. The token lives in
`localStorage` under `syncr.session`; every read and write is wrapped in
try/catch because private mode and "block site data" *throw* rather than
return null.

`lib/useAuth.ts` resolves that token into a user by asking the server, rather
than trusting it — it may have expired or been revoked from another device.
Until the answer arrives the state is `loading`, and pages render nothing.
Rendering the signed-out view first and swapping is the flash this exists to
avoid.

**`components/RoomGate.tsx` is the interesting one.** It decides who is at the
door before any socket opens:

| URL | Who | Token used |
|---|---|---|
| `/room/<id>?t=<invite>` | candidate | the invite from the link |
| `/room/<id>` | interviewer | the session token in localStorage |

The invite is checked *first*, so a link still works for someone who also
happens to have an interviewer session in that browser — the link is what they
were sent.

Access is resolved over REST before connecting, and that ordering is
load-bearing: **a refused WebSocket handshake reaches the browser as close code
1006, identical to a dropped connection.** A socket alone cannot tell "you are
not allowed in" from "the wifi died", so the reconnect loop would hammer a room
the viewer will never enter, and the UI could never say why. The server still
authorises the socket itself; the REST check is about being able to explain the
refusal. `useRoomSocket` therefore takes a token and stays shut while it is
null.

The dashboard holds revealed invite tokens in component state only. A reload
loses them — the server stores just the hash — which is why a row's action
reads **"New link"** rather than "Copy link" once the token is gone. That is
the trade, surfaced rather than hidden.

Creating a room copies the candidate link to the clipboard immediately, since
that is the only reason anyone creates one. `navigator.clipboard` is
unavailable over plain http on anything but localhost, so there is a
`window.prompt` fallback — ugly, but it still gets the link into a hand.

**Marketing CTAs changed.** Nav and Hero pointed at `/room/demo`, which the
gate now refuses: rooms need an account or an invite, so an anonymous demo room
does not exist any more. They point at `/register` and `/login`, and the nav's
"Try live demo" became "Get started". Restoring a demo means building one that
is genuinely open, not repointing the links.

`.field`, `.field-label` and `.form-error` were added to `globals.css`; there
was no input style before this. `:focus` on `.field` deliberately replaces the
global `:focus-visible` outline, which sits outside the rounded box and reads
as detached at that size.

Verified with the real backend: all four routes serve 200, the login form
server-renders, the CORS preflight returns `Access-Control-Allow-Headers:
Content-Type, Authorization` and echoes `http://localhost:3000`, and a
cross-origin register returns 201 with the origin echoed. `next build` and
`tsc --noEmit` are both clean.

### Still stubbed
- Nothing of the *document* persists. A room's text lives only in its hub
  goroutine and dies when the last client leaves — pinned by
  `TestReopenedRoomStartsEmpty`. The room record survives; its contents do not.
- **Run output is not shared.** Only the person who pressed Run sees the
  result; the other side of the interview sees nothing. The document syncs but
  the output panel does not. Fixing it means a new `result` frame relayed by
  the hub — the natural next increment on top of step 4.
- Language selection is per-client, not synced, so two people in one room can
  submit the same source under different languages. The room now *has* a
  language column, so the fix is to make the client honour it.
- No rate limiting on login. bcrypt makes it slow, not impossible.
- Expired sessions are swept only by an explicit `DeleteExpiredSessions` call,
  which nothing schedules yet. Harmless — the expiry is enforced in the query,
  so a stale row is unusable, not dangerous.

### Testing note

`go test ./...` covers snapshot-on-join, author exclusion, presence, room
isolation, room lifecycle, edit serialisation under concurrent writers, and
slow-client eviction.

Two tests (`TestConcurrentEditsSerialise`, `TestSlowClientIsDropped`) drive the
hub's channels directly instead of going through sockets, and that is
deliberate. Over real connections, enough writers overrun each other's send
buffers, so the hub correctly drops them and the room empties — which turns a
serialisation test into a flaky backpressure test. Keep them white-box.

**Judge0 has now been run against the real service — and it cannot execute on
Docker Desktop for Windows.** The image finally downloaded (3.06GB compressed,
14.1GB extracted), the stack came up, and `GET /about` returned 1.13.0. Then
every submission failed:

```
status 13 Internal Error
Failed to create control group /sys/fs/cgroup/memory/box-1/: No such file or directory
Cannot write /sys/fs/cgroup/memory/box-2/tasks: No such file or directory
```

**The cause is cgroup v1 vs v2.** Judge0 1.13.0 sandboxes with isolate 1.8.1,
which only speaks cgroup v1. Docker Desktop's WSL2 VM is cgroup v2 only
(`stat -fc %T /sys/fs/cgroup` → `cgroup2fs`). isolate wants
`/sys/fs/cgroup/memory/box-N/tasks`; v2 has neither that hierarchy nor a
`tasks` file (it uses `cgroup.procs`). `isolate --cg -b 0 --init` fails on its
own, so this is not submission-specific.

Four workarounds were tried and all fail — do not spend the day re-deriving
this:

| Attempt | Result |
|---|---|
| Upgrade to `judge0/judge0:1.13.1` | Ships the **same isolate 1.8.1**. Identical failure. Not a fix. |
| `mkdir` the v1 paths in a privileged container | Passes `--init` (they become v2 subgroups), then dies writing `tasks` |
| Mount cgroup v1, Docker Desktop running | `memory` → `EBUSY`; the v2 hierarchy has the controller |
| Mount cgroup v1, Docker Desktop stopped | Still `EBUSY` — Ubuntu's own systemd claims `cpu memory pids` |

`cpuacct` *does* mount as v1; only the controllers something else has claimed
are blocked. Untried: `.wslconfig` with
`kernelCommandLine = systemd.unified_cgroup_hierarchy=0`. It is machine-wide,
changes the kernel command line for the `docker-desktop` distro too, and WSL's
init mounts `cgroup2fs` before systemd starts — so it may not help and may
break the Docker Desktop engine.

**What did get verified.** The language IDs are correct against the live
service — `GET /languages` gives 60 = `Go (1.13.5)`, 63 =
`JavaScript (Node.js 12.14.0)`, 71 = `Python (3.8.1)`, exactly as pinned in
`internal/judge0/client.go`. The full chain also works up to the sandbox:
`POST /api/run` returns 200 with the normalised `Result`, the Judge0 verdict
propagates through the Go proxy, the `Message` → `Stderr` fallback in
`toResult` fires, and an unsupported language is still rejected 400 by our own
allowlist. Nothing in this repo needs changing.

**Running Judge0 for real needs a Linux host.** Requirements, all confirmed:

- **amd64.** `judge0/judge0:1.13.0` is a single-arch amd64 manifest — no ARM
  build, so Oracle's free ARM tier and Graviton are out.
- **cgroup v1.** Ubuntu 20.04 has it by default. On 22.04/24.04 set
  `GRUB_CMDLINE_LINUX="systemd.unified_cgroup_hierarchy=0"`, `update-grub`,
  reboot — **a default 24.04 box hits this exact wall.**
- **Root with privileged Docker.** Rules out Railway, Render, Fly.io and Cloud
  Run; managed container platforms do not grant cgroup access.
- **~20GB disk**, 2-4GB RAM. The image alone is 14.1GB.

Vercel cannot host Judge0 (no containers, no privileged mode, 250MB function
limit) and cannot host the Go backend either — it does not support WebSockets,
and the one-hub-goroutine-owns-the-document design needs a long-lived process
with shared memory, which serverless cannot provide. Vercel is right for
`web/` only; the Go backend and Judge0 need a VPS.

**The race detector has now been run, and it is clean.** Every package passes
under `-race`, and `internal/ws` — the concurrent one — also passes a 10x
stress run:

```
ok  internal/api     4.161s
ok  internal/judge0  4.303s
ok  internal/ws      4.892s      (and -count=10 -> ok, 11.747s)
```

All nine ws tests pass: snapshot-on-join, author exclusion, presence, edit
serialisation under concurrent writers, slow-client eviction, room isolation,
room open/close lifecycle, reopened-room emptiness, and room-ID rejection.
**No data races were reported in the hub, registry or client.** That is the
first real evidence that the share-by-communicating design holds up — the
document and client set are only ever touched by their owning goroutine.

Caveat worth keeping in mind: `-race` only reports races that actually happen
during a run, so this is evidence, not proof. Re-run it after any change to
hub, registry or client — it is cheap now that the toolchain is in place.

**Toolchain setup, because it was not obvious.** The detector needs
`CGO_ENABLED=1` and a gcc, and this machine had none — no MinGW, no chocolatey
gcc, and Ubuntu-24.04 has neither gcc nor Go. `winget install` for a mingw
package hung with zero bytes downloaded (Delivery Optimization stall), so gcc
was installed by hand instead:

```bash
# mingw-w64 GCC 16.2.0, ~103MB, no admin needed
curl -L -C - -o mingw.7z https://github.com/niXman/mingw-builds-binaries/releases/download/16.2.0-rt_v14-rev1/x86_64-16.2.0-release-posix-seh-ucrt-rt_v14-rev1.7z
cd /c/Users/hesha && /c/Windows/System32/tar.exe -xf mingw.7z   # bsdtar reads 7z
```

It now lives at `C:\Users\hesha\mingw64\bin\gcc.exe`, which is **not on PATH**.
To run the detector:

```bash
export PATH="/c/Users/hesha/mingw64/bin:$PATH"
CGO_ENABLED=1 CC="C:/Users/hesha/mingw64/bin/gcc.exe" go test -race ./...
```

Prefer niXman's `.7z` over the WinLibs `.zip`: same compiler, 103MB against
261MB. Windows' own `System32\tar.exe` is bsdtar/libarchive and unpacks 7z
without needing 7-Zip installed.

**Next:** step 5, auth for interviewers plus shareable candidate links. Judge0
execution stays unvalidated until there is a Linux host; that is a deployment
task, not a code task, and nothing in step 5 depends on it.

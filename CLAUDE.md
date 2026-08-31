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

`OWNER_EMAILS` is the operator's own accounts, comma-separated — see
"Owners and the admin UI". Unset means nobody is an admin.

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
| `POST /api/promo/redeem` | bearer | claim a promotion code |
| `GET/POST /api/admin/promo` | owner | list or issue codes |
| `DELETE /api/admin/promo/{code}` | owner | revoke a code |
| `GET /api/admin/users` | owner | list accounts and usage |
| `PATCH /api/admin/users/{id}` | owner | change a subscription |
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
| `result` | both | `text`, `failed` | Output of a run. Relayed to everyone **except** the author, who already has it |
| `prompt` | both | `prompt` | The interview question. Accepted from an **interviewer only**; room state, so it is in the snapshot |
| `activity` | both | `kind`, `lines`, `ms`, `text`, `activity`, `event` | Candidate left, returned, or pasted. Accepted from a **candidate only**; the tally and the new event ride along |
| `pointer` | both | `x`, `y`, `role` | Mouse position as a fraction of the viewport. Relayed, never stored |
| `ended` | server → client | — | The interview ran out of time. The room goes read-only |

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

**The CSP has to allow the CDN for scripts, styles *and* fonts.** Monaco is
fetched by `@monaco-editor/react` rather than bundled, and Pyodide is fetched
by `public/python-worker.js`; both come from `cdn.jsdelivr.net`.

`script-src` allowed it and `style-src` did not, so Monaco's JavaScript loaded
while `editor.main.css` was refused. Monaco then rendered without the rules
that position its hidden `<textarea>` and map clicks onto text: the textarea
showed up as a plain resizable form field floating over the code, complete with
the browser's own resize grip, and the caret stopped following the mouse. It
was reported as "the cursor is stuck and there are boxes", which is exactly
what it looked like — nothing in it pointed at a blocked stylesheet.

Two things about that failure are worth remembering. It appears **only in a
browser**, so `tsc`, `eslint`, `next build` and CI were all green throughout.
And Monaco ships its icons as `codicon.ttf` from the same path, so `font-src`
needs the CDN too or the editor silently loses its icons.

The policy in `web/next.config.ts` is now a list of directives with the reason
written next to it, rather than one long string in which a missing entry is
invisible.

**Monaco needs `automaticLayout: true`, and it is not optional here.** It
caches its container's size and maps mouse coordinates against that cache, so
once the container has resized without it being told, clicks put the caret
somewhere other than where you clicked — and in the region it believes is
outside the editor, nothing happens at all. It reads exactly like a stuck
cursor, which is how it was reported. The editor sits inside two draggable
`SplitPane`s and a resizable window, so its container changes size constantly.
`automaticLayout` installs a `ResizeObserver`; it is not the old polling loop
that guides still warn about.

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

**The invite is read once, not live from the router.** The gate strips `?t=`
out of the address bar as soon as it has it, so a live invite token does not
sit in a screenshot, a screen share or browser history. But Next feeds
`history.replaceState` straight back into `useSearchParams`, so reading the
token live meant it went **null the instant the URL was cleaned** — the effect
re-ran, its cleanup aborted the join request already in flight, and the second
pass fell through to the interviewer branch. A candidate holding a perfectly
good link was told to sign in. The token is now latched into state on first
render, which breaks that loop.

It is also kept in `sessionStorage`, keyed by room. Once the URL has been
cleaned it is the only copy, and without it **a reload locked the candidate out
of their own interview**. Per tab and per room: sessionStorage dies with the
tab, which is the right lifetime for a link admitting somebody to one
interview, and the key stops two rooms open in one tab overwriting each other.
A refusal from the API clears it, so a rotated link stops replaying the same
error on every reload.

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

### Python runs in the browser

`web/lib/runPython.ts` runs Python through **Pyodide** — CPython compiled to
WebAssembly — in the viewer's own tab. Judge0 is still wired up and still
handles Go and JavaScript; Python simply never reaches it.

This is not a stopgap dressed as a decision. An interview is an invitation for
a stranger to run code, and on a managed host there is no sandbox available:
`exec.Command("python3", ...)` in the Go process would let a candidate read
`os.environ["DATABASE_URL"]` and walk off with every account in the database.
Judge0 exists to prevent exactly that, but it needs cgroup v1 and a privileged
container, which Render does not grant. Running the code in the candidate's own
browser **sidesteps** the problem instead of defending against it: there is
nothing of ours near the code, and the worst outcome is a hung tab.

The trade is that output is not verified — a determined candidate could patch
the result before it is shared. In a live interview you are watching them, so
this matters far less than it would for take-home grading. If verified
execution is ever needed, that is Judge0 on a real Linux host, and nothing here
is in the way of it.

Two details worth keeping:

- The runtime is **prefetched when Python is selected**, not on the first
  click. It is a ~6MB download; paying for it while someone reads the question
  is invisible, paying for it after they press Run is a long unexplained wait.
- `runPython` **never throws**. A traceback is the *output* of a failed run and
  belongs in the same panel as a successful one, not in a separate error
  banner. Anything printed before the failure is kept, because it is usually
  the half that explains the traceback.

Pyodide's version is pinned: its wheel URLs are version-scoped, so a floating
version breaks package loading.

### Getting around

Every page has a way out, which it did not before — a room was a dead end with
no route back to the dashboard, and the marketing pages still pointed at a
`/room/demo` that the auth gate now refuses.

The room header is **role-aware**, and that is the part worth not undoing:

- **Interviewer** — a back chevron to `/dashboard`, labelled "Interviews".
- **Candidate** — the mark is decoration, not a link. They have no dashboard to
  return to, and a stray click mid-interview would drop them out of the room.
  Leaving is what the browser's own back button is for.

The header shows the room's title with the ID beneath it, so an interviewer
running several sessions can tell which one they are in.

The nav offers "Go to interviews" instead of "Sign in" when a session token is
present. `lib/useSession.ts` reads that through `useSyncExternalStore` rather
than an effect: localStorage is external state, so this gives the right answer
on the client without a hydration mismatch and without the extra render an
effect would cost. It deliberately does **not** verify the token with the
server — being wrong costs a redirect to `/login`, nothing more. Use `useAuth`
where the answer has to be trustworthy.

The "not open to you" screen offers whatever actually helps: an interviewer who
is still signed in gets their list back, one whose session lapsed gets sign-in,
and everyone gets a way home. A screen with no exit is how someone gets stuck.

**The room warns before unload** while the socket is open. The friction is
justified because the document is not persisted — it lives in the hub goroutine
and dies with the last connection, so a mistaken reload can lose a candidate's
work outright. Browsers ignore any custom message and show their own wording;
`preventDefault` is what arms the dialog at all.

### Watching the candidate

Detection only, and the limit is worth stating before the design: **a browser
cannot stop someone opening another tab.** There is no API for it, deliberately
— a page is not allowed to trap a user — and anyone determined can use a phone,
a second monitor, or another machine. Every event here is a signal that
somebody stepped away, never proof of anything, and the UI is worded so it
cannot be read as an accusation.

What is collected, and nothing else:

| Signal | Source |
|---|---|
| Left the tab / came back, with duration | `visibilitychange` and window `blur` |
| Pasted into the editor, with line count **and the pasted text** | Monaco `onDidPaste` |

**Aggregates, not a stream.** No keystroke or mouse logging. Not because it is
hard, but because "candidate pressed 1,847 keys" answers no question, while
being real surveillance of someone who is not an employee and creating a
data-protection obligation the product does not otherwise have. *Left four
times, two minutes total, pasted twice* tells an interviewer everything a raw
log would and is defensible if a candidate asks what was kept.

Paste is the signal that earns its place: people switch tabs to read
documentation many interviewers explicitly allow, but forty lines arriving at
once is a different kind of event.

**The tally lives in the hub**, not in the interviewer's browser, so reloading
mid-interview does not reset it and a second interviewer sees the same numbers.
Pinned by `TestTallySurvivesAReload`.

Three things that are load-bearing:

- **A short absence is ignored** (`MIN_AWAY_MS`, 1.5s). A notification stealing
  focus or a click on the browser chrome is not somebody leaving, and without
  the floor the interviewer's panel fills with noise and stops being read —
  which is worse than not having it.
- **`visibilitychange` and `blur` both fire for one switch.** "Away" is
  reported on the leading edge only, and the hub guards against a duplicate as
  well. Removing either guard double-counts every tab switch.
- **The client reports its own away duration.** The hub timing it would be
  meaningless across a reconnect. A candidate could under-report, but one who
  is editing the payload is already past what this claims to catch.

**The pasted text itself is captured**, capped at `MaxPasteChars` (2000) and
flagged `truncated` when cut — an interviewer must not read a fragment as the
whole paste. It is read back out of the Monaco model rather than the clipboard:
no permission prompt, and it captures what actually landed in the document.
A count alone was useless in practice; "pasted 12 lines" does not say whether
it was a test case or a finished solution.

The hub keeps a bounded log (`MaxActivityEvents`, 50) with **server-stamped**
times — the browser's clock is not ours to trust for a record somebody may rely
on. The log goes to **interviewers only**: a candidate has no business reading
the record being kept about them mid-interview.

The interviewer reads it in an **Activity tab** beside Prompt, not a badge. The
first attempt was a pill in the top bar and it was missed entirely — too small,
wedged between the avatars and the Run button, and `hidden md:inline-flex` on
top of that. Only "Away" now shows in the bar, because that is the one state
worth seeing without looking.

**The candidate always sees a banner saying what is tracked**, and it names the
paste capture explicitly — collecting content while disclosing only "when you
paste" would be the kind of half-truth that makes the whole feature
indefensible. That is the
feature, not a compliance checkbox: someone who knows the interviewer can see
tab switches does not switch, so visible monitoring *prevents* what silent
monitoring merely catches. It also gives them standing to explain a false
positive rather than being quietly marked down for one.

Interviewer activity is never relayed — it is their own business, and it would
put noise in front of the person meant to be reading the signal.

### Shared pointers

`components/PointerLayer.tsx` shows the other person's mouse. "This line here"
is most of what gets said in an interview, and without it that has to be typed.

- **Off by default**, toggled in the top bar. A cursor drifting across the
  screen while somebody is thinking is a real distraction; it earns its place
  when two people are looking at the same code, not the rest of the time.
- **Fractions of the viewport, not pixels.** The two people have different
  window sizes, so a pixel position lands somewhere else on the other screen.
- **Throttled to 60ms.** Mouse moves fire faster than anyone can see, and every
  frame passes through the room's single hub goroutine — this is about not
  flooding the room, not about bytes.
- **Never stored.** A pointer is stale the instant it arrives, so there is
  nothing sensible to put in a snapshot. Out-of-range values are dropped rather
  than clamped, so a bad client cannot park a cursor off-screen.
- **The fade is CSS**, restarted by changing the element's key. No timer, no
  state, no re-render per mouse move — see `.pointer-ghost` in `globals.css`.
- `pointer-events-none` throughout. This floats over Monaco, and a layer that
  swallowed clicks would make the room unusable.

### Pricing, plans and the clock

Priced on interview time, not seats. A live room holds a hub goroutine and a
sandbox, and that is the thing that actually costs something; a seat licence
would punish a small team that interviews occasionally, which is exactly who
this is for.

`internal/plan` is the single definition. If the marketing page and this
disagree, the page is wrong and somebody finds out by being refused.

| | Free | Pro | Enterprise |
|---|---|---|---|
| Interviews | 2 **for life** | 30 / month | unlimited |
| Each | 10 min | 60 min | unlimited |
| Price | — | $10/mo | $0.50 per interview-hour |
| Unit | — | ~$0.33/hr | $0.50/hr |

Enterprise costs **more** per unit than Pro, which is the right shape: you pay
a premium for not committing. It also means Pro is the better deal right up to
its ceiling, after which Enterprise is the only thing that fits.

**The clock starts when the candidate arrives**, not when the room is created
and not when the interviewer opens it. A room booked on Monday and held on
Friday gets its full time, and an interviewer opening early to write the
question does not burn the candidate's minutes. `StartRoom` uses
`COALESCE(started_at, now())` so two people arriving in the same millisecond
cannot both stamp it — the first write wins and the second is a no-op rather
than moving the deadline.

**The deadline reaches the hub through the `Grant`** an authorizer returns.
The authorizer is already looking the room up, so asking separately would be a
second query per join and the two answers could disagree. The first client
through sets it; later ones are ignored, or a second joiner could extend an
interview by reporting a deadline further out. Pinned by
`TestLaterJoinCannotExtendTheDeadline`.

**The deadline rides on the `Client`, not on a channel of its own.** It used to
go in as a separate `SetDeadline` message, and that was a race: two sends into
the same `select` have no defined order, so the hub could register the client
and build its snapshot *before* reading the deadline. The first joiner then got
a room with no countdown, and somebody reopening a finished interview was told
it was still running. `applyDeadline` now runs inside the register case —
before the client is added to the set, so an interview that is already over is
reported by that client's own snapshot rather than by an `ended` frame arriving
ahead of it.

CI is what found this. It failed one push and passed the pull request on the
**same commit**, which is the signature of a race rather than a break; the
local repro is `GOMAXPROCS=2 go test -race -count=30 ./internal/ws/`. Two
sends that must be ordered are a bug even when they are ordered in the source,
so if a third thing ever needs to arrive with a join, put it on the client too.

**On expiry the room goes read-only, and nobody is disconnected.** Cutting the
sockets would leave both people staring at a reconnect spinner with no idea
why; an `ended` frame lets the UI say what happened, and they can still read
and copy the code. Edits after that are dropped silently — the client has
already been told, and an error per keystroke would be noise on top of it.

Two things about the numbers:

- **Duration is clamped, not rejected.** Somebody on Free asking for an hour
  wants an interview; ten minutes with the limit shown beats an error telling
  them to try again with a smaller number.
- **The allowance is counted from the `rooms` table**, not a separate counter.
  The rooms are the record, and a counter would be one more thing to keep in
  step with them. Exhausting it answers **402**, not 403 — this is not a
  permission problem, and the difference tells the client whether to offer an
  upgrade or an apology.

`plan.ByName` falls back to Free for anything it does not recognise, so a row
written by a newer build fails closed.

### Owners and the admin UI

**`OWNER_EMAILS` is a comma-separated list of the operator's own addresses.**
Those accounts resolve to `plan.Unlimited` and reach the admin routes.

It lives in the environment rather than in an `is_admin` column, and that is
the whole point. **A column can only be set by somebody who can already write
to the database** — and on a managed host with no shell and no SQL console,
that is a cycle with no way in. Render grants neither on the free tier, so an
environment variable is the only bootstrap available. It also survives the
database: the free Postgres expires on a timer and takes every row with it, and
an owner defined in the environment is still an owner afterwards.

Empty means **nobody**, so a deployment that forgets the variable has no
privileged account rather than an accidental one. Matching is case-insensitive,
because `users` enforces uniqueness on `lower(email)` and the two must agree.

`api.effectivePlan` now reads three sources, most specific first:

```
owner list  ->  live promotion  ->  users.plan
```

The owner winning outright matters at exactly one moment: an owner whose promo
grant lapsed must not fall back to Free, because that is precisely when they
need to get in and fix something. Pinned by
`TestOwnerOutranksALapsedPromotion`.

`isAdmin` is separate from the plan and only reads the owner list. **An
unlimited *plan* must never imply administrative access** — a comped customer
is on the same tier as the operator and must not be able to see other people's
accounts. `TestUnlimitedPlanDoesNotImplyAdmin` exists to stop that being
"simplified" into one check.

| Route | Purpose |
|---|---|
| `GET /api/admin/promo` | codes with who claimed each |
| `POST /api/admin/promo` | issue one |
| `DELETE /api/admin/promo/{code}` | revoke; `?grants=revoke` also strips grants |
| `GET /api/admin/users` | accounts, effective tier, rooms, minutes |
| `PATCH /api/admin/users/{id}` | change a subscription |

`RequireAdmin` mounts **inside** `RequireAuth` — that is what puts the user in
the context for it to read — and answers **404, not 403**, matching the room
routes. A 403 confirms the endpoint is real and that somebody gets through it.

Three things worth keeping:

- **Deleting a code leaves its grants alone by default.** Stopping new claims
  and taking back access somebody is relying on are different decisions;
  `?grants=revoke` is opt-in and the UI asks as a second, separate question.
- **`PATCH` had to be added to the CORS allowlist.** The same omission bit
  `PUT` when the prompt route landed — a method missing there fails every
  preflight and looks like a broken endpoint rather than a config line.
- **`SetUserPlan` writes `users.plan` only.** An account with a live promotion
  keeps being served by it, so the admin UI shows subscription and effective
  tier side by side; otherwise an operator changes the subscription, sees no
  effect, and changes it again.

The web side is `/admin` (`components/Admin.tsx`), linked from the dashboard
header only when `is_admin` is set. That flag rides on `/api/me` and is a
**rendering hint** — the server is the boundary.

Still done with SQL, because there is no UI for it: making somebody else an
admin (add them to `OWNER_EMAILS` and redeploy).

### Promotion codes

Some people are given the product: pilot customers, universities, anyone being
shown it. A promotion code is how — they register, redeem a code, and their
limits lift with no card and no subscription.

`POST /api/promo/redeem` takes `{"code"}` and answers with the **same shape as
`/api/me`**, so the client swaps its user wholesale rather than refetching to
find out what changed.

**The grant overrides the subscription, it does not replace it.** `users.plan`
stays whatever they pay for and `users.promo_plan` sits beside it; when the
grant lapses they fall back to their subscription instead of being dropped to
Free. Everything reads `api.effectivePlan`, and a second place that reads
`u.Plan` directly is how a comped account quietly stops being comped.

A lapsed grant is left in the row rather than swept, for the same reason an
expired session is: the check is on the read path, so a stale grant is inert,
and clearing it would make every request a write.

| Column | Means |
|---|---|
| `plan` | tier granted; `unlimited` unless you want "3 months of Pro" |
| `max_redemptions` | how many people may claim it; **0 = no ceiling** |
| `expires_at` | when the *coupon* stops working; NULL = never |
| `grant_days` | how long the *grant* lasts once claimed; NULL = forever |

`plan.Unlimited` is a separate tier from Enterprise on purpose. Both are
uncapped, but Enterprise is *metered and billed* per interview-hour, and an
invoicing run has to tell a paying account from a comped one.

**The code is stored in plain text**, unlike session and invite tokens. A token
identifies a person and must be useless to whoever steals the database; a promo
code is a coupon — printed on a slide, typed by hand, often shared with a whole
cohort deliberately. Hashing it would mean an operator could never read back a
code they issued or write one with a plain INSERT, and the blast radius of a
leak is free interview minutes, which `max_redemptions` and `expires_at`
already bound.

Four things that are load-bearing:

- **The whole redemption is one transaction over a locked coupon row.** Every
  interesting failure here is a race — two people claiming the last seat, or
  one person double-clicking Redeem. `SELECT ... FOR UPDATE` is what makes the
  ceiling a real ceiling. Pinned by
  `TestRedemptionCeilingHoldsUnderConcurrentRedeems`, which races ten accounts
  at a three-seat code.
- **The primary key on `promo_redemptions` is the point of that table.**
  Without it a code with `grant_days` set could be re-redeemed every morning to
  push the expiry out forever, turning a 30-day trial into a permanent one.
- **The tier is vetted inside the transaction, before any write.** An
  end-to-end run caught the opposite: checking after the commit meant a typo in
  a hand-written `plan` column burned the one redemption that account would
  ever get, overwrote the grant it already had, and *then* answered 500.
  `plan.Grantable` is passed down as a func so `store` stays ignorant of what a
  plan is. Pinned by `TestMisconfiguredCodeCostsTheRedeemerNothing`.
- **Redemption is rate limited per account**, 10 failures an hour, in memory.
  A promo code is guessable in a way a session token is not — it has to be
  short enough to type — and the prize is unmetered use. Per account because
  the route is behind auth and an IP is shared by a whole office; in memory
  because a failed guess must not cost a write, which would make the defence
  the amplifier. It resets on restart, so codes should still be long enough not
  to fall to a burst.

Codes are normalised — upper-cased, all whitespace stripped — so `syncr-pilot`,
`SYNCR - PILOT` and a trailing newline are one coupon. They are read off slides
and out of emails; refusing those would be a support ticket per customer.

**Issuing one is a SQL INSERT.** There is no admin UI, and adding one means
building an admin role, which is a bigger piece of work than the feature:

```sql
-- unlimited, for 25 people, no end date
INSERT INTO promo_codes (code, plan, max_redemptions, note)
VALUES ('SYNCR-PILOT', 'unlimited', 25, 'launch pilot');

-- 90 days of Pro, one claim
INSERT INTO promo_codes (code, plan, max_redemptions, grant_days, note)
VALUES ('UNI-CS-2026', 'pro', 1, 90, 'placement office');
```

Revoking is deliberate and separate: `DELETE FROM promo_codes` stops new claims
but leaves grants already handed out, because deleting a leaked coupon should
not quietly rewrite them. Taking those back is
`UPDATE users SET promo_plan = NULL WHERE promo_code = '...'`.

**There is no billing.** `users.plan` is the entire subscription system; move
somebody to Pro with an UPDATE. Stripe is a separate piece of work needing a
company entity and tax setup, and is well beyond a pilot.

### Still stubbed
- Nothing of the *document* persists. A room's text lives only in its hub
  goroutine and dies when the last client leaves — pinned by
  `TestReopenedRoomStartsEmpty`. The room record survives; its contents do not.
- ~~Run output is not shared.~~ **Done.** A `result` frame is relayed by the
  hub, so both sides see the output. It is deliberately *not* folded into the
  document — run output is not something either person edits, and writing it
  into the shared text would fight with whoever is typing — and it is not
  replayed to late joiners, because a result is a moment rather than state.
  Both properties are pinned by tests in `internal/ws/result_test.go`.
- Language selection is per-client, not synced, so two people in one room can
  submit the same source under different languages. The room now *has* a
  language column, so the fix is to make the client honour it.
- Rate limiting is entirely in memory, so it resets on restart and does not
  add up across instances. Login (10 per 5 min) and register (5 per 10 min)
  are limited by IP; promo redemption is per account. That is enough for a
  single instance — a second one would need Redis, which is already in the
  design for WS fan-out.
- Making somebody an admin still means editing `OWNER_EMAILS` and
  redeploying. There is no way to grant it from the UI, deliberately — the
  environment is what makes the bootstrap work at all.
- Expired sessions are swept only by an explicit `DeleteExpiredSessions` call,
  which nothing schedules yet. Harmless — the expiry is enforced in the query,
  so a stale row is unusable, not dangerous.

### Continuous integration

`.github/workflows/ci.yml`, on every pull request and on pushes to `main`.
Deliberately *not* every push to every branch — a branch with an open pull
request built twice for one result, reported side by side as "CI / Go (push)"
and "CI / Go (pull_request)". `main` still builds on push because the merge
commit is what deploys and no pull request tested that commit as such.
Three parallel jobs: **Go** (gofmt gate, vet, build, `go test -race ./...`),
**Next.js** (`npm ci`, `tsc --noEmit`, eslint, `next build`), and
**govulncheck**.

Two things it does that a local run cannot, and they are the reason it exists:

- **`-race` is simply the default.** The detector needs CGO and a gcc; on this
  machine that means the mingw dance under "Toolchain setup" below, so in
  practice it was run when somebody remembered. A Linux runner has a toolchain,
  so every push now gets it.
- **The store tests never skip.** A Postgres service container supplies
  `TEST_DATABASE_URL`, and because the container is new for every run, the
  migrations are applied **from an empty schema every time**. Locally they only
  ever run against a database that already has them — so a migration that is
  broken from zero is invisible until a deploy. This is the check that catches
  it.

Details worth not undoing:

- **govulncheck is a separate job, not a step in the Go one.** A newly
  disclosed CVE in a dependency would otherwise fail the job before the tests
  ran, hiding a real regression behind news about somebody else's code. They
  are read differently and should fail separately.
- **The gofmt step turns a file list into an exit code by hand.** `gofmt -l`
  exits 0 whether or not it found anything, so `run: gofmt -l ./...` would
  pass forever while printing the problem.
- **Postgres is on 5432 in CI, 5433 locally.** The local mapping avoids
  clashing with a developer's own Postgres; a runner has none to clash with.
- **`npm ci`, not `npm install`** — it fails when `package.json` and the
  lockfile disagree, which is what makes the run reproducible.

Dependabot (`.github/dependabot.yml`) opens grouped weekly pull requests for
gomod, npm and github-actions. Grouped because ungrouped daily updates produce
a stream nobody reads, which is worse than no automation.

**CI does not gate deployment.** Render and Vercel build from the push itself,
so a red run still ships. Render's service settings can be told to wait for
checks; that is a dashboard toggle, not something in this repo.

### Testing note

`go test ./...` covers snapshot-on-join, author exclusion, presence, room
isolation, room lifecycle, edit serialisation under concurrent writers, and
slow-client eviction.

**Never write to one socket and then immediately dial another.** There is no
ordering between two independent connections: the frames are still in flight
while the new connection does its handshake, so the hub can build the joiner's
snapshot before it has read anything the first client sent. That is not a bug
in the hub — imposing an order across connections would mean blocking joins
behind other people's traffic — but eight tests were written as if it were.

They failed roughly one CI run in three, on a *different* test each time,
which is what made it look like several unrelated problems. Every one of them
now puts a **witness** in the room first: the hub mutates its state and only
then relays, so a witness that has *received* the relay proves the state is
applied, and the hub being a single goroutine means any join it handles
afterwards must see it.

Four of the eight failed outright. The other four asserted an *absence* —
"a candidate joining gets no event log" — and so passed for the wrong reason
whenever the race went the other way, which is worse: they would have kept
passing if the code had started leaking the log. Both kinds are fixed.

Reproduce with `GOMAXPROCS=2 go test -race -count=80 ./internal/ws/`. A
developer machine with idle cores never loses these races; a shared CI runner
with three jobs on it does.

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

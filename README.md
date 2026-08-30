# SyncR

A collaborative code editor for technical interviews. An interviewer creates a
room and sends a link; the candidate opens it and starts typing. Both sides see
the same document live, and code runs for real in a sandbox.

Built as a leaner alternative to CoderPad and HackerRank, for startups, small
teams and university placement programmes.

- **Frontend** — Next.js (App Router) + TypeScript + Tailwind, Monaco editor
- **Backend** — Go: a hand-built WebSocket hub, one goroutine per room owning
  that room's document, plus a REST API
- **Storage** — PostgreSQL for users, sessions and rooms
- **Execution** — Judge0, in its own containers, never in the Go process

`CLAUDE.md` holds the detailed architecture notes and the reasoning behind the
decisions that are easy to undo by accident. Read it before changing the hub,
the auth model, or the design system.

## Running locally

```bash
# 1. Application database — users, sessions, rooms
docker compose -f docker-compose.app.yml up -d --wait

# 2. Backend — WebSocket hub + REST on :8080. Migrations run at startup.
cd backend && go run ./cmd/server

# 3. Frontend — http://localhost:3000
cd web && npm install && npm run dev
```

Then open http://localhost:3000/register, create an account, and make a room.

### Code execution (optional, and not possible on Windows)

```bash
docker compose up -d          # Judge0 — a 14.1GB image, be patient
curl http://localhost:2358/about
```

**Judge0 cannot execute anything on Docker Desktop for Windows.** It sandboxes
with isolate 1.8.1, which speaks only cgroup v1, and Docker Desktop's WSL2 VM
is cgroup v2 only — every submission returns `Internal Error`. It needs an
amd64 Linux host with cgroup v1 (Ubuntu 20.04 by default; on 22.04+ set
`GRUB_CMDLINE_LINUX="systemd.unified_cgroup_hierarchy=0"` and reboot). The full
diagnosis, and the four workarounds that do not work, are in `CLAUDE.md`.

Without it the app runs fine; the Run button returns 502.

### Tests

```bash
cd backend
go test ./...

# Store tests need a real Postgres and skip without this
TEST_DATABASE_URL='postgres://syncr:syncrdev@localhost:5433/syncr' \
  go test ./internal/store/

# The race detector needs CGO and a gcc — see CLAUDE.md for the local path
CGO_ENABLED=1 go test -race ./...
```

## Deploying

The two halves deploy separately: **Vercel** for the frontend, **Render** for
the Go backend and Postgres. `render.yaml` describes the backend half.

Order matters, because each side needs the other's URL.

**1. Render** — New → Blueprint → this repository. It creates `syncr-api` and a
free `syncr-db` from `render.yaml`. Set `CORS_ORIGIN=*` for now. Wait for the
first deploy; the schema creates itself, since migrations run at startup. Note
the URL, e.g. `https://syncr-api.onrender.com`.

**2. Vercel** — import the same repository and **set the Root Directory to
`web`**. The repo root is not the Next.js app, and this is the step that is
easy to miss. Add:

```
NEXT_PUBLIC_API_URL=https://syncr-api.onrender.com
NEXT_PUBLIC_WS_URL=wss://syncr-api.onrender.com
```

Deploy, and note the URL.

**3. Render again** — set `CORS_ORIGIN` to the Vercel production URL. That one
variable gates both REST CORS and the WebSocket origin check, so it is what
admits the browser to the socket. It is comma-separated; add Vercel preview
origins too if you want branch deploys to work.

`NEXT_PUBLIC_*` values are baked in at build time, so changing them later needs
a Vercel redeploy, not just a settings save.

### What the free tier costs you

| | |
|---|---|
| Postgres | **Expires 30 days after creation.** Accounts and rooms go with it. Upgrading that one database is the whole fix. |
| Web service | Sleeps after 15 minutes idle, ~1 minute to wake. An active interview keeps it awake. |
| Run button | Returns 502. Judge0 cannot run on any PaaS — no privileged containers. |

Everything else works: accounts, rooms, shareable candidate links, live
collaborative editing and presence.

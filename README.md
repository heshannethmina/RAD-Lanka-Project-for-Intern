# SyncR

[![CI](https://github.com/heshannethmina/RAD-Lanka-Project-for-Intern/actions/workflows/ci.yml/badge.svg)](https://github.com/heshannethmina/RAD-Lanka-Project-for-Intern/actions/workflows/ci.yml)

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

## Becoming the admin

Set **`OWNER_EMAILS`** on the backend service to your own address:

```
OWNER_EMAILS=you@example.com
```

On Render that is Environment -> Add Environment Variable on the `syncr-api`
service, then Save (which redeploys). Comma-separate for more than one.

That address then gets unlimited interviews and an **Admin** link in the
dashboard header, with **nothing written to the database** — which is the
point. An `is_admin` column could only be set by somebody who can already
write to the database, and Render grants neither a shell nor a SQL console on
the free tier. It also survives the free Postgres expiring and taking every
row with it.

Leave it unset and nobody is an admin, which is the right default.

From `/admin` you can issue and revoke promotion codes, see who claimed each
one, and change any account's plan — no SQL for any of it.

## Giving somebody free access

Pilot customers, universities and anyone being shown the product get a
promotion code instead of a subscription. They register, enter it under
**Have a promotion code?** on their interviews page, and their limits lift.

The admin UI at `/admin` is the normal way to issue one. If you would rather
do it in SQL, or need to before you have set `OWNER_EMAILS`:

```sql
-- unlimited interviews and unlimited length, for 25 people, no end date
INSERT INTO promo_codes (code, plan, max_redemptions, note)
VALUES ('SYNCR-PILOT', 'unlimited', 25, 'launch pilot');

-- 90 days of Pro, claimable once
INSERT INTO promo_codes (code, plan, max_redemptions, grant_days, note)
VALUES ('UNI-CS-2026', 'pro', 1, 90, 'placement office');
```

`max_redemptions` of 0 means no ceiling; a NULL `grant_days` means the grant
never lapses; `expires_at` bounds the *code* rather than the grant. Codes are
matched upper-cased and with whitespace stripped, so `syncr-pilot` works.

Who has claimed what:

```sql
SELECT r.code, u.email, r.redeemed_at
FROM promo_redemptions r JOIN users u ON u.id = r.user_id
ORDER BY r.redeemed_at DESC;
```

Deleting a code stops new claims but leaves grants already given, on purpose.
To take those back as well:

```sql
UPDATE users SET promo_plan = NULL, promo_expires_at = NULL
WHERE promo_code = 'SYNCR-PILOT';
```

## Continuous integration

`.github/workflows/ci.yml` runs on every push and on pull requests to `main`.
Three jobs, in parallel:

| Job | Runs |
|---|---|
| **Go** | `gofmt` check, `go vet`, `go build`, `go test -race ./...` |
| **Next.js** | `npm ci`, `tsc --noEmit`, `eslint`, `next build` |
| **govulncheck** | reachable vulnerabilities in Go dependencies |

Two of these are stronger in CI than they can be locally:

- **`-race` always runs.** It needs CGO and a gcc, which on Windows means
  installing mingw-w64 by hand and exporting `PATH` first — so locally it gets
  run occasionally. A Linux runner has a toolchain already.
- **The store tests never skip.** They need a real Postgres and skip without
  `TEST_DATABASE_URL`; CI provides one as a service container. Because that
  container is new every run, **the migrations are applied to an empty schema
  every time**, which is the one thing a local run never checks.

Dependabot (`.github/dependabot.yml`) opens grouped weekly update pull
requests for Go, npm and the actions themselves, so every upgrade goes through
the suite above before anyone looks at it.

Nothing here gates deployment. Render and Vercel both build straight from a
push, so a red CI run still ships. Render's service settings have an option to
wait for checks before deploying; turning it on is the one manual step that
closes that gap.

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

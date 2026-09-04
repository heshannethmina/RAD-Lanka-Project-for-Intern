# Contributing to SyncR

Thanks for looking. This document is the practical half: how to get SyncR
running, what the checks expect, and where a conversation belongs.

The *reasoning* behind the design lives in [`CLAUDE.md`](CLAUDE.md) — why one
goroutine owns each document, why tokens are not JWTs, why Python runs in the
browser. It is worth reading before proposing a change to any of that. Several
things in this codebase look like accidents and are not.

---

## Where things go

| | Use |
|---|---|
| A bug, with steps to reproduce | **Issue** |
| A concrete, scoped feature request | **Issue** |
| A task or piece of tech debt | **Issue** |
| "How do I…", "why does it…" | **Discussion → Q&A** |
| A half-formed idea | **Discussion → Ideas** |
| A design that needs agreement before code | **Discussion → Design Proposals** |
| A security vulnerability | **[Private advisory](https://github.com/heshannethmina/SyncR/security/advisories/new)**, never an issue |

The rule of thumb: an issue should have an answer to "how would we know this is
done". If it does not yet, it is a discussion.

### Design proposals

Anything that changes the WebSocket protocol, the database schema, the
authorization model, or the concurrency design should start as a **Design
Proposal** discussion rather than as a pull request. Writing it down first is
cheaper than discovering in review that the approach was wrong.

Proposals move through labels: `Proposal/Draft` → `Proposal/Review` →
`Proposal/Approved` → `Proposal/Implemented` (or `Proposal/Rejected`). Once
approved, open issues for the implementation and link them back to the
discussion.

---

## Getting it running

You need **Go**, **Node 20+**, and **Docker**. You do not need Judge0 — see the
warning below.

### 1. The application database

```bash
docker compose -f docker-compose.app.yml up -d --wait
```

**Note the `-f`.** Plain `docker compose up` starts the *Judge0* stack, which is
a different thing entirely with its own Postgres. Starting the wrong one is the
most common way to end up confused about why nothing works.

Postgres binds **5433** locally, not 5432, so it does not collide with a
Postgres you may already be running. CI uses 5432, where there is nothing to
collide with.

### 2. The backend

```bash
cd backend && go run ./cmd/server
```

Listens on `:8080`. Migrations run automatically at startup, so an empty
database populates itself.

### 3. The frontend

```bash
cd web && npm ci && npm run dev
```

Listens on `:3000`. Use `npm ci`, not `npm install` — it fails when
`package.json` and the lockfile disagree, which is what keeps everyone on the
same dependency tree.

### Judge0 does not work on Docker Desktop for Windows

Do not spend a day on this. Judge0 1.13.x sandboxes with isolate 1.8.1, which
speaks **cgroup v1 only**, and Docker Desktop's WSL2 VM is cgroup v2 only. Every
submission fails with `Failed to create control group`. Four workarounds have
been tried and all four failed; the details are in `CLAUDE.md`.

You do not need it. **Python runs entirely in the browser** through Pyodide, so
the Run button works without any sandbox. Only Go and JavaScript go to Judge0,
and those need a Linux amd64 host with cgroup v1.

---

## Running the checks

CI runs all of these on every pull request, so you are not required to run them
locally. Running them first is faster than waiting for a red run.

```bash
cd backend && go test ./...          # the Go suite
cd web && npx tsc --noEmit           # types
cd web && npm run lint               # eslint
```

Two of the checks have traps worth knowing about.

**The store tests skip themselves silently** without a database URL. If you have
touched anything under `backend/internal/store/`, run them properly:

```bash
TEST_DATABASE_URL='postgres://syncr:syncrdev@localhost:5433/syncr' \
  go test ./internal/store/
```

Without that variable they pass by not running, and it is easy to believe you
ran the suite when you ran two thirds of it.

**The race detector needs CGO and a gcc**, which Windows does not have by
default. CI runs `-race` on every pull request on Linux, so you do not strictly
need it locally. If you have touched `backend/internal/ws/`, run it anyway:

```bash
GOMAXPROCS=2 go test -race -count=30 ./internal/ws/
```

The low `GOMAXPROCS` and the repeat count are the point. A race in the hub
passes happily on an idle developer machine and fails on a loaded CI runner;
this is the incantation that reproduces those locally.

### The test only a person can run

**Open the room in two browsers.** Half the behaviour in this project only
exists with two clients connected, and nothing in CI covers it. Sign in as the
interviewer in one window, open the invite link in a second (a private window
works), and check that:

- typing in one appears in the other,
- the cursor of the person *not* typing does not jump,
- Run output appears on both sides,
- the candidate path works through the link, not just the interviewer's view.

---

## Pull requests

**One branch per change, branched from `main`.** Merged branches are kept
rather than deleted, so the history of a change stays reachable after the squash
commit replaces it — but a branch is still finished once its pull request
merges. Start a new one for the next piece of work instead of reusing an old
one: a long-lived branch drifts from `main`, and two people cannot share it.

```bash
git switch main && git pull
git switch -c fix/short-description
```

**The title must be a conventional commit.** Merges are squashes and the squash
message is the pull request title, so the title is literally what lands on
`main`. A workflow checks it:

```
feat: sync the room language across clients
fix: stop the caret jumping on a remote edit
docs: explain the two-browser check
chore: bump the actions group
```

Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`,
`chore`, `revert`. The subject must start lowercase — `fix: handle …`, not
`fix: Handle …`.

**Link the issue.** Put `Closes #123` in the description, not just in a comment.
Closing keywords only fire at merge time; adding one afterwards does nothing.

**What review expects.** `main` requires a passing CI run (Go with `-race`,
Next.js, govulncheck, CodeQL) and one approving review from a code owner. Review
threads must be resolved before merge. Keep pull requests small — a large one is
not reviewed more thoroughly, it is reviewed less thoroughly.

---

## Conventions

**Go.** Standard layout (`cmd/`, `internal/`). Errors are returned, not
panicked, for anything expected. `gofmt` is a CI gate.

**The concurrency rule, which is the one that matters.** If you are reaching for
a `sync.Mutex` to protect room or document state, stop. That state is owned by a
single goroutine and mutated only through channel messages, and it is why the
hub is correct without operational transformation. Share memory by
communicating, not the other way around. A change that introduces a mutex there
needs an argument in the pull request, not just a passing test.

**If you add shared room state, it has to appear in the snapshot** a late joiner
receives — otherwise the second person in the room sees a different room from
the first.

**`internal/ws` must not import `store`.** Authorization arrives as a
`ws.Authorizer` function so the hub stays a thing that moves text between
sockets.

**Never execute submitted code in the Go process.** It goes to Judge0 or it runs
in the candidate's browser. If a change makes shelling out tempting, that is the
wrong turn.

**Frontend.** TypeScript strict mode, Tailwind for styling, no CSS-in-JS.

---

## Triage

New issues get `needs-triage` automatically, and it clears when a maintainer
sets a `Priority/` label. Area labels (`backend`, `frontend`, `websocket`, …)
are applied from the issue form's own dropdown, so filling that in accurately
saves everyone a step.

`good first issue` marks work that is genuinely self-contained — no cross-stack
knowledge and no unstated context. If you pick one up, say so in a comment so
two people do not do it twice.

## Code of conduct

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

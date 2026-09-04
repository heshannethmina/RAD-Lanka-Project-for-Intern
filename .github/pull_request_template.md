## What this changes

<!-- One or two sentences. The title carries the summary; this is for the
     reasoning that will not fit in it. -->

Closes #

## Why

<!-- What was wrong before. If this reverses a decision written down in
     CLAUDE.md, say so explicitly and say why: several of those choices look
     like accidents and are not. -->

## How it was verified

<!-- Delete what does not apply. CI already runs the Go suite with -race, tsc,
     eslint and a Next build on every pull request, so this section is for what
     CI cannot do: what you actually clicked. -->

- [ ] `cd backend && go test ./...`
- [ ] Store tests against a real Postgres (`TEST_DATABASE_URL=...`)
- [ ] `cd web && npx tsc --noEmit && npm run lint`
- [ ] Opened a room in **two browsers** and confirmed live sync still works
- [ ] Checked the candidate path through an invite link, not just the interviewer's

## Notes for the reviewer

<!-- Anything you are unsure about, or deliberately left out. -->

---

<!-- Two things that are easy to trip over here, kept in the template because
     they are cheap to check and expensive to miss:

     * If you touched backend/internal/ws, run the race detector before
       pushing. A race there passes on an idle laptop and fails on a busy CI
       runner: GOMAXPROCS=2 go test -race -count=30 ./internal/ws/

     * If you added shared room state, it has to appear in the snapshot a late
       joiner receives, or the second person in the room sees a different room
       from the first. -->

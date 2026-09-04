# Security Policy

## Reporting a vulnerability

**Please do not open a public issue.**

Report it privately through GitHub's advisory form:

**https://github.com/heshannethmina/SyncR/security/advisories/new**

That creates a draft advisory only you and the maintainers can see, so a fix can
be prepared before the problem is described in public.

If you would rather not use GitHub, email the address in the repository owner's
GitHub profile and say "SyncR security" in the subject.

## What to expect

SyncR is maintained by one person, so please read these as honest estimates
rather than a service commitment:

| | |
|---|---|
| Acknowledgement | within 5 days |
| Assessment, and a rough fix timeline | within 14 days |
| Credit | in the advisory, unless you ask otherwise |

If you have had no reply after two weeks, assume the message was missed and
send it again rather than concluding it was ignored.

## Why this matters more than the size of the project suggests

SyncR does several things that are ordinarily considered dangerous, because the
product does not work otherwise:

- **It runs code that a stranger typed.** That is the entire point of it.
- **It holds session tokens in `localStorage`**, not in an HttpOnly cookie. That
  is a considered trade — the web app and the API are separate origins in every
  environment, so a cookie would need `SameSite=None; Secure` and break local
  development outright — but it does mean any XSS is also session theft.
- **An invite link admits an anonymous person to a room.** Anyone holding the
  link is in; candidates deliberately have no account.
- **Candidate paste content is captured** and kept in the room's hub. That is
  real text typed by a real person, so exposing it is a privacy incident and not
  only a bug.

A finding in any of those areas is worth reporting even if it looks minor.

## In scope

- The deployed application and its API.
- This repository: the Go backend, the Next.js app, the deployment
  configuration, and the GitHub Actions workflows.
- Dependencies, where SyncR's use of them is what creates the problem.

## Not vulnerabilities

Two categories come up often enough to name:

**The interview-integrity signals are detection only, and documented as
defeatable.** SyncR reports when a candidate switches tab or pastes. A browser
cannot prevent either — there is no API for it, deliberately — and anyone can
use a phone or a second machine. "I opened another tab and it did not stop me"
is the design, not a bug. So is under-reporting an away duration: the client
times its own absence, because timing it server-side would be meaningless
across a reconnect.

**Python output is not verified.** Python runs in the candidate's own browser
through Pyodide, so a determined candidate could alter the result before it is
shared. This is a deliberate trade: on a managed host there is no sandbox
available, and running the code in the candidate's own tab means there is
nothing of ours near it. In a live interview someone is watching. Verified
execution means Judge0 on a real Linux host, which is a deployment task.

Both are written up at more length in `CLAUDE.md`.

## Please do not

- Test against live interview rooms that are not yours. There may be a real
  candidate in one, being assessed for a real job.
- Run denial-of-service or load tests against the deployed instance.
- Access, modify or keep other people's data. If you stumble into some, stop and
  say so in the report.

Testing against your own local deployment is unrestricted, and is the best way
to look at any of this.

## Supported versions

There is one version: whatever is on `main` and deployed. There are no
maintained release branches and no backports.

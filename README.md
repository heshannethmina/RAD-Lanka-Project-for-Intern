<div align="center">
  <img src="web/public/syncr-logo.png" alt="SyncR" width="260" />

  <p><strong>A focused, real-time code interview workspace.</strong></p>

  <p>
    <a href="https://github.com/heshannethmina/SyncR/actions/workflows/ci.yml"><img src="https://github.com/heshannethmina/SyncR/actions/workflows/ci.yml/badge.svg" alt="CI status" /></a>
    <a href="#contributing">Contribute</a>
  </p>
</div>

## What is SyncR?

SyncR makes technical interviews feel natural: an interviewer creates a room, shares an invite link, and collaborates with a candidate in the same code editor in real time. Candidates can join from their browser—no installation or account required.

It is designed for startups, hiring teams, and university placement programmes that want a lightweight alternative to larger interviewing platforms.

## Product highlights

- Live shared code editing with room-based invite links
- Browser-based coding workspace powered by the Monaco editor
- Multi-language code execution via Judge0 when an execution service is connected
- Interview dashboard for creating and managing rooms
- Secure accounts, sessions, and room access
- Admin tools for promotion codes and account plans

## Built with

| Area | Technologies |
| --- | --- |
| Frontend | Next.js, React, TypeScript, Tailwind CSS, Monaco Editor |
| Backend | Go, Gorilla WebSocket |
| Data | PostgreSQL |
| Code execution | Judge0 |
| Deployment | Vercel and Render |
| Quality | GitHub Actions, ESLint, Go tests |

## Repository guide

| Path | Purpose |
| --- | --- |
| `web/` | Next.js application and the product interface |
| `backend/` | Go API, WebSocket collaboration service, and data layer |
| `docs/` | Project documentation and route map |
| `.github/` | Automated checks and dependency maintenance |

## Contributing

Contributions, bug reports, and product ideas are welcome.

1. Check existing issues or open one to discuss a substantial change.
2. Fork the repository and create a focused branch.
3. Keep each pull request small, clear, and accompanied by relevant tests.
4. Describe the user-facing impact in the pull request and link the related issue when there is one.

Before opening a pull request, please make sure the relevant frontend or backend checks pass. The automated CI workflow will validate both parts of the project again.

## Community

If you would like to help, a great place to start is improving the interface, documentation, accessibility, tests, or interview workflows. Please be kind, specific, and collaborative in issues and pull requests.

---

Built for better technical conversations.

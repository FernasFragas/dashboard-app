# ADR-002: Ship one Go binary with the frontend embedded via go:embed

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-001, ADR-003, docs/ARCHITECTURE.md |

## 1. Context and Problem Statement

The app is a Go API plus a Vite/React frontend, run on a personal Mac and reached from a phone
over Tailscale. It must survive reboots and start itself, and it must keep running for months
with as little attention as possible. Whatever runs it will be supervised by launchd, so the
number of things that have to be up and in the right order is a direct reliability cost.

This decides how the frontend reaches the browser in production. It does **not** decide the
dev-time workflow (Vite's dev server with an `/api` proxy is used regardless), nor the process
supervisor (M7).

## 2. Decision Drivers (ranked)

1. **One thing to start and restart.** Every additional process is a way to be half-up.
2. **No runtime dependencies on the host.** Node should not be needed to serve the app.
3. **Correct deep links.** `/goals` and `/?project=synapse` must work on a cold open from a
   phone home-screen icon.
4. **Iteration speed while building.** Hot reload must not be sacrificed.

## 3. Options Considered

- Option 0 — Two processes in production: Go API plus a static server for `dist`
- Option 1 — `go:embed web/dist` into the binary
- Option 2 — Go serves `dist` from disk at a configured path

### Option 0: Two processes

- **Description** — Run the Go API and a separate static file server (nginx, `serve`, Caddy),
  with the frontend calling the API across origins or via a proxy rule.
- **Pros** — Frontend can be redeployed without rebuilding Go. Familiar production shape.
- **Cons / risks** — Two launchd agents, two failure modes, and an ordering dependency (fails
  #1). Adds CORS or proxy configuration for no benefit. If it's `serve`, Node is now a runtime
  dependency (fails #2).
- **Cost** — Highest ongoing: two supervised processes and a proxy config to maintain.

### Option 1: `go:embed web/dist`

- **Description** — `make build` runs `pnpm build`, copies `dist` into the embedded directory,
  and compiles it into the binary. The binary serves `/api/*` and falls back to `index.html`
  for every other path.
- **Pros** — One artefact and one process (#1). No Node, no files, nothing to path-resolve at
  runtime (#2). SPA fallback lives in the same router as the API, so deep links are one handler
  (#3). Copying the binary to another machine copies the whole app.
- **Cons / risks** — Any frontend change requires a Go rebuild to appear in the binary. If
  `dist` is missing or stale at build time, the binary silently embeds nothing and serves a
  blank page — a genuinely confusing failure.
- **Cost** — A build tag, an embed directive, a Makefile step, and a fallback handler.

### Option 2: Go serves `dist` from disk

- **Description** — One process, but the frontend is read from a directory given by a flag.
- **Pros** — One process (#1) and no Node at runtime (#2). Frontend changes appear without a Go
  rebuild.
- **Cons / risks** — The binary is no longer self-contained: it depends on a directory being
  present, correct, and in sync with the binary's expectations. Moving or renaming the checkout
  breaks a running install, and the failure is a 404 rather than something loud.
- **Cost** — Lowest, but it trades a build-time guarantee for a runtime one.

### Comparison

| Driver (ranked) | Opt 0 Two processes | Opt 1 go:embed | Opt 2 Serve from disk |
| --- | --- | --- | --- |
| 1. One thing to start | ❌ | ✅ | ✅ |
| 2. No runtime deps | ⚠️ | ✅ | ⚠️ |
| 3. Deep links | ⚠️ | ✅ | ✅ |
| 4. Iteration speed | ✅ | ⚠️ | ✅ |

## 4. Decision

> **We will embed the built frontend into the Go binary with `go:embed`, because it collapses
> the entire deployment to one self-contained artefact (#1, #2) and puts the SPA fallback in the
> same router as the API (#3).**

We accept that a frontend change requires a Go rebuild. That cost is paid only at ship time:
during development, `make dev` runs Vite's dev server with an `/api` proxy, so hot reload is
untouched (#4). The build-time coupling is precisely what buys the runtime guarantee — a binary
that starts is a binary with a complete frontend inside it.

## 5. Consequences

**Positive**
- Deployment is `scp` the binary and restart. Rollback is the previous binary.
- No version skew between API and frontend: they are compiled together, so a stale cached
  frontend calling a changed API cannot happen from the server side.
- The `CGO_ENABLED=0` build from ADR-001 keeps this a single static file.

**Negative**
- A missing or stale `web/dist` at build time produces a binary that serves a blank page.
  *Mitigation:* the Makefile builds the frontend first and **fails loudly** if `dist` is absent;
  the embed directory is never committed.
- Frontend-only fixes need a full rebuild. *Mitigation:* accepted — `make build` is one command
  and this app is not deployed often.

**Follow-up work created**
- Makefile `build` target with the dist-missing guard (M7).
- SPA fallback handler serving `index.html` for non-`/api/` paths (M7).
- A `.gitignore` entry for the embedded dist directory.

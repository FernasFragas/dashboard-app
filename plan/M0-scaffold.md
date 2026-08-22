# M0 · Scaffold

**Depends on:** nothing. **Unblocks:** every other milestone.
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §2

Goal: an empty-but-wired project where a React page renders data fetched from the Go server,
and one command checks the whole thing.

## Deliverables

```
dashboard-app/
├─ .github/workflows/ci.yml
├─ .nvmrc                       24
├─ .gitignore                   web/dist, web/node_modules, bin/, *.db
├─ .golangci.yml
├─ Makefile
├─ go.mod                       module github.com/FernasFragas/dashboard-app
├─ cmd/server/main.go           flags: -addr -data -token -log-format
├─ internal/api/                health handler
├─ web/
│  ├─ package.json  pnpm-lock.yaml  vite.config.ts  tsconfig.json
│  ├─ eslint.config.js  .prettierrc  vitest.config.ts
│  ├─ index.html
│  └─ src/  main.tsx  App.tsx  index.css  api/client.ts
└─ README.md
```

## Backend

- `cmd/server/main.go`: `net/http` + `http.ServeMux`, one route `GET /api/health` returning
  `{"status":"ok","version":"<build>"}`.
- Flags: `-addr` (default `:8484`), `-data` (default `~/dashboard-data`), `-token` (default
  empty), `-log-format` (default `text`). Creates the data dir if missing.
- No router library, no store yet — M2 adds SQLite.

## Frontend

- Vite + React 18 + TypeScript, **Tailwind v4** (`@import "tailwindcss"` in `index.css`,
  `@tailwindcss/vite` plugin in `vite.config.ts`, no `tailwind.config.js`).
- `vite.config.ts` proxies `/api` → `http://localhost:8484`.
- `App.tsx` fetches `/api/health` via `src/api/client.ts` and renders the status — this is the
  proof the proxy works. Dark background only.

## make dev — one command, both servers

`make dev` starts Go on `:8484` and Vite on `:5173` **concurrently**, streams both log outputs
with a prefix per side, and a single Ctrl-C kills both (trap SIGINT, kill the process group —
no orphaned Vite left holding the port).

## Tooling — full tier

| Tool | Wired as |
|---|---|
| gofmt + `go vet` | `make fmt`, `make vet` |
| golangci-lint | `.golangci.yml` — enable `errcheck govet staticcheck revive ineffassign` · `make lint` |
| ESLint (flat config) + Prettier | `pnpm lint`, `pnpm format` |
| tsc `--noEmit` | `pnpm typecheck` |
| Vitest | `pnpm test` — one smoke test at M0 so the runner is proven |
| `go test ./...` | `make test` |
| **`make check`** | fmt + vet + lint + typecheck + test, both sides — the single pre-commit gate |
| GitHub Actions `ci.yml` | on push/PR to `main`: Go 1.26 + Node 24, runs `make check` |

## Done when

1. `make dev` → `http://localhost:5173` shows the React shell rendering the **live** health
   response (kill the Go process → the page shows an error, not a cached value).
2. `make check` passes clean on the empty scaffold.
3. CI is green on the first push.

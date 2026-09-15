# Dependencies

Every direct dependency, why it is here, and the behaviour you need to know before changing code
that uses it. Versions are pinned by `go.sum` and `web/pnpm-lock.yaml`; the ranges below are what
the manifests allow.

## Choosing a dependency

- Prefer the standard library. This project routes with `net/http`, logs with `log/slog` and embeds
  with `embed` on purpose (ADR-005, ADR-006, ADR-002).
- Anything new must be open source, readable, maintained, and must build with `CGO_ENABLED=0`.
- Say why the standard library or an existing dependency is not enough in the pull request, add a
  row here, and write an ADR if the choice shapes the architecture.

## Reading a dependency's source

- Go: `go doc modernc.org/sqlite`, or the source under `$(go env GOMODCACHE)`.
- Frontend: packages are installed under `web/node_modules/`; `pnpm why <package>` explains why
  something is installed.
- Read the pinned version's source rather than guessing from memory of another version.

## Go

| Module | Version | Why | Behaviour to know |
|---|---|---|---|
| `modernc.org/sqlite` | v1.57.0 | Pure-Go SQLite, so the binary builds with `CGO_ENABLED=0` (ADR-001). | Driver name is `sqlite`, not `sqlite3`. PRAGMAs go in the DSN as `_pragma=name(value)` so every pooled connection gets them. Errors are `*sqlite.Error` with `.Code()`; `internal/store/errors.go` maps them to sentinels. `PRAGMA foreign_keys` is a no-op inside a transaction. The store uses one connection (`SetMaxOpenConns(1)`), so nested queries inside a transaction deadlock. Backups use `VACUUM INTO`. |
| `github.com/skip2/go-qrcode` | pseudo-version from 2020 | Encodes the one-time pairing URL for the desktop QR code. | Used only by `qrSVG` in `internal/api/pair.go`. Unmaintained but small and stable; replace it rather than patch it if it ever breaks. |
| Standard library `net/http` | Go 1.26 | Routing without a framework (ADR-005). | Go 1.22+ patterns: `"PATCH /api/tasks/{id}"`, `r.PathValue("id")`. The most specific pattern wins. A pattern without a method matches every method. |
| Standard library `log/slog` | Go 1.26 | Structured logs (ADR-006). | Text handler for `-log-format=text`, JSON for `json`. Pass key/value pairs, never formatted strings. |

Transitive modules (`modernc.org/libc`, `github.com/google/uuid`, …) come in through the SQLite
driver. Do not import them directly.

## Frontend runtime

| Package | Range | Why | Behaviour to know |
|---|---|---|---|
| `react`, `react-dom` | ^18.3.1 | UI. | `React.StrictMode` in `web/src/main.tsx` runs effects twice in development; effects must be idempotent. |
| `@tanstack/react-query` | ^5.101.4 | Server state, cache, optimistic mutations. | v5 object syntax only. Defaults in `web/src/App.tsx`: `staleTime` 10 s, `retry` 1, no refetch on window focus. Keys shared between pages live in `web/src/lib/queryKeys.ts`. |
| `wouter` | ^3.10.0 | Six routes without a large router. | `Switch`, `Route`, `Link`, `useLocation`. Query strings are read by the pages themselves. |
| `@dnd-kit/core`, `@dnd-kit/sortable`, `@dnd-kit/utilities` | ^6.3.1, ^10.0.0, ^3.2.2 | Kanban drag on desktop. | Ids given to `SortableContext` must match `useSortable` ids. Collision detection is pointer-first with a rectangle fallback (`plan/M5-followups.md` F3). Unit tests do not exercise this wiring. |
| `lucide-react` | ^1.33.0 | Icons. | Import icons by name so unused ones are tree-shaken. |

## Frontend tooling

| Package | Range | Behaviour to know |
|---|---|---|
| `vite`, `@vitejs/plugin-react` | ^7.0.0, ^5.0.0 | `web/vite.config.ts` proxies `/api` to `DASHBOARD_API_URL` (default `http://localhost:8484`). Only `VITE_*` variables reach the browser. |
| `tailwindcss`, `@tailwindcss/vite` | ^4.1.0 | Version 4 is CSS-first: theme tokens live in `@theme` in `web/src/index.css`, and there is no `tailwind.config.js`. Some utility names differ from v3 (for example `size-11`). |
| `typescript` | ^5.8.0 | `strict` mode; `pnpm typecheck` runs `tsc --noEmit`. |
| `vitest`, `jsdom`, `@testing-library/*` | ^3.0.0, ^30.0.1 | `globals: true`; `web/src/test/setup.ts` loads jest-dom matchers; `vi.mock` calls are hoisted above imports. |
| `eslint`, `typescript-eslint`, React plugins | ^9.0.0 | Flat config in `web/eslint.config.js`, including the rule that keeps HTTP calls in the API client. |
| `prettier` | ^3.0.0 | `printWidth` 100, double quotes, trailing commas (`web/.prettierrc`). |

## Toolchain

| Tool | Version | Note |
|---|---|---|
| Go | 1.26.x | `go.mod` directive. |
| Node | 24.x | Pinned in `.nvmrc`. |
| pnpm | 11.22.0 | `packageManager` in `web/package.json`. |
| golangci-lint | 2.4.0 | Must be built with Go 1.26 (`go install`); release binaries built with go1.25 refuse this module. |
| jq | any | Used by the Claude Code hooks in `.claude/hooks/`. |

`make doctor` checks all of these.

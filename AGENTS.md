# AGENTS.md

Operating manual for coding agents, and for humans working the same way. It is a map: keep it
short and put detail in the linked documents.

## What this is

A single-user productivity dashboard that tracks a personal career plan (`master-plan-v5.md`):
weekly tasks, a goals Kanban, an accomplishment log, daily reviews, metrics, skills and XP. One Go
binary with the React frontend embedded, one SQLite file, reached from a laptop and a phone over
Tailscale. No accounts, ever.

## Commands

| Goal | Command |
|---|---|
| First setup, or check the toolchain | `make setup` · `make doctor` |
| Run the app on the real data directory (:8484 + :5173) | `make dev` |
| Run an isolated instance: free ports, throwaway DB, logs on disk | `make dev-instance` (see `scripts/dev-instance.sh --help`) |
| Everything CI runs | `make check-ci` |
| Go tests, all or one | `go test ./...` · `go test ./internal/store -run TestGoals` |
| Frontend tests, lint, types | `cd web && pnpm test` · `pnpm lint` · `pnpm typecheck` |
| Docs still point at real files | `make check-docs` |
| Validate and regenerate the plan seed | `go run ./cmd/server plan validate master-plan-v5.md` · `make seed-gen` |
| Release binary | `make build` |

## Rules

Each rule names what enforces it. "Review" means nothing automated catches it yet, so reviewers
must.

| # | Rule | Enforced by |
|---|---|---|
| 1 | **Two layers.** `internal/api` is transport with no SQL; `internal/store` is persistence and never logs. No service layer, interfaces or mocks (ADR-004). | depguard in `.golangci.yml` |
| 2 | **Errors are logged once**, at the API boundary; everything below wraps with `%w`. | depguard (no logger below `internal/api`) + review |
| 3 | **Docs lead code.** Schema changes start in `docs/DATABASE.md`, endpoint changes in `docs/API.md`. | `internal/store/schema_contract_test.go`, `internal/api/contract_test.go` |
| 4 | **Migrations are forward-only and immutable once merged.** | `scripts/check-migrations-immutable.sh` (CI), Claude Code hook |
| 5 | **Every table is exported** by `GET /api/export`, except documented exceptions. | `internal/store/schema_contract_test.go` |
| 6 | **Store facts, derive progress.** No stored counters, levels, tiers or streaks (ADR-007, ADR-008). | Review |
| 7 | **Plan vocabulary is data.** Never hardcode project, category, skill or week ids in app code. | Review |
| 8 | **Strict input.** Unknown JSON fields and unknown `GET` query parameters are `400`. | `internal/api/contract_test.go` (query), `decodeJSON` (fields) |
| 9 | **Time.** Store RFC3339 UTC; compute day boundaries in `Europe/Lisbon`; pass `now` and the location in. | Review; tests pin clocks |
| 10 | **Frontend HTTP goes through `web/src/api/client.ts` only.** | ESLint in `web/eslint.config.js` |
| 11 | **The plan markdown is parser input.** Validate → `make seed-gen` → choose reseed mode → tests. | `plan validate`, `TestParseRealMasterPlan` |
| 12 | **Tailscale is the perimeter.** Never bind `0.0.0.0` in production, never add accounts, never log tokens or pairing codes (ADR-003, ADR-010). | Review; `TestPairRequestLogRedactsCodes` |

## Operating limits

- Never run against, reset, migrate or reseed the live database in `~/dashboard-data/`. Use
  `make dev-instance` or `-data "$(mktemp -d)"`.
- Ask first before: `-plan-reset`, `launchctl`, deleting backups, a `/plan` replace, committing,
  pushing, merging.
- Never hand-edit `internal/seed/seed.json` (run `make seed-gen`), a committed migration (add a
  new one), or an ADR's decision (append an amendment).
- **Done** means: the task brief's acceptance checks pass, the checks for the change's risk level
  ran green, and self-review plus independent review findings are resolved or recorded. See
  [docs/agents/PLAYBOOK.md](docs/agents/PLAYBOOK.md).

## Documentation map

| Need | Read |
|---|---|
| How work flows: briefs, risk-based checks, review loop, autonomy levels, maintenance | [docs/agents/PLAYBOOK.md](docs/agents/PLAYBOOK.md) |
| Task brief template | [docs/agents/TASK_BRIEF.md](docs/agents/TASK_BRIEF.md) |
| Recipes: endpoint, schema change, page, achievement, plan edit, dependency, release | [docs/agents/RECIPES.md](docs/agents/RECIPES.md) |
| Architecture: components, layering, request flow, time, security | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Where each concern lives in the code; boot sequence | [docs/CODEMAP.md](docs/CODEMAP.md) |
| Product rules, domain glossary, tracked projects | [docs/PRODUCT.md](docs/PRODUCT.md) |
| Schema (source of truth) and API contract | [docs/DATABASE.md](docs/DATABASE.md) · [docs/API.md](docs/API.md) |
| Why each decision was made | [docs/adr/README.md](docs/adr/README.md) |
| Dependencies and the behaviour you must know | [docs/DEPENDENCIES.md](docs/DEPENDENCIES.md) |
| Milestones, status, work in progress | [docs/PROGRESS.md](docs/PROGRESS.md) |
| Known technical debt; feedback already turned into checks | [docs/TECH_DEBT.md](docs/TECH_DEBT.md) |
| Something fails silently or confusingly | [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) |
| Plan format; a plan edit went wrong | [docs/PLAN-FORMAT.md](docs/PLAN-FORMAT.md) · [docs/PLAN_CHANGE_RUNBOOK.md](docs/PLAN_CHANGE_RUNBOOK.md) |
| Local setup, Tailscale, phone | [docs/RUN_LOCALLY.md](docs/RUN_LOCALLY.md) · [docs/TAILSCALE.md](docs/TAILSCALE.md) |

`plan/` holds the milestone specs the app was built from. They are history, not current docs.

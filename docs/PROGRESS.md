# Progress

Milestone status and work in progress. Update this file in the same pull request that changes a
status, so the next session starts from the truth.

_Last updated: 2026-09-16._

## Planning documents: which one wins

| Document | Role |
|---|---|
| `dashboard-plan-v2.md` | The original *why*. Cited throughout, not in the repository ([TECH_DEBT.md](TECH_DEBT.md) TD-1). |
| [`dashboard-plan-v3.md`](../dashboard-plan-v3.md) | Milestone index and global decisions. Where v2 and v3 disagree, v3 wins. |
| `plan/M*.md` | Implementable spec per milestone: how the app was built. Historical once shipped. |
| `docs/` | The app's current documentation. |
| [`master-plan-v5.md`](../master-plan-v5.md) | The content the app tracks, currently Master Plan v6. Parser input. |

## Milestones

| # | Scope | Status |
|---|---|---|
| M0 | Scaffold: repo tree, Vite + Tailwind, health endpoint, tooling, CI | Shipped |
| M1 | ARCHITECTURE, DATABASE and API docs; ADRs | Shipped (ADR-001 to ADR-010) |
| M2 | SQLite store, migrations, seed | Shipped |
| M3 | Endpoints, validation, middleware, plan week and streak | Shipped |
| M4 | App shell, Today screen, quick log | Shipped |
| M5 | Goals Kanban with drag and tap-to-move | Shipped; follow-up F3 (real drag re-test) open in `plan/M5-followups.md` |
| M6 / M6+ | Log feed, Review, metrics, checkpoints; Review field guide | Shipped |
| M7 | Ship: embedded frontend, PWA manifest, backups, launchd, README | Shipped |
| M8 | Skills: every task builds a skill, Skills tab | Shipped |
| M9 | Gamification: XP ledger, levels, tiers, achievements | Shipped |
| M10 | Any plan: vocabulary as data, configurable grammar, `-plan` | Shipped |
| M11 | Load a plan from the UI: preview, diff, apply or replace | Shipped |
| M12 | Pair a phone with a one-time QR code | Shipped |
| M13 | Host on Fly.io as a tailnet node (tsnet, Litestream, read-only mirror) | Not started; spec in `plan/M13-host-on-fly.md` |

If M13 starts: ADR-003 still holds (tailnet only, no public port), ADR-001 holds with off-volume
replication added, and the container is the binary on `FROM scratch`.

## In progress (uncommitted on 2026-09-16)

### Master Plan v6 content

The robotics on-ramp is front-loaded as W1–W4, every later week shifts by +4, Phase 1 is now 16
weeks (Aug 24 → Dec 13), and the full checkpoint moves from W12 to W16.

- `master-plan-v5.md` rewritten with v6 content; `internal/seed/seed.json` regenerated.
- Achievement seed keys retargeted in `internal/api/game.go`; descriptions updated by the new
  `migrations/008_v6_achievement_text.sql` (codes unchanged).
- Plan-shaped test constants updated in the parser, seed, API and game tests.
- New runbooks: `docs/PLAN_CHANGE_RUNBOOK.md`, `docs/TAILSCALE.md`.
- Because week codes shifted, an existing database should be reset rather than reseeded
  additively, or moved tasks will be duplicated.

### Harness engineering

- `AGENTS.md` documentation map; knowledge moved into `docs/` (this file, CODEMAP, PRODUCT,
  TECH_DEBT, TROUBLESHOOTING, DEPENDENCIES, `docs/agents/`).
- Automated checks: depguard layering rules, API and schema contract tests, `cmd/doccheck`,
  applied-migration check, ESLint rule for HTTP calls, weekly scheduled CI.
- 11 GET handlers now reject unknown query parameters, as `docs/API.md` already promised.
- `make setup`, `make doctor`, `make dev-instance`; Claude Code hooks, skills and reviewer agent.

## Next

1. Commit the v6 plan work and the harness work as separate pull requests.
2. Pilot the playbook on one real task (see [agents/PLAYBOOK.md](agents/PLAYBOOK.md), "Pilot").
3. Close M5 F3 with a manual drag pass, or replace it with a browser test (TD-9).

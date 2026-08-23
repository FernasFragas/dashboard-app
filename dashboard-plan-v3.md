# Productivity Dashboard — Refined Build Plan v3 (index)

Refinement of `dashboard-plan-v2.md`: each milestone expanded into agent-implementable detail
in its own file under `plan/`. v2 stays the *why*; v3 is the *what exactly*. Where the two
disagree, **v3 wins**.

`plan/` = how we build it (this refinement). `docs/` = the app's own architecture docs, which
are M1's deliverable. Don't mix them.

## Milestones

| # | File | Scope | Status |
|---|---|---|---|
| M0 | [plan/M0-scaffold.md](plan/M0-scaffold.md) | repo tree, Vite+Tailwind, health endpoint, tooling, CI | refined |
| M1 | [plan/M1-architecture-docs.md](plan/M1-architecture-docs.md) | ARCHITECTURE · DATABASE · API · 6 ADRs | refined |
| M2 | [plan/M2-store.md](plan/M2-store.md) | SQLite open, migrations, `001_init.sql`, store CRUD, seed | refined |
| M3 | [plan/M3-api.md](plan/M3-api.md) | all endpoints, validation, middleware, plan-week + streak | refined |
| M4 | [plan/M4-today.md](plan/M4-today.md) | app shell, Today screen, quick-log + bar | refined |
| M5 | [plan/M5-kanban.md](plan/M5-kanban.md) | Goals Kanban, drag + tap-move | refined |
| M6 | [plan/M6-log-review.md](plan/M6-log-review.md) | Log feed, Review form, metrics, checkpoints | refined |
| M6+ | [plan/M6-review-fieldguide.md](plan/M6-review-fieldguide.md) | Review field guide: per-field meaning, helper copy, examples, metric help | refined |
| M7 | [plan/M7-ship.md](plan/M7-ship.md) | go:embed, PWA, backups, launchd, README | refined |
| M8 | [plan/M8-skills.md](plan/M8-skills.md) | Skills entity, task↔skill association (≥1 rule), seeded descriptions + mapping, Skills tab | refined |
| M9 | [plan/M9-gamification.md](plan/M9-gamification.md) | XP ledger, career-ladder levels, skill tiers, achievements, game feel | refined |
| M10 | [plan/M10-any-plan.md](plan/M10-any-plan.md) | decouple from master-plan-v5: data-driven calendar, vocabulary tables, configurable grammar, plan bundles | refined |
| M11 | [plan/M11-load-plan-from-ui.md](plan/M11-load-plan-from-ui.md) | upload a plan .md from the browser: preview, diff, apply — history preserved | refined |
| M12 | [plan/M12-pair-phone-by-qr.md](plan/M12-pair-phone-by-qr.md) | scan a QR on the desktop to open the app on the phone, authenticated, on the same screen | refined |
| M13 | [plan/M13-host-on-fly.md](plan/M13-host-on-fly.md) | run on Fly as a tailnet node via tsnet, Litestream durability, read-only mirror | refined |
| — | [plan/M5-followups.md](plan/M5-followups.md) | version-churn bug, redundant refetch, unverified drag, doc drift | F3 open |

## Global decisions (apply to every milestone)

| Decision | Value |
|---|---|
| Repo root | `/Users/fernando/personal-projects/dashboard-app` — app tree at root, no `dashboard/` nesting |
| Go module | `github.com/FernasFragas/dashboard-app` (matches the git remote) |
| Go version | 1.26.x |
| Frontend pkg mgr | pnpm (lockfile `pnpm-lock.yaml` committed) |
| Node version | 24.x, pinned in `.nvmrc` |
| Tailwind | v4 — CSS-first (`@import "tailwindcss"` + `@theme`), `@tailwindcss/vite` plugin, **no** `tailwind.config.js` |
| Default data dir | `~/dashboard-data` (`-data` flag default), DB at `~/dashboard-data/dashboard.db` |
| HTTP router | stdlib `net/http` ServeMux (ADR-005) |
| Logging | `log/slog` — text handler in dev, JSON in prod, `-log-format` flag (ADR-006) |
| Primary keys | `INTEGER PRIMARY KEY`; human codes (`G0`…`G17`) are labels in a `code` column, not identity |
| Timestamps | `TEXT`, RFC3339 **UTC**; dates as `YYYY-MM-DD` |
| Day boundary | `Europe/Lisbon` (`-tz` flag) — governs streaks, Log-feed day grouping, "today" |

## Deltas from v2

**Scope**
- Tooling tier is "full": lint + Vitest + GitHub Actions CI.
- The seed parser (`cmd/seedgen`, a v2 *stretch* item) is pulled into v1.
- ETag / `If-Match` optimistic concurrency on `goals` and `tasks` is pulled into v1.
- The W12/B7 checkpoint form is built in v1 rather than deferred.
- Review ships self-explanatory: every field carries a one-line helper + realistic example, ⓘ full
  guidance per section, metric help served from `metric_defs`
  ([plan/M6-review-fieldguide.md](plan/M6-review-fieldguide.md)).
- **Post-ship expansion — skills + gamification** ([plan/M8-skills.md](plan/M8-skills.md),
  [plan/M9-gamification.md](plan/M9-gamification.md)): every task builds ≥1 named skill (17 seeded
  with descriptions), then an XP ledger, career-ladder level titles, skill tiers, and achievements
  on top. **M8–M9 sit outside the v1 scope and are never started before M7 ships** — the tracker
  must exist before it becomes a game.
- **Guardrail #1's timebox was deliberately raised** to hold the scope above. The figure itself
  lives in `master-plan-v5.md`, which is where guardrails belong — this plan records the decision,
  not the number. Consequence: G0's "Aug 31" target may not hold — either G0 moves to W2, or M7
  ships late and W1 is tracked in dev mode. Decide against real elapsed time, not against a guess
  written here.

**Schema** (all detailed in [plan/M2-store.md](plan/M2-store.md), all must reach `DATABASE.md` first)
- `goals`: + `code TEXT UNIQUE NULL`, + `version INTEGER`.
- `tasks`: + `steps TEXT` (JSON), + `done_means TEXT`, + `seed_key TEXT UNIQUE NULL`, + `version INTEGER`.
- New table `checkpoints`; new seeded table `metric_defs`.
- Seeding is **additive upsert** by `code` / `seed_key`, not v2's "only when goals is empty".

**Corrections**
- v2 acceptance check #7 says "5 ADRs" → **6** (ADR-006 logging added).
- v2's repo tree shows a `dashboard/` root folder → dropped; the app tree lives at repo root.
- v2 says "run.sh or systemd unit" → **launchd**; this is macOS.
- v2 says the backup ticker "copies the db" → **`VACUUM INTO`**; a plain copy of a WAL database
  can be corrupt.
- `PATCH /api/goals/{id}` takes a 0-based `position`, not a raw `sort_order`; the server
  renumbers the affected columns and returns every card whose `sort_order` changed, bumping
  only the moved card's `version`.

## How to use this with a coding agent

One milestone per session, in order. Hand the agent `plan/MN-*.md` plus `dashboard-plan-v2.md`
for context — the milestone file is the contract, v2 is the rationale. M2 and M3 are built from
`docs/` (M1's output), not from these files.

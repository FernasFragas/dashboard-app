# Code map

Where each concern lives and how the binary boots. For *why* the pieces are shaped this way, read
[ARCHITECTURE.md](ARCHITECTURE.md); for the schema, [DATABASE.md](DATABASE.md) (§2 has the ERD).

## Runtime layout

```
iPhone (PWA) / laptop ──Tailscale──▶ bin/dashboard :8484   (one static Go binary)
                                       ├─ /api/*, /pair/*  → internal/api
                                       └─ everything else  → internal/web (embedded SPA, index.html fallback)
                                                 │
                                  ~/dashboard-data/dashboard.db   SQLite, WAL
                                  ~/dashboard-data/backups/       dashboard-YYYYMMDD.db (14 kept), pre-plan-*.db
                                  ~/dashboard-data/logs/          launchd stdout / stderr
```

In development, `make dev` runs Vite on :5173, proxying `/api` to Go on :8484, against the real
data directory. `make dev-instance` runs the same pair on free ports against a throwaway directory,
with logs written to disk. A Go build without the `embed_frontend` tag answers non-API paths with
a 404 that says so.

## Boot sequence (`cmd/server/main.go`)

1. `dashboard plan validate …` short-circuits everything else.
2. Flags: `-addr :8484`, `-data ~/dashboard-data`, `-token`, `-log-format text|json`,
   `-tz Europe/Lisbon`, `-public-url`, `-plan`, `-plan-reset`. Build the slog logger.
3. Create the data directory. `-plan-reset` deletes `dashboard.db` plus `-wal`/`-shm`, and only
   when `dashboard.db.backup` exists.
4. Load the timezone (exit 2 if invalid) and resolve the public URL used in pairing QR codes.
5. Load the seed document: the embedded `internal/seed/seed.json`, or `plan.ParseFile(-plan)`.
6. `store.Open` (PRAGMAs set via the DSN, then verified) → `Migrate(migrations.FS)` → `seed.Apply`
   (refuses a different plan id) → log the schema version.
7. Sweep expired pairing codes.
8. Top-level mux: `/api`, `/api/` and `/pair/` → `api.NewMux`; `/` → `web.Handler()`.
9. `backup.Start`: `VACUUM INTO` at 03:00 local time, prune to 14.
10. Serve. SIGINT or SIGTERM → 5 s graceful shutdown.

Any failure in steps 2–7 exits non-zero. There is no degraded mode.

## Backend

| Concern | File(s) |
|---|---|
| Flags, wiring, boot, `plan validate`, `-plan-reset` | `cmd/server/main.go` |
| Markdown plan → `seed.json` generator | `cmd/seedgen/main.go` |
| Routes, middleware, JSON and validation helpers, store-error mapping | `internal/api/server.go` |
| Dashboard, goals, tasks, logs, reviews, metrics, checkpoints, export handlers | `internal/api/handlers.go` |
| Skills endpoints, skill-id validation | `internal/api/skills.go` |
| Plan status, preview, apply; diff fingerprints; `planMode` | `internal/api/plan.go` |
| Phone pairing, QR SVG, rate limiter, bootstrap page | `internal/api/pair.go` |
| Game write envelopes, achievement conditions | `internal/api/game.go` |
| Pure time derivations (`PlanWeekFor`, streak) | `internal/api/time.go` |
| ETag and If-Match | `internal/api/etag.go` |
| API contract tests (routes ↔ `docs/API.md`, query validation) | `internal/api/contract_test.go` |
| Open, PRAGMAs, `tx` helper, clock, pre-plan snapshot | `internal/store/store.go` |
| Driver error → sentinel (`classify`) | `internal/store/errors.go` |
| Migration runner | `internal/store/migrate.go` |
| Row and response types | `internal/store/models.go` |
| Per-table CRUD | `internal/store/{goals,tasks,logs,reviews,metrics,checkpoints,skills,reference,pairing,plan}.go` |
| XP ledger, levels, skill tiers, achievement facts | `internal/store/game.go` |
| Full JSON export | `internal/store/export.go` |
| Schema contract tests (tables ↔ `docs/DATABASE.md`, export coverage) | `internal/store/schema_contract_test.go` |
| Seed document types | `internal/seed/document.go` |
| Seed `Load`, `Apply` (additive), `Replace` | `internal/seed/load.go` |
| Plan parser | `internal/plan/parse.go` |
| Parser profile, `plan.yaml`, front matter, `ParseFile` | `internal/plan/config.go` |
| Nightly backup and prune | `internal/backup/backup.go` |
| SPA handler; development vs embedded build | `internal/web/handler.go`, `internal/web/dist_dev.go`, `internal/web/dist_prod.go` |
| Schema history | `migrations/` (`001_init.sql` onwards) |

## Frontend

| Concern | File(s) |
|---|---|
| Query client, app shell, navigation, routes | `web/src/App.tsx` |
| API client and response types | `web/src/api/client.ts` |
| Today: week banner, rhythm slot, week tasks, quick log, counters, streak, game profile | `web/src/pages/TodayPage.tsx` |
| Goals Kanban (dnd-kit on desktop, tap-to-move on phones) | `web/src/pages/GoalsPage.tsx`, `web/src/pages/goalBoard.ts` |
| Log feed, category filters, weekly recap, delete with undo | `web/src/pages/LogPage.tsx` |
| Daily review, metrics, checkpoints, field guide | `web/src/pages/ReviewPage.tsx`, `web/src/components/FieldGuide.tsx` |
| Skills profile and XP bars | `web/src/pages/SkillsPage.tsx`, `web/src/components/GameSkillBar.tsx` |
| Plan upload, preview, apply; Pair phone | `web/src/pages/PlanPage.tsx` |
| Level-up celebration | `web/src/components/LevelUpMoment.tsx` |
| User-facing strings | `web/src/copy/` |
| Shared query keys, Lisbon dates, reduced motion | `web/src/lib/` |
| PWA manifest and icons | `web/public/` |

## Tooling and operations

| Concern | File |
|---|---|
| Toolchain check and first-time setup (`make doctor`, `make setup`) | `scripts/doctor.sh` |
| Isolated instance (`make dev-instance`) | `scripts/dev-instance.sh` |
| Documentation link and path check (`make check-docs`) | `cmd/doccheck/main.go` |
| Applied-migration check (`make check-migrations`) | `scripts/check-migrations-immutable.sh` |
| Lint and format config, including layering rules | `.golangci.yml`, `web/eslint.config.js`, `web/.prettierrc` |
| CI | `.github/workflows/ci.yml` |
| Pull request checklist | `.github/pull_request_template.md` |
| launchd agent template | `deploy/launchd/com.fernando.dashboard.plist` |
| Claude Code harness: hooks, path rules, skills, reviewer agent | `.claude/` |

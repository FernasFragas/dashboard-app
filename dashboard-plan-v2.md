# Productivity Dashboard — Build Plan v2

**Decided:** Go API + React/TypeScript front · SQLite · single binary · runs on your machine, reached over Tailscale from laptop + phone.
**Timebox:** ≤14h total (guardrail #1). Build window **Aug 22–31** so tracking is live for W1. No code in this doc — plan only.
**Changed from v1:** new **M1 · Architecture & technical docs** task (database decision, database design, API contract), milestones renumbered M0–M7, `project` field added to goals/tasks (from master-plan-v5 tags), acceptance check #7. Router deliberately undecided → ADR-005.

---

## 1. General plan

### What it is

A single-user accountability app seeded from `master-plan-v5.md`. Four screens: **Today** (this week's tasks + one-tap accomplishment logging), **Goals** (simplified Kanban), **Log** (what you actually did, not what's pending), **Review** (Sunday ritual + metrics). Everything you tap is one SQLite write away from permanent.

### Architecture

```
┌─ phone / laptop (Tailscale) ──────────────┐
│  http://<host>:8484  (PWA, installable)   │
└───────────────┬───────────────────────────┘
                │ JSON over HTTP
┌───────────────▼───────────────────────────┐
│  one Go binary                            │
│  ├─ HTTP router     /api/*                │
│  ├─ go:embed        web/dist (React)      │
│  ├─ store           SQLite (modernc, WAL) │
│  ├─ seed            embedded seed.json    │
│  └─ backup ticker   nightly db copy       │
└───────────────┬───────────────────────────┘
        ~/dashboard-data/dashboard.db
```

- **Backend:** Go, HTTP router **undecided → ADR-005** (stdlib ServeMux · chi · Echo/Gin, picked in M1), `modernc.org/sqlite` (pure Go, no CGO → trivial cross-compile to a home server later).
- **Frontend:** Vite + React 18 + TypeScript, Tailwind, TanStack Query (server state), dnd-kit (Kanban drag), wouter (routing). Built once, embedded via `go:embed` — no Node at runtime.
- **Network:** bind to the Tailscale IP (`-addr 100.x.y.z:8484`); tailnet ACL is the auth. Optional `-token` flag → `X-Token` header check as a second lock. No accounts.

### Screens (final design)

Dark theme only. Bottom tab bar on phone, left rail on desktop. Big tap targets; quick-log reachable with a thumb. Desktop shortcut `q` = quick log.

**1 · Today (default)**
- Banner: current plan week + focus ("W5 · Chaos: fail-open / fail-static") — computed from the date (W1 starts Aug 24).
- Today's rhythm slot from the operating-system table ("Thu — SAA prep + 30 min community").
- This week's tasks as a checklist (from seed + anything you add). Checking one = instant save + strikethrough.
- Project chips (`synapse · gateway · dash · oss · learn · write · career`) filter the checklist to the repo you have open.
- **The + bar:** 8 category buttons (Application 📮 · Module 📚 · Portfolio 🔧 · Post ✍️ · OSS 🔀 · Number 📊 · Network 🤝 · Exam 🎓). Tap → bottom sheet: title (required), note/URL (optional), link to goal (optional) → Save. Target: log an application in <5 taps.
- Weekly counters ("This week: 3 applications · 5 modules · 1 number") + streak chip (consecutive days with ≥1 log or review).

**2 · Goals (Kanban)**
- Three columns: **Backlog · Active · Done**. Cards = G0–G17 from the seed: ID, title, project tag, "done means", target (W8/B1/Q2), and a chip counting linked log entries (proof of motion).
- Drag between columns on desktop (dnd-kit); on phone, tap card → "Move to…" menu (drag on mobile web is flaky — don't fight it).
- Accountability rule in UI: Active > 3 shows a warning badge. Moving to Done stamps `completed_at`.

**3 · Log (accomplishments)**
- Reverse-chron feed grouped by day — the anti-checkbox view: only things that happened.
- Filter chips per category. Weekly recap card at top ("Week of Sep 21: 2 applications, 4 modules, 1 benchmark, 1 OSS comment").
- Entries deletable (typos), never editable-in-bulk — the log is the record.

**4 · Review**
- Daily 3-bullet form (learned / issue / next) — upsert keyed by date.
- Week completion % (done tasks / week tasks).
- Metric quick-add: pick a metric name from the targets table (p95, rejection rate, cache hit…), enter value → time-series kept.
- On checkpoint weeks (W12, B7): the checkpoint questions render as a form.

### Data model (SQLite)

```
goals          id PK · title · done_means · project TEXT · phase (P1|P2|P3)
               · status (backlog|active|done) · target TEXT ("W8","B1","Q2")
               · sort_order · created_at · completed_at
tasks          id PK · week TEXT ("W1".."W12","B1".."B7") · title · project TEXT
               · goal_id FK NULL · status (todo|done) · done_at
categories     id PK (slug) · label · icon · sort_order        -- seeded, extensible
log_entries    id PK · category_id FK · title · note NULL · url NULL
               · goal_id FK NULL · occurred_at · created_at
daily_reviews  date PK (YYYY-MM-DD) · learned · issue · next · minutes NULL
metrics        id PK · name · value REAL · unit · note NULL · recorded_at
```

`project` values via CHECK: `synapse|gateway|dash|oss|learn|write|career|all` — straight from master-plan-v5 tags.
Indexes: `log_entries(occurred_at)`, `log_entries(category_id)`, `tasks(week)`, `goals(status, sort_order)`.
Full column types, constraints, FK delete rules, and the ERD are specified in M1's `docs/DATABASE.md` — that document is the source `001_init.sql` is written from.

### Saving & persistence (how progress is stored)

- **Every user action = one HTTP call = one SQLite transaction.** No local-only state that can be lost. WAL mode + `busy_timeout=5000`; single user, single writer — no contention story needed.
- **Frontend:** TanStack Query mutations; task-check and Kanban-move are optimistic (instant UI, rollback on error). Everything else waits for the 200.
- **Migrations:** numbered SQL files embedded in the binary (`001_init.sql`, …), applied at boot, tracked in a `schema_migrations` table. Same discipline as Project 3, zero deps.
- **Seed:** `seed.json` embedded in the binary — goals G0–G17 with project tags, W1–W12 task lists with project tags, 8 categories, rhythm table, metric names — generated from master-plan-v5. Loader runs only when `goals` is empty (idempotent). Re-seeding never touches user data.
- **Backups:** in-app ticker copies the db nightly to `backups/dashboard-YYYYMMDD.db`, keeps 14. `GET /api/export` returns a full JSON dump on demand (restore = future concern, the JSON is the insurance).
- **Restart-safe:** state lives only in the db file; the binary is stateless.

### API surface

| Method + path | Purpose |
|---|---|
| GET /api/health | liveness |
| GET /api/dashboard | today bundle: plan week, day slot, week tasks, counters, streak |
| GET · POST /api/goals | list / create |
| PATCH · DELETE /api/goals/{id} | move column (status + sort_order), edit, done |
| GET /api/tasks?week=W5&project= · POST /api/tasks | week list (filterable) / add ad-hoc task |
| PATCH /api/tasks/{id} | toggle done |
| GET /api/logs?category=&from=&to= · POST /api/logs | feed / **the + button** |
| DELETE /api/logs/{id} | remove typo entry |
| GET /api/logs/summary?range=week\|all | counters per category |
| GET · POST /api/reviews | daily 3-bullet (POST = upsert by date) |
| GET · POST /api/metrics | time-series numbers |
| GET /api/export | full JSON dump |

Errors: `{"error": "msg"}` + proper status. Validation in handlers (title required, enum checks). If `-token` set: middleware rejects missing/wrong `X-Token`.

### Repo layout

```
dashboard/
├─ cmd/server/main.go          flags: -addr -data -token
├─ docs/                       ARCHITECTURE.md · DATABASE.md · API.md · adr/
├─ internal/store/             sqlite open, migrations, typed CRUD + tests
├─ internal/api/               HTTP handlers, middleware
├─ internal/seed/              seed.json + loader
├─ internal/log/               for logging implementation
├─ web/                        Vite app (src/screens, src/components, src/api)
├─ migrations/                 001_init.sql …
├─ Makefile                    dev · build · embed
└─ README.md                   run + Tailscale + phone-install steps
```

---

## 2. Implementation tasks

Each task: scope → **done when**. Sequence is the dependency order.

**M0 · Scaffold**
Repo tree above; Vite React TS app with Tailwind; bare `net/http` server with /api/health (one route needs no router — the real pick is ADR-005 in M1); Makefile: `make dev` = Go on :8484 + Vite on :5173 proxying `/api`.
**Done when:** `make dev` → browser shows React shell rendering the health response.

**M1 · Architecture & technical docs**
Write the decisions down before the schema exists, in `docs/`:
1. `ARCHITECTURE.md` — the component diagram from §1, each component's responsibility, the request flow (tap → HTTP → SQLite tx → query invalidation), and the stack list with a one-line rationale per choice.
2. ADRs, one page each in `docs/adr/`:
   - **ADR-001 · Database = SQLite** — considered: SQLite / Postgres / flat JSON. Chosen: SQLite (single user, zero ops, WAL is enough, backup = file copy). Consequence: revisit only if the app ever becomes multi-user.
   - **ADR-002 · Single binary** — go:embed'd frontend vs separate deploys. Consequence: one artifact, no Node at runtime, frontend changes require rebuild.
   - **ADR-003 · Auth = network perimeter** — Tailscale ACL + optional `X-Token`, no accounts. Consequence: never expose the port publicly.
   - **ADR-004 · Layering = pragmatic two-layer, not hexagonal** — considered: full ports-and-adapters (incl. store decorators for logging). Rejected: no domain to isolate, no second adapter planned, and a 6-table store means an ~20-method interface — decorators become ~200 lines of hand-written ceremony (Go has no AOP). Rule: handlers = transport only, store = persistence only, plan-week/streak = pure functions; the only decorators are the HTTP middleware chain (recover, request log, token). **Zero log calls below the API layer:** store and pure functions return wrapped errors (`%w`) and never log; one respond-error helper logs each failure exactly once at the boundary. Store tests run against a real temp SQLite file, not mocks. Consequence: if per-operation telemetry or a second backend ever appears, extract a store interface then and wrap it — mechanical retrofit, nothing to redesign. (Hexagonal + provider decorators belong in LLMGateway-Go, where the port is narrow and real.)
   - **ADR-005 · HTTP router — undecided, decide here.** Candidates: stdlib `net/http` ServeMux (Go 1.22+ handles `GET /api/tasks/{id}` + `r.PathValue`, zero deps) · chi (stdlib-compatible, adds route grouping + middleware chaining) · Echo/Gin (own context type — reshapes every handler and middleware signature). The real axis: stay net/http-shaped or adopt a framework context. 12 endpoints, one user — pick in 10 minutes and record the why.
3. `DATABASE.md` — the database design: mermaid ERD; per table every column with type, nullability, default, CHECK enums; FK delete rules (goal deleted → `goal_id` SET NULL on tasks and logs — history survives); each index paired with the exact query it serves; migration policy (numbered, forward-only, applied at boot); seed policy (only when `goals` is empty).
4. `API.md` — the endpoint table from §1 plus one request/response JSON example per endpoint, the error envelope, and status codes.
**Done when:** M2 and M3 could be built from `docs/` alone without reading this plan, and `001_init.sql` is later written 1:1 from `DATABASE.md`.

**M2 · Store + migrations + seed**
Open SQLite (WAL, busy_timeout); migration runner over embedded SQL; `001_init.sql` transcribed from `DATABASE.md` (six tables + indexes); store package with typed methods per table; seed.json + idempotent loader.
**Done when:** `go test ./internal/store` green; deleting the db and rebooting recreates it with G0–G17 (with project tags), 12 weeks of tasks, 8 categories.

**M3 · API **
All endpoints in `API.md`; validation; error envelope; token middleware; request log line per call. Dashboard endpoint computes plan-week from date (W1 = Aug 24) and streak from logs+reviews.
**Done when:** a curl checklist in the README (create goal → move it → add task → toggle → log entry → summary) passes end to end.

**M4 · Today screen + quick log**
App shell with bottom nav; dashboard query; task checklist with optimistic toggle + project chips; + bar with 8 buttons → bottom-sheet form → POST /api/logs; counters + streak chip.
**Done when:** on the phone over Tailscale, logging a job application takes <5 taps and survives reload.

**M5 · Kanban**
Columns from /api/goals grouped by status; dnd-kit drag → PATCH (status, sort_order); phone fallback: tap → "Move to…" sheet; Active>3 warning badge; Done stamps completed_at.
**Done when:** drag a card, hard-refresh, it stayed; same flow works via tap-move on the phone.

**M6 · Log + Review**
Feed grouped by day with filter chips + weekly recap card; review form (upsert by date); metric quick-add with name picker; week-% bar.
**Done when:** the full Sunday ritual (3 bullets + one metric + glance at recap) works on the phone.

**M7 · Ship**
`go:embed web/dist` behind a build tag; `make build` → single binary (CGO_ENABLED=0); PWA manifest + icons (installable, no offline worker); nightly backup ticker + keep-14 pruning; run.sh or systemd unit; README: Tailscale bind, `-token`, add-to-home-screen steps.
**Done when:** one binary on the host machine; phone opens it via tailnet name, installs to home screen; next morning a backup file exists.

## 3. Stretch (explicitly not v1)

Charts (recharts weekly trends) · calendar heatmap · offline service worker · reminders/notifications · master-plan.md parser for re-import · CSV export · light theme · OpenAPI spec generated from API.md.

## 4. Out of scope, permanently

Accounts/multi-user · public deployment · native mobile app · websockets/live sync · editing history.

## 5. Risks & answers

- **Drag on mobile web is unreliable** → tap-to-move menu is the primary mobile interaction, drag is the desktop nicety.
- **CGO/sqlite pain** → modernc.org/sqlite, pure Go.
- **Scope creep** (the real one) → stretch list is quarantine; v1 acceptance below is the contract.
- **Vite/embed path mismatch** → Makefile copies dist into the embedded dir; build fails loudly if missing.
- **Docs drift from code** → `DATABASE.md` is the source of `001_init.sql`; schema changes start in the doc, then become a migration.

## 6. v1 acceptance (all on the phone, over Tailscale)

1. Check off a W1 task → persists across restart.
2. Log an application via + in <5 taps → appears in Log feed + weekly counter.
3. Move G1 to Active on the Kanban → still there after reboot.
4. Complete a Sunday review + record one metric.
5. Kill the process mid-use → zero data loss on restart.
6. `backups/` contains yesterday's db copy.
7. `docs/` contains ARCHITECTURE.md, DATABASE.md, API.md, 5 ADRs — and `DATABASE.md` matches `001_init.sql` exactly.

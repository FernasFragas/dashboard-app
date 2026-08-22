# M2 · Store + migrations + seed

**Depends on:** M1 (`docs/DATABASE.md` is the source of truth). **Unblocks:** M3.
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §2

Everything here is transcribed from `docs/DATABASE.md`. If this file and `DATABASE.md`
disagree, fix `DATABASE.md` first, then re-transcribe — the doc is upstream of the SQL.

## Cross-cutting conventions

| Convention | Rule |
|---|---|
| Primary keys | `INTEGER PRIMARY KEY` everywhere. Human codes are **labels, not identity**. |
| Goal codes | `goals.code TEXT UNIQUE` holds `G0`…`G17`. Hand-created goals have `code = NULL`. |
| Timestamps | `TEXT`, RFC3339 **UTC** (`2026-08-22T14:03:00Z`). Sorts as string, reads in any SQLite browser, marshals straight from Go's `time.Time`. |
| Dates | `TEXT`, `YYYY-MM-DD` (`daily_reviews.date`). |
| Day boundary | **`Europe/Lisbon`**, overridable with `-tz`. Stored UTC → converted to local for streaks, the Log feed's day grouping, and "today". A 00:30 local log must count as today, not yesterday. |
| Driver | `modernc.org/sqlite` (pure Go), `WAL`, `busy_timeout=5000`, `foreign_keys=ON`. |

## Schema deltas from v2 §1

1. `goals`: `id INTEGER PK` **+ new `code TEXT UNIQUE NULL`**.
2. `tasks`: **new `steps TEXT`** (JSON array of strings, the numbered substeps) and
   **new `done_means TEXT NULL`** (the `→ **Done =**` line). Without these the app loses the
   part of the master plan that says what to actually do.
3. `tasks`: **new `seed_key TEXT UNIQUE NULL`** — stable identity for seeded rows,
   `"<week>:<slug(title)>"`. Rows you create in the UI have `seed_key = NULL`.
   Required by the additive re-seed below.
4. `goals` and `tasks`: **new `version INTEGER NOT NULL DEFAULT 1`**, incremented on every
   update. Backs the `ETag` / `If-Match` optimistic concurrency chosen for M3. Store update
   methods take the expected version and return `store.ErrConflict` on mismatch.
5. **New tables `weeks` and `rhythm`** — the week windows (code, phase, start/end date, focus)
   and the operating-system day/slot rows. v2 put both in `seed.json` without saying where they
   land at runtime; the dashboard endpoint needs to query them. Full spec: `docs/DATABASE.md`
   §4.2, §4.3.
6. **New table `checkpoints`** — `id INTEGER PK · week TEXT UNIQUE ("W12","B7") ·
   questions TEXT (JSON array) · answers TEXT NULL (JSON array, index-aligned) ·
   completed_at TEXT NULL`. M6 renders the checkpoint form from this.
7. `metrics`: the seeded name list also carries `baseline` and `target` text from the master
   plan's Metrics targets table, so the Review screen can show "p95 latency (cached) — target
   <0.8s" next to the input. Stored in a seeded `metric_defs` table
   (`name TEXT PK · unit · baseline TEXT NULL · target TEXT NULL · sort_order`);
   `metrics` rows stay free-form so a new name never needs a migration.

Everything else (categories, log_entries, daily_reviews, metrics, the `project` CHECK enum,
the four indexes) is as specified in v2 §1 / `DATABASE.md`.

## Migrations

Numbered SQL files in `migrations/`, `go:embed`'d, applied at boot inside a transaction,
tracked in `schema_migrations(version INTEGER PK, applied_at TEXT)`. Forward-only, no down
migrations. `001_init.sql` is transcribed 1:1 from `DATABASE.md`. Running the runner twice is
a no-op — this is an explicit test.

## Seed generator — `cmd/seedgen`

> **Note:** v2 lists the parser as a *stretch* item; pulled into v1 deliberately.
> Mitigation: `seed.json` is **committed**, so a parser bug can never block a boot — the binary
> only ever reads the JSON, never the markdown.

`make seed-gen` runs `cmd/seedgen master-plan-v5.md > internal/seed/seed.json`. It parses:

| Source in master-plan-v5.md | Produces |
|---|---|
| `## Log categories` — the `Name 📮 · Name 📚 · …` line | 8 categories: slug `application, module, portfolio, post, oss, number, network, exam` + label + emoji + sort_order |
| `## Goals (Kanban seed)` table rows `\| G0 \| title \| project \| done means \| target \|` | goals — `code`, `title`, `project`, `done_means`, `target`; all seeded `status='backlog'` |
| `## Operating system` day/slot table | rhythm entries (day → slot text) |
| `## W1 · Aug 24–30 — Focus` / `## B1 · Nov 16–29` headings | week code, start/end dates, focus label |
| `- [ ] **[project] Title**` + numbered lines + `→ **Done =** …` | tasks — `week`, `project`, `title`, `steps[]`, `done_means` |
| `## Metrics targets` table | `metric_defs` — name, baseline, target, for the Review screen's picker |
| `## W12` task step "answer in writing: …" and the `## B7` "same 5-question format" line | `checkpoints` — W12 questions split on `?`; B7 seeded with the same five |

Parser is strict: any line under a week heading that matches none of the shapes is a **hard
error**, not a silent skip. A silent skip is how you lose a week of tasks without noticing.

## Seed loader — additive upsert

Runs at boot, after migrations:

- **Insert only.** Never `UPDATE`, never `DELETE`. A row that already exists is left exactly
  as it is, including its `status` / `done_at`.
- Match key: `goals.code`, `tasks.seed_key`, `categories.id`.
- Rows with `seed_key = NULL` (your hand-created tasks) are never touched by any of this.
- Net effect: add a task to the master plan → `make seed-gen` → restart → it appears; edit an
  existing task's **title** → it appears as a *new* task, because the title is part of its key.
  That is the accepted trade of additive-upsert over the delete-and-reseed rule in v2.

## Store package

One `store.Store` struct (`store.New(db) (*Store, error)`), methods split across
`goals.go · tasks.go · logs.go · reviews.go · metrics.go · categories.go`. Handlers take a
single `*store.Store`.

Per ADR-004: **zero log calls in this package.** Errors are wrapped with `%w` and returned;
the API layer logs them once at the boundary. Sentinel errors: `store.ErrNotFound`,
`store.ErrConflict`.

## Tests — full coverage incl. error paths

Real temp SQLite files, no mocks. Beyond CRUD per table, these must exist:

1. Migration runner applied twice → no-op, schema unchanged.
2. Seed loader run twice → row counts identical (idempotent).
3. Seed loader run after a task is marked done → that task stays `done` (additive-only proof).
4. Delete a goal → linked tasks and log entries survive with `goal_id = NULL`.
5. `project` CHECK rejects an unknown value; `status`/`phase` CHECKs reject unknown enums.
6. Unique violations on `goals.code` and `tasks.seed_key` → `store.ErrConflict`.
   Updating with a stale `version` → `store.ErrConflict`; a successful update bumps `version`.
7. Day-boundary: a log at `23:30Z` and one at `00:30Z` land on the correct **Lisbon** days.

## Done when

1. `go test ./internal/store` green, including all seven rules above.
2. Deleting `~/dashboard-data/dashboard.db` and restarting recreates it with G0–G17 (with
   project tags and codes), 12 weeks + 7 blocks of tasks with steps, and 8 categories.
3. `make seed-gen` regenerates `seed.json` from `master-plan-v5.md` with no diff when the
   master plan hasn't changed.

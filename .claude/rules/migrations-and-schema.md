---
paths:
  - "migrations/**"
  - "internal/store/**"
  - "docs/DATABASE.md"
---

# Schema and migration rules

## Order of work

1. Change `docs/DATABASE.md` first: tables §4, indexes §5, seed policy §8 if relevant. It is the
   schema source of truth; if it and the SQL disagree, the SQL is the bug.
2. Add `migrations/NNN_description.sql` with the next free number (`ls migrations/`).
3. Update `internal/store/models.go`, the store methods, and store tests.
4. If the table holds data, add it to `internal/store/export.go` — `GET /api/export` must dump
   every table — and to the export shape in `docs/API.md`.
5. If it is plan-owned reference data, update `internal/seed/document.go` and
   `internal/seed/load.go` (both the additive upsert and the replace path).

## Migration rules

- Forward-only. No down migrations; the recovery path is a backup file.
- **Never edit a migration that may already be applied**, not even to fix copy. Add a new one.
  `008_v6_achievement_text.sql` is the model: `UPDATE` the rows, leave frozen codes alone.
- Each file runs inside its own transaction and is recorded in `schema_migrations`. One logical
  change per file. A failing migration aborts boot.
- Filenames must be `NNN_description.sql`; a duplicate version fails boot.
- `PRAGMA foreign_keys` cannot be switched inside a transaction, and the runner always opens one.
  Plan table rebuilds with that in mind and end them with `PRAGMA foreign_key_check;`.
- Must run on the pure-Go `modernc.org/sqlite` driver: no loadable extensions, no CGO.

## Schema conventions (`docs/DATABASE.md` §1)

- PKs are `INTEGER PRIMARY KEY`, except genuinely stable natural keys (vocabulary ids,
  `metric_defs.name`, `daily_reviews.date`, `xp_rules.source`, …).
- Human codes (`G7`, `W5:slug`) are labels in `code` / `seed_key`, never identity.
- Timestamps: `TEXT`, RFC3339 UTC with `Z`. Dates: `TEXT` `YYYY-MM-DD`, Lisbon calendar.
- No booleans: `TEXT` states with a `CHECK`.
- JSON arrays in `TEXT` only for lists always read and written whole (`tasks.steps`, checkpoint
  `questions` / `helpers` / `answers`). Never query into them.
- No `CHECK (x IN (…))` for plan vocabulary; reference the vocabulary table with an FK.
- `version INTEGER NOT NULL DEFAULT 1` only on user-edited resources (`goals`, `tasks`).
- FK delete rules carry meaning: history survives (`SET NULL` on goal references), vocabulary under
  history is protected (`RESTRICT`), link rows die with either side (`CASCADE`).
- Non-blank text: `CHECK (length(trim(title)) > 0)`.
- **No stored derived values**: no counters, totals, levels, tiers or streaks (ADR-007, ADR-008).
- **No speculative indexes.** Every index names the query it serves in `docs/DATABASE.md` §5.
- `metrics.name` deliberately has no FK to `metric_defs`, so a new metric needs no migration.

## Store tests

Use `newTestStore(t)`: a real migrated temp database with fixtures inserted by raw SQL rather than
the real seed, so store tests don't break when the plan content changes.

## Enforced

- `internal/store/schema_contract_test.go`: every migrated table has a `### 4.x` section in
  `docs/DATABASE.md` and is exported, or is listed in `notExported` with the reason.
- `scripts/check-migrations-immutable.sh` (CI and `make check-migrations`): merged migrations are
  never modified or deleted.
- `.claude/hooks/guard-migrations.sh`: an Edit or Write on a committed migration is blocked.

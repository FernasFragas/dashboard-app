---
paths:
  - "master-plan-v5.md"
  - "internal/plan/**"
  - "internal/seed/**"
  - "cmd/seedgen/**"
  - "plan/examples/**"
  - "internal/api/game.go"
  - "internal/api/plan.go"
---

# Plan, parser and seed rules

`master-plan-v5.md` is **input to a strict parser and the source of database identity**, not a
document. The filename says v5 but the content is Master Plan v6; keep the filename, because the
`Makefile`, tests and docs point at it.

## Pipeline

```
master-plan-v5.md ──make seed-gen──▶ internal/seed/seed.json  (committed, embedded)
                                              │ every boot
-plan PLAN.md (dev/import) ──plan.ParseFile───┤
                                              ▼
                             migrations → seed.Apply (additive upsert) → serve

/plan in the browser → POST /api/plan/preview (no writes) → POST /api/plan/apply (additive | replace)
```

Stored uploaded sources are never parsed at boot (ADR-009).

## Order for any plan edit

```sh
go run ./cmd/server plan validate master-plan-v5.md   # 1. parse
make seed-gen                                          # 2. regenerate seed.json; read the counts line
                                                       # 3. choose a reseed mode (below)
go test ./...                                          # 4. coupled constants and tests
```

Skipping step 2 means the edit never reaches the app. A zero or a drop in the seed-gen counts means
a section silently stopped parsing.

## Identity

- Task identity: `seed_key = "<week>:" + slug(title)`. Retitling a task or renumbering a week mints
  a new key, so an additive reseed inserts a duplicate and keeps the old row.
- Goals match on `code` (`G\d+`). Goals and tasks are **insert-only** on reseed; user progress is
  never overwritten.
- Reference tables **upsert**: projects, phases, skill_tiers, weeks, rhythm, categories,
  metric_defs, skills, and checkpoint question/helper text.
- `metric_defs` upserts on `name`; renaming a metric while keeping its `slug` fails boot on
  `UNIQUE metric_defs.slug`.
- One `plan_meta.id` per database; loading a different id is refused (`seed.ErrPlanMismatch`).

## Reseed modes

| Change | Mode |
|---|---|
| Content inside existing weeks; week codes and task titles unchanged | **Additive**: restart |
| Renumbered weeks or renamed tasks, database disposable | **Reset**: `cp dashboard.db dashboard.db.backup`, start with `-plan-reset` |
| Same, but keep logs, reviews, metrics and XP history | **Replace**: bump the plan `id`, upload via `/plan` *before* `make seed-gen` + restart |

Never reset or replace against `~/dashboard-data` without explicit approval.

## Parser traps

- Section headings match **by prefix** (`sectionIs` uses `strings.HasPrefix`): `## Skills tracker`
  is parsed as the Skills table. Human tables need headings that don't start with a configured
  section word — the plan uses `## Skill progress`.
- Inside a week, an unrecognised line is an error on purpose. Don't "fix" it by moving content out
  of the week.
- Outside weeks, non-table lines are skipped silently: a prose `## Log categories` yields zero
  categories and no error.
- A `## Projects` table makes task tags strict. A `## Skills` table makes `→ **Skills =**`
  mandatory on every task.
- A task title containing "Checkpoint" requires `## Checkpoint questions`.
- Week heading shape: `## W1 · Aug 24–30 — Focus` (en dash in the date range). Task details are
  indented two spaces.
- Custom grammar comes from `plan.yaml` beside the file or from YAML front matter; front matter
  wins. Worked example: `plan/examples/side-plan/`.

## Code coupled to plan content

- `gameAchievementConditions` in `internal/api/game.go` hardcodes seed keys and a checkpoint week.
  A stale key doesn't error; the achievement just becomes unreachable. Retarget after renumbering.
- Achievement **codes** are frozen join keys (`boss_w12` now targets W16). Change descriptions
  with a new migration; never edit `006_gamification.sql`.
- Plan-shaped test constants: `internal/plan/parse_test.go`, `internal/seed/load_test.go`,
  `internal/api/api_test.go`, `internal/api/game_test.go`.
- List the real keys:
  `python3 -c "import json;print([t['seed_key'] for t in json.load(open('internal/seed/seed.json'))['tasks']])"`

Troubleshooting: `docs/PLAN_CHANGE_RUNBOOK.md`. Format reference: `docs/PLAN-FORMAT.md`.

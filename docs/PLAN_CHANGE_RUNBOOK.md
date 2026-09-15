# Plan Change Runbook

Troubleshooting companion to `docs/CHANGE_OR_ADD_PLAN.md`. That doc is the happy path. This one
is for when editing the plan breaks validation, breaks boot, or quietly produces a dashboard that
looks wrong.

The recurring cause is that `master-plan-v5.md` is not prose — it is the input format for a
strict parser and, downstream, the source of database identity. Editing it as a document is how
most of these failures start.

---

## The Order That Works

```sh
go run ./cmd/server plan validate master-plan-v5.md   # 1. parse
make seed-gen                                          # 2. regenerate internal/seed/seed.json
                                                       # 3. choose a reseed mode (section 3)
make dev                                               # 4. run
go test ./...                                          # 5. catch coupled code and stale tests
```

Why this order: validation costs nothing and the database costs a restore. Steps 1 and 2 catch
almost everything below before any row changes.

**Step 2 is not optional.** The app seeds from `internal/seed/seed.json`, not from the markdown.
Editing the plan and restarting without `make seed-gen` changes nothing at all — the single most
common "my edit didn't show up".

Its output line is the cheapest assertion you have:

```
seedgen: 23 weeks, 78 tasks, 22 goals, 8 categories, 12 metrics, 2 checkpoints, 20 skills
```

Read those seven numbers every time. A section you flattened into prose shows up here as a zero
or a drop, long before you notice it missing in the UI.

---

## 1. Validation Errors

All of these come from `go run ./cmd/server plan validate`, and all of them are fixed in the
markdown. Most carry a line number.

| Message | Cause | Fix |
|---|---|---|
| `metric "X" needs definition and how to measure` | Metrics table has fewer than 7 columns | Restore `Metric \| Baseline \| Target \| Slug \| Unit \| Definition \| How to measure` |
| `metric row needs metric, baseline and target` | Fewer than 3 columns | Same |
| `skill row needs code, name, description, associate when and target` | The `## Skills` table lost its columns, **or** a human-readable table sits under a heading starting with `Skills` | Restore the 5 columns; rename the prose table to something not starting with `Skills` |
| `task W:x has no skills` | A `## Skills` section exists, so every task needs `→ **Skills =** code` | Add the marker, indented two spaces |
| `task W:x references skills but no skills section exists` | The reverse: markers without a catalogue | Add the `## Skills` table, or strip the markers |
| `task W:x references undefined skill "c"` | Typo, or a code you never defined | Fix the code or add the row |
| `found N checkpoint tasks but no checkpoint questions section` | A task title contains the word "checkpoint" | Add `## Checkpoint questions` with `question \| helper` rows |
| `unknown project tag "x"` | A `## Projects` table exists, which makes tags strict | Add the id to the table. Deleting the table also works — ids then become their own labels |
| `unrecognised line in week W: "..."` | Inside a week, anything that is not a task bullet, an indented detail, or an italic aside | Indent it as a detail, or move it out of the week |
| `unrecognised task detail: "..."` | A continuation line that does not start with a space | Indent it two spaces |
| `task has no [project] tag` | Bullet does not open with `**[tag] ...**` | Add the tag |
| `duplicate seed_key "W:slug"` | Two tasks in one week whose titles slug identically | Make one title distinct |
| `week W overlaps week V` | Date ranges collide | Fix the dates. Gaps are allowed; overlaps are not |
| `duplicate week code "W"` | Two headings, same code | Renumber |
| `unsupported date range "..."` | Heading dates not in `Mon D–D` form | Match the existing headings, en dash included |
| `category row needs id, label and icon` | Log-categories table missing a column | Restore `id \| label \| icon` |

### The Two Traps Behind Most Of These

**Section headings are matched by prefix.** `sectionIs` (`internal/plan/parse.go:172`) uses
`strings.HasPrefix`, so `## Skills tracker` is claimed by the `Skills` parser and `## Metrics
targets for 2027` by the metrics parser. If you want a human-readable table near a machine-read
one, give it a heading that does not start with the configured word — `## Skill progress`, not
`## Skills tracker`.

**Weeks are strict on purpose.** Outside a week section, an unrecognised line is skipped. Inside
one it is an error. From `internal/plan/parse.go:15`: *silent skips are how a week of tasks
disappears unnoticed.* Do not "fix" a week error by moving the content out of the week.

---

## 2. Failures That Do Not Error

These parse cleanly and produce a wrong dashboard. Only the `seed-gen` counts catch them.

| Symptom | Cause |
|---|---|
| The `+` buttons are gone from the Log | `## Log categories` was rewritten as prose. Non-table lines are skipped silently → 0 categories |
| Projects show as `synapse`, `dash` rather than proper labels | The `## Projects` table was deleted. Ids are backfilled from tags as their own labels (`parse.go:588`) |
| A whole phase has no tasks | Its week headings became prose. `**B1 · Dec 14–27:**` is not a heading; `## B1 · Dec 14–27` is |
| A checkpoint never appears | Its week heading is gone, or no task title in that week contains "checkpoint" |

---

## 3. Choosing The Reseed Mode

This is the step people skip, and it is the one that corrupts data.

The seed loader is **additive-upsert** (`internal/seed/load.go`). The distinction that matters:

- **Reference tables upsert** — weeks, projects, categories, metric_defs, skills, checkpoints,
  rhythm. Edits to these reach an existing database on restart.
- **Goals and tasks are insert-only** — `ON CONFLICT DO NOTHING`. An existing task is *never*
  updated. Retitle one and you do not edit it; you get a second task.

A task's identity is `seed_key = week + ":" + slug(title)` (`parse.go:385`). So renaming a task
or renumbering a week mints a new key, and the old row stays.

| Your change | Mode | How |
|---|---|---|
| Content edits inside existing weeks; week codes and task titles unchanged | Additive | Just restart |
| Renumbered weeks, renamed tasks or metrics, new plan version — and the database is disposable | Reset | Below |
| Same, but you want to keep logs, reviews and metrics | Replace | Below |

### Reset

Wipes everything, reseeds clean. Stop the server first.

```sh
cp ~/dashboard-data/dashboard.db ~/dashboard-data/dashboard.db.backup
go run ./cmd/server -plan-reset
```

`-plan-reset` refuses to run unless that `.backup` exists (`cmd/server/main.go:250`), then removes
`dashboard.db` and its `-wal`/`-shm`. Confirm the log line: `seed applied weeks=… tasks=… goals=…`.

### Replace, Keeping History

`seed.Replace` drops plan-owned rows but preserves `log_entries`, `daily_reviews` and `metrics`,
and snapshots to `backups/pre-plan-<id>-<stamp>.db` first. It is reached **only** through the UI
loader at `/plan`, and only when the plan **id** differs — `planMode` (`internal/api/plan.go:407`)
compares ids, not content. So bump the id in front matter:

```yaml
---
plan:
  id: master-plan-v6
  name: Master Plan v6
---
```

**Order matters.** Upload through `/plan` *before* running `make seed-gen` and restarting. If the
embedded seed carries the new id while the database still holds the old one, boot fails outright
(see section 4).

---

## 4. Boot Errors

The server has no degraded mode by design (`docs/ARCHITECTURE.md` section 5): a seed failure exits
non-zero rather than serving a half-populated dashboard.

**`UNIQUE constraint failed: metric_defs.slug`**

You renamed a metric but kept its slug. The metrics upsert conflicts on `name`, which is the
primary key, so a renamed metric is an INSERT — and it lands on the old row's slug, which
`idx_metric_defs_slug` rejects. The plan is fine; the old database cannot accept it. Reset, or
revert the metric name to the spelling already in the database.

**`database already holds plan "X", refused to load "Y"`**

The plan id changed. `ensurePlan` refuses rather than mixing two plans. Use a different `-data`
directory, reset, or go through the `/plan` replace flow described above.

**Seeds fine, but there are now twice as many tasks**

The additive path ran against renumbered weeks. Every moved task was inserted alongside its
predecessor. There is no un-merge — reset and reseed.

---

## 5. After Renumbering Weeks

Week codes and seed keys are referenced from Go, from migrations, and from tests. `go test ./...`
catches all of it, which is why it is step 5 and not optional.

- **`internal/api/game.go`** — achievement conditions hardcode seed keys and a checkpoint week
  (`gatekeeper`, `chaos_suite`, `honest_number`, `in_the_arena`, `boss_w12`). A stale key does not
  error; the achievement simply becomes unreachable. Retarget every one.
- **Achievement descriptions** live in `migrations/006_gamification.sql` and name week numbers.
  Do not edit an applied migration — add a new one with `UPDATE achievements SET description`.
  Achievement *codes* stay frozen; they are the join key.
- **Renamed a task title?** That moves its seed key too, not just the week prefix. Check the
  actual key rather than assuming the prefix changed:
  ```sh
  python3 -c "import json;print([t['seed_key'] for t in json.load(open('internal/seed/seed.json'))['tasks']])"
  ```
- **Tests holding plan-shaped constants**: `internal/plan/parse_test.go` (`TestParseRealMasterPlan`),
  `internal/seed/load_test.go` (counts), `internal/api/api_test.go` (checkpoint week),
  `internal/api/game_test.go` (seed keys).

---

## 6. What Each Section Must Contain

| Heading | Columns | Notes |
|---|---|---|
| `## Projects` | `id \| label` | Optional. Present ⇒ tags become strict |
| `## Log categories` | `id \| label \| icon` | Must be a table. Prose is skipped silently |
| `## Operating system` | `Day \| Slot` | Weekday names and ranges only |
| `## Goals` | `ID \| Goal \| Project \| Done means \| Target` | ID must match `G\d+` |
| `## Metrics targets` | `Metric \| Baseline \| Target \| Slug \| Unit \| Definition \| How to measure` | All 7. One metric per row — no `p95 / p99` sharing a line |
| `## Skills` | `code \| name \| description \| associate when \| target` | Present ⇒ every task needs a skills marker |
| `## Checkpoint questions` | `question \| helper` | Required if any task title contains "checkpoint" |

Week headings: `## W1 · Aug 24–30 — Focus text`, code `[WB]\d+`, en dash in the date range.

Task bullets: `- [ ] **[tag] Title**`, with details indented two spaces — numbered steps,
`→ **Done =** …`, `→ **Skills =** code, code`.

---

## 7. Recovery

| What | Where |
|---|---|
| Manual pre-reset copy | `~/dashboard-data/dashboard.db.backup` |
| Daily automatic snapshot | `~/dashboard-data/backups/dashboard-YYYYMMDD.db` |
| Pre-replace snapshot from `/plan` | `~/dashboard-data/backups/pre-plan-<id>-<stamp>.db` |

To restore, stop the server and copy the file over `dashboard.db`, removing any `-wal` and `-shm`
alongside it first.

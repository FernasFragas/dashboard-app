# M10 · Any plan — the dashboard stops knowing about `master-plan-v5.md`

**Goal:** someone clones this repo, points it at *their* plan document, and gets a working
tracker — no Go edits, no migration, no rebuild. Today, changing the plan means editing code in
five places.

**Depends on:** M7 shipped. **Should precede M9** — gamification adds XP rules keyed to plan
vocabulary, and porting those twice is wasted work.
**Sequencing:** post-v1. **Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md)

> **Not multi-user.** v2 §4 rules out accounts and public deployment permanently, and this does
> not touch that. "Shareable" here means the *repo* is usable by someone else with their own
> plan on their own machine — one plan, one user, one binary, as before.

---

## 0 · What is actually welded to this plan

Measured, not assumed:

| Layer | Coupling |
|---|---|
| **Calendar** | `internal/api/time.go` holds a `planWindows` literal: all 19 windows with hardcoded dates. **This duplicates the seeded `weeks` table** — two sources of truth for the same calendar, which can silently disagree. |
| **Schema** | `CHECK (project IN (…))` in `goals` and `tasks`; `CHECK (id IN (…))` on `categories`; `phase IN ('P1','P2','P3')`; `target_tier IN ('Practitioner','Expert')`. New vocabulary needs a migration. |
| **Code mirrors** | The project list appears in four places: the two CHECKs, `allowedProjects` in `internal/api/server.go`, `ProjectID` in `web/src/api/client.ts`, and `projects[]` in `TodayPage.tsx`. |
| **Parser** | Heading grammar, the `→ **Done =**` marker, the `"answer in writing:"` prose split for checkpoints, and fixed table layouts for goals, metrics and rhythm. |
| **Curated data** | `cmd/seedgen/skills.go` hardcodes 17 skills and a 66-entry `taskSkills` map keyed by this plan's exact `seed_key`s. `parse.go` hardcodes `metricGuides` (keyed by exact metric name), `checkpointHelpers` (positional), and `extraMetricDefNames`. The generator is strict both ways, so **a different plan makes every one of those an error**. |
| **Distribution** | `seed.json` is `go:embed`'d — one plan per binary. `active_limit: 3` is a literal in `handlers.go`. |

## Decisions taken

1. **Scope:** anyone's plan. One active plan at a time; no `plan_id` on every table.
2. **Authoring:** stays markdown. The grammar becomes configurable rather than compiled in.
3. **Curated data moves into the plan document** — a plan that doesn't carry its own skills and
   vocabulary isn't portable, it is just different tasks for *this* plan's skills.
4. **Vocabulary becomes reference tables with foreign keys**, replacing the `CHECK` lists.

---

## Stage 1 · One calendar, and it is data

Delete `planWindows` from `internal/api/time.go`. `PlanWeekFor` takes the windows as an
argument — it stays a pure function (ADR-004) — and the handler passes `store.ListWeeks`.

This is worth doing on its own merit even if the rest is never built: the duplication is a live
correctness risk today, and the `weeks` table already holds the truth.

- `firstPlanWeekCode` / `lastPlanWeekCode` derive from the same list.
- Empty `weeks` (a plan with no calendar) → `state: "not_started"`, and every screen stays
  usable. The dashboard must not require a week to exist.

**Done when:** `grep -rn '2026-' internal/ --include='*.go' | grep -v _test` returns nothing, and
a test with two fabricated windows proves `PlanWeekFor` follows the data rather than the literal.

---

## Stage 2 · Vocabulary as reference tables

`migrations/004_vocabulary.sql`. `docs/DATABASE.md` first, as always.

| New table | Columns | Replaces |
|---|---|---|
| `projects` | `id TEXT PK · label · sort_order` | the `project` CHECK in `goals` and `tasks` |
| `phases` | `id TEXT PK · label · sort_order` | `CHECK (phase IN ('P1','P2','P3'))` |
| `skill_tiers` | `id TEXT PK · label · sort_order` | `CHECK (target_tier IN (…))` |

- `goals.project` and `tasks.project` become `REFERENCES projects(id) ON DELETE RESTRICT`.
- `categories` keeps its table but **loses the `CHECK` on `id`** — the eight slugs are this
  plan's, not every plan's.
- `goal_status` and `task_status` **stay hardcoded**: `backlog/active/done` and `todo/done` are
  the app's own model, not the plan's vocabulary. A plan does not get to invent a fourth column.

SQLite cannot add a foreign key to an existing column, so this is a table rebuild:
`PRAGMA foreign_keys=OFF` → create new → copy → drop → rename → `foreign_keys=ON` → verify.
Do it inside the migration's transaction, and assert row counts before and after.

**API and UI stop mirroring the list:**

- Delete `allowedProjects` from `internal/api/server.go`; validate against the `projects` table.
- New `GET /api/projects`, and `active_limit` moves out of the handler literal into plan config.
- `ProjectID` in TypeScript becomes `string`; `TodayPage`'s `projects[]` becomes a query.

**Done when:** adding a project to a plan requires no migration and no code change, and the
four-place duplication is one place.

---

## Stage 3 · The plan document carries its own data

New markdown sections, in the existing document's voice. Everything is **optional** — a plan
with no skills, no metrics and no checkpoints must load and produce a usable tracker.

### `## Projects`

```markdown
| id | label |
|---|---|
| synapse | Synapse |
| gateway | LLM Gateway |
```

### `## Skills`

Replaces `cmd/seedgen/skills.go`'s 17 hardcoded entries.

```markdown
| code | name | description | associate when | target |
|---|---|---|---|---|
| evals | Evaluation systems | Golden sets, rejection metrics, regression gates. | the task creates, runs, or gates on an evaluation. | Expert |
```

### Task → skill links, inline

Replaces the 66-entry `taskSkills` map. Same shape as the existing `→ **Done =**` marker, so it
reads like the document already does:

```markdown
- [ ] **[synapse] Golden set committed**
  1. Create `eval/golden/`.
  → **Done =** set is in git.
  → **Skills =** evals, tooling
```

### Metric help and checkpoint helpers

Two extra columns on the existing Metrics targets table (`definition`, `how to measure`), and a
`## Checkpoint questions` section carrying question and helper as a table — replacing both the
`"answer in writing:"` prose split and the positional `checkpointHelpers` array.

**Consequence, stated plainly:** `master-plan-v5.md` must gain these sections, and until it
does, this repo's own plan is one of the plans that does not load. Stage 3 ends with
`master-plan-v5.md` updated and `seed.json` regenerating byte-identically to today's content.

**Done when:** `cmd/seedgen/skills.go` and the curated maps in `parse.go` are deleted, and the
generator reads all of it from the document.

---

## Stage 4 · Configurable grammar

A `plan.yaml` beside the plan document. Deliberately a **bounded profile, not a parser
generator** — named knobs for the things that vary, with a regex escape hatch only for the two
line shapes that genuinely differ between documents.

```yaml
plan:
  id: master-plan-v5          # identity; see Stage 5
  name: "Master Plan v5"
  start_year: 2026
  active_goal_limit: 3

sections:                      # heading text → what it holds
  projects: "Projects"
  goals: "Goals (Kanban seed)"
  skills: "Skills"
  rhythm: "Operating system"
  categories: "Log categories"
  metrics: "Metrics targets"
  checkpoints: "Checkpoint questions"

markers:                       # the "→ **X =**" labels
  done: "Done"
  skills: "Skills"

week_heading:                  # the one genuinely document-shaped thing
  pattern: '^##\s+(?P<code>[WB]\d+)\s*·\s*(?P<dates>.+?)(?:\s+—\s+(?P<focus>.*))?$'
  date_format: "Mon D–D"
```

Rules that keep this from becoming a language:

- **Every knob has a default** matching `master-plan-v5.md`, so an unconfigured plan in that
  shape still works and `plan.yaml` is optional.
- **M11 front matter uses the same keys.** For browser upload, the profile can live at the top
  of the markdown file between `---` delimiters. Front matter wins over `plan.yaml` when both
  are present, so one uploaded file is enough.
- **The parser stays strict.** An unrecognised line inside a week remains a hard error — that
  strictness is what stopped a week of tasks vanishing silently, and a configurable grammar
  makes it more valuable, not less.
- **No arbitrary code.** Regexes only, and only for `week_heading` and the task bullet.

**Done when:** a second, differently-shaped plan document parses using only `plan.yaml`, with
no Go changes. That second document is the acceptance artefact — write a small one.

---

## Stage 5 · Loading a plan, and not corrupting the last one

### `-plan` flag

```sh
dashboard -plan ~/plans/my-plan.md          # plan.yaml beside it, if present
```

`seed.json` stays embedded as the zero-config default, so the existing experience is unchanged.
`-plan` regenerates the seed at boot instead of reading the embedded copy.

### Plan identity — the safety property

The seed loader is an **additive upsert**. Pointing it at a different plan would merge two plans
into one database: two sets of weeks, two sets of goals, tasks whose `seed_key`s collide by
accident. That is data loss wearing a friendly face.

New `plan_meta` table: `id TEXT · name · loaded_at`. At boot:

| Database | Plan | Behaviour |
|---|---|---|
| empty | any | load it, record the id |
| holds plan X | plan X | normal additive upsert, as today |
| holds plan X | plan Y | **refuse, exit non-zero**, and say to use a different `-data` directory or `-plan-reset` |

`-plan-reset` deletes and reseeds — and refuses unless a backup exists, because it destroys
every task status and goal position.

### `dashboard plan validate`

The command that makes this usable by someone who did not write the parser. Reports, with line
numbers: unknown project tags, tasks with no skills, skills referenced but not defined, metric
help missing, overlapping week windows, unparsed lines. Exits non-zero on any error.

This is the UX of the whole feature. A shareable tool whose failure mode is a Go stack trace is
not shareable.

### Docs

`docs/PLAN-FORMAT.md`: the document format, every section, every marker, `plan.yaml`'s knobs and
defaults, and a minimal complete example plan that fits on one screen.

---

## Testing

1. `PlanWeekFor` against fabricated windows — not this plan's dates.
2. A plan with no skills / no metrics / no checkpoints / no weeks loads and serves every screen.
3. Adding a project to a plan document requires no migration: seed, restart, create a task with
   the new tag.
4. Foreign keys hold: a task cannot reference an unknown project; deleting a referenced project
   is refused.
5. The table rebuild in migration 004 preserves every row — counts and a checksum before/after.
6. **The second plan document** parses with only `plan.yaml`, and its seed contains its own
   vocabulary and skills.
7. `master-plan-v5.md` still produces the byte-identical `seed.json` it produces today.
8. Loading plan Y into a database holding plan X exits non-zero and changes nothing.
9. `plan validate` reports a line number for each of: bad project tag, task with no skills,
   undefined skill reference, overlapping weeks.

## Acceptance

1. No Go file contains a date, project id, skill, or metric name from any specific plan.
2. A second plan document, in a different shape, runs the dashboard end to end.
3. Swapping plans needs no migration and no rebuild.
4. `master-plan-v5.md` behaves exactly as it does today, seed byte-identical.
5. Pointing at a different plan than the database holds is refused, not merged.
6. `docs/PLAN-FORMAT.md` is sufficient to author a plan without reading Go.
7. `DATABASE.md` matches migration 004, and both land before the code reading the new tables.

## What stays hardcoded, on purpose

- `goal_status` (`backlog/active/done`) and `task_status` (`todo/done`) — the app's model.
- The eight-category *shape* of the quick-log bar is data, but "log entries have a category" is
  not negotiable.
- Europe/Lisbon stays the **default** `-tz`, not a plan property: the day boundary belongs to
  the person, not the plan.
- One active plan per database. Multi-plan was considered and rejected — it puts a `plan_id` on
  nearly every table and every query to serve a case a single user rarely has.

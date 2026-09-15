# Product rules and glossary

What the app is for, the rules it follows, and the words it uses. Technical rules live in
[ARCHITECTURE.md](ARCHITECTURE.md) and [adr/README.md](adr/README.md).

## Purpose

An accountability record for one person working through a long career plan. It must be trusted as
a record, be fast enough to use from a phone in seconds, and never make the owner feel punished
for resting. "Shareable" means someone else can run the repository with *their* plan on *their*
machine. It never means multi-user, accounts or public hosting.

## Rules

Each rule has a reason. Changing a rule means changing this document in the same pull request.

### Planning

- **The plan is data.** The app never knows which plan it tracks: projects, phases, categories,
  skills, weeks and checkpoints all come from the loaded plan document.
- **Goals seed into Backlog, and only the owner promotes them.** Choosing what is active is the
  accountability mechanism the board exists to force; deriving it from dates would remove the
  decision.
- **The active-goal limit warns, never blocks.** The API accepts the move and the board shows a
  badge.
- **Every task builds at least one skill** when the plan defines skills. Tasks created before
  skills existed show a "needs skill" badge instead of pretending the rule held.

### Recording

- **Log entries are append and delete only.** The log is a record of what happened, not a
  document. Delete has a short client-side undo hold, because a mis-tap on a phone is otherwise
  unrecoverable.
- **Quick log stays under five taps.** Skills are optional on log entries so the most-used action
  never grows a required choice.
- **One daily review per Lisbon date**, three bullets (learned, issue, next). It may be backdated,
  because the Sunday ritual often happens on Monday; it may not be future-dated; an entirely blank
  review cannot exist, because it would count toward the streak.
- **Metric names are free text.** The picker enforces consistent spelling so a new metric never
  needs a migration.
- **Checkpoint answers are saved whole.** A blank answer is an answer.

### Game layer

- **XP mirrors real value.** Large amounts only for real outcomes (goal done, exam passed, post
  published). Opening the app earns nothing; streaks earn no XP.
- **Rest is sacred.** A zero-XP Sunday shields the streak instead of breaking it.
- **Celebrate once.** Each level-up or unlock is shown exactly once.
- **No guilt mechanics.** No decay, no "falling behind" copy, no red inactivity states.
- **Honesty is rewarded.** Publishing a worse-than-hoped number earns a badge, not a penalty.
- **Respect `prefers-reduced-motion`.** No sound.

### Experience

- **Phone first.** Tap targets are at least 44 px; the bottom tab bar is within thumb reach;
  drag-and-drop is desktop-only and phones use a tap-to-move sheet.
- **Dark theme only.**
- **No offline mode.** Without a service worker, an unreachable tailnet fails plainly instead of
  showing stale data that looks current.

## Glossary

| Term | Meaning | Where |
|---|---|---|
| **Plan** | One markdown document: projects, goals, dated weeks or blocks of tasks, skills, metrics, checkpoints. A database holds exactly one plan id. | `master-plan-v5.md`, `plan_meta` |
| **Profile** | Parser configuration: section names, markers, week-heading and task-bullet regexes. From `plan.yaml` or front matter. | `internal/plan/config.go` |
| **Phase** | Top-level plan period such as `P1`, derived from `# Phase N` headings and goal targets. | `phases` |
| **Week / block** | A dated plan window. `W1…W16` are the weeks of Phase 1; `B1…B7` are two-week blocks in Phase 2. Gaps are allowed, overlaps are not. | `weeks` |
| **`week.state`** | `not_started`, `active` or `plan_complete`, relative to today. | `GET /api/dashboard`, `internal/api/time.go` |
| **`task_week`** | Object `{code, start_date, end_date, focus}` for the week the task list came from. Set even outside the plan window; new tasks go to `task_week.code`. | `GET /api/dashboard` |
| **Rhythm / operating system** | Which kind of work belongs to which weekday (`Tue–Wed` is stored as `2,3`). | `rhythm` |
| **Goal** | Kanban card with `done_means` and a free-text `target`. Status `backlog`, `active` or `done`. Seeded goals carry a `code` such as `G7`. | `goals` |
| **Task** | Checklist item in a week: `steps`, `done_means`, status `todo` or `done`, one or more skills. | `tasks`, `task_skills` |
| **`seed_key`** | Identity of a seeded task: `"<week>:<slug(title)>"`. Null for tasks created in the app. Retitling a task changes it. | `tasks.seed_key` |
| **Category** | A quick-log button (id, label, emoji). Retired rather than deleted when a replacement plan drops it but old entries still use it. | `categories` |
| **Log entry** | Record of something that happened. Input goes to `url` when it parses as http(s), otherwise to `note`. A linked goal counts as "proof of motion". | `log_entries`, `log_skills` |
| **Daily review** | Three bullets plus optional minutes, one per Lisbon date. | `daily_reviews` |
| **Metric / metric def** | A reading (`name`, `value`, `unit`) / its catalogue entry with baseline, target, definition and how to measure. | `metrics`, `metric_defs` |
| **Checkpoint** | Question set with helper text, attached to a week that has a task titled "…Checkpoint…". | `checkpoints` |
| **Skill** | A named capability with `associate_when` (the tagging rule) and an optional `target_tier`. Its stats are derived. | `skills` |
| **XP event** | Ledger row for an XP-earning action; the amount is copied from `xp_rules` at award time. | `xp_events`, `xp_event_skills` |
| **Level / title** | Derived from total XP; level n starts at `25 * n * (n + 1)`; titles follow a career ladder. | `internal/store/game.go` |
| **Skill tier** | Derived from per-skill XP: Untrained (0), Novice (1), Apprentice (60), Practitioner (150), Adept (300), Expert (500). Every linked skill receives an event's full amount. | `internal/store/game.go` |
| **Streak** | Dashboard: consecutive active Lisbon days ending today or yesterday. Game: days with at least one XP event, with zero-XP Sundays shielded. | `internal/api/time.go`, `internal/store/game.go` |
| **Achievement / unlock** | Catalogue row with a frozen `code`, a condition in Go, and one stored unlock time. | `achievements`, `internal/api/game.go` |
| **Game envelope** | Optional `game` field on write responses: `xp_awarded`, `level_up`, `unlocks`, `event`. | [API.md](API.md) §1 |
| **Reseed modes** | *Additive* on every boot, *reset* with `-plan-reset`, *replace* through `/plan` with a new plan id. | `internal/seed/load.go`, [PLAN_CHANGE_RUNBOOK.md](PLAN_CHANGE_RUNBOOK.md) |
| **Pairing code** | One-time 90 s code behind the desktop QR; redeeming it stores the token on the phone. | `pairing_codes`, `internal/api/pair.go` |

## Projects the default plan tracks

From `## Projects` in `master-plan-v5.md`. These are plan data: tests use `synapse` and `gateway`
only as fixtures, and app code must never list them.

| id | Label | What it is |
|---|---|---|
| `synapse` | Synapse | synapsePlatform, which also holds `infra/` and `ml/` |
| `gateway` | LLM Gateway | LLMGateway-Go |
| `dash` | Dashboard | this app |
| `mcap` | mcap-tools | the one sanctioned extra repository (W1 only, frozen afterwards) |
| `oss` | Open source | foxglove/mcap contributions |
| `learn` | Learning | courses, reading, certifications |
| `write` | Writing | Medium posts |
| `career` | Career | CV, applications, networking |
| `all` | All repos | cross-repository tasks |

The plan's first guardrail applies to this repository: the dashboard stays a utility.

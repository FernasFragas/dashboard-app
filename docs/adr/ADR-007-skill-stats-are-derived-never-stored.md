# ADR-007: Derive skill progress from associations instead of storing counters

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-001, ADR-004, docs/DATABASE.md §4.12–4.14 |

## 1. Context and Problem Statement

M8 adds a skill profile: seventeen named skills, every task linked to at least one, log entries
optionally linked as evidence. The Skills tab shows per-skill progress — tasks done out of
tasks linked, how many log entries reference it, when it was last touched.

Those numbers can either be stored on the `skills` row and updated whenever a task is toggled,
or computed from the join tables at read time. This decides which. It does **not** decide the
XP and level mechanics of M9, which sit on top of the same associations.

## 2. Decision Drivers (ranked)

1. **The numbers must never be wrong.** A profile that disagrees with the task list is worse
   than no profile — it is the artefact an interviewer would be shown.
2. **Write paths stay simple.** Toggling a task is the app's most frequent write and already
   carries optimistic concurrency; every extra side effect is a way for it to fail.
3. **Read cost at this size.** A few hundred tasks and a few thousand log entries, single user.
4. **Retrofitting.** Backfills, re-seeds and hand edits in a SQLite browser must not corrupt
   state.

## 3. Options Considered

- Option 0 — Store counters on `skills`, update on every task/log write *(status quo for most apps)*
- Option 1 — Derive on read from `task_skills` / `log_skills`
- Option 2 — Derive on read, plus a cache table refreshed by trigger

### Option 0: Stored counters

- **Description** — `skills.tasks_done`, `skills.evidence_count`, `skills.last_activity_at`,
  incremented and decremented by the task and log handlers.
- **Pros** — Reads are a single-row lookup (#3). Trivially fast at any size.
- **Cons / risks** — Every counter is a second source of truth that can drift from the rows it
  summarises (fails #1). Untoggling a task must decrement, deleting a linked task must
  decrement, re-seeding must not, and editing a row by hand in a SQLite browser silently
  corrupts the count (fails #4). The task-toggle handler grows a write to a second table inside
  the same transaction (fails #2).
- **Cost** — Cheap to add, expensive forever: every new write path has to remember the counters.

### Option 1: Derive on read

- **Description** — `GET /api/skills` computes done/total, evidence count and last activity with
  aggregate queries over the join tables. `skills` holds only description-style columns.
- **Pros** — Cannot drift: there is one source of truth and it is the associations themselves
  (#1). Task toggle is unchanged — it writes `tasks` and nothing else (#2). A hand edit or a
  re-seed produces correct numbers on the next read (#4).
- **Cons / risks** — Every Skills-tab read runs aggregates. Cost grows with task and log count,
  and a naive implementation would run a query per skill (N+1).
- **Cost** — Two indexes and one grouped query per stat.

### Option 2: Derive plus a trigger-maintained cache

- **Description** — Option 1's queries, with a materialised table kept current by SQLite
  triggers.
- **Pros** — Fast reads and a single logical source (#1, #3).
- **Cons / risks** — Triggers are invisible from Go: nothing in the codebase shows why a number
  changed, which contradicts ADR-004's rule that the store is the only place persistence logic
  lives. Adds a schema object to keep in step with every future column.
- **Cost** — The highest, for a read that is not slow.

### Comparison

| Driver (ranked) | Opt 0 Counters | Opt 1 Derived | Opt 2 Cache |
| --- | --- | --- | --- |
| 1. Numbers never wrong | ❌ | ✅ | ⚠️ |
| 2. Simple write paths | ❌ | ✅ | ✅ |
| 3. Read cost | ✅ | ⚠️ | ✅ |
| 4. Safe to retrofit | ❌ | ✅ | ⚠️ |

## 4. Decision

> **We will derive every per-skill statistic at read time from `task_skills` and `log_skills`,
> and store no progress columns on `skills`, because a stored counter is a second truth that
> drifts (#1) and because it would add a side effect to the app's most frequent write (#2).**

We accept that the Skills tab pays for aggregates on every read. At this size that is a few
grouped queries over a few thousand rows on a local SQLite file — far below the threshold where
anyone would notice. The mitigation for the real risk is structural, not architectural: the
stats are fetched with **one grouped query per statistic across all skills**, never a query per
skill.

If the Skills tab ever becomes slow, the fix is Option 2 and the associations are already the
right shape for it — nothing needs redesigning, only caching.

## 5. Consequences

**Positive**
- Toggling a task remains a single-table write with an unchanged `If-Match` story.
- Re-seeding, deleting a goal, deleting a task, or editing rows by hand all leave the profile
  correct, because there is nothing to keep in step.
- M9's XP ledger can layer on the same associations without a second set of counters to
  reconcile against.

**Negative**
- Skills-tab reads are aggregate queries. *Mitigation:* `idx_task_skills_skill` and
  `idx_log_skills_skill`, and a hard rule of one grouped query per statistic — a store test
  asserts the endpoint's query count does not scale with the number of skills.
- Per-skill history over time is not available, because nothing is recorded per point in time.
  *Mitigation:* accepted for M8. M9's `xp_events` ledger is where a time series would come
  from, and it is append-only for exactly that reason.

**Follow-up work created**
- `idx_task_skills_skill` and `idx_log_skills_skill` in the M8 migration.
- A store test proving the N+1 rule.
- M9 reads its skill XP from the same join tables, never from a cached column.

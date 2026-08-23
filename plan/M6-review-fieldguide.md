# M6+ · Review field guide — every field self-explanatory

**Goal:** a first-time user — or you, in November, tired — completes a full Sunday review without
opening any plan doc. Every input says what it means, shows a realistic example, and one tap
reveals the fuller guidance.

**Depends on:** M6 (the Review screen exists and renders metrics + checkpoints).
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Sibling:** [M6-log-review.md](M6-log-review.md)

> **Sequencing:** an addendum to M6, not a shipping concern. Land it after M6 and before M7.
> Renamed from `M7-review-fieldguide.md` to match the index's link and its own content.

---

## 0 · Prerequisite — the seed loader must be able to update reference data

**This blocks everything below.** `docs/DATABASE.md` §8 makes the loader insert-only: it never
updates an existing row. That rule exists to protect user state — a re-seed must never undo a
finished task or reopen a completed goal.

But this milestone adds columns to `metric_defs` and `checkpoints`, and every one of those rows
already exists in your database. Insert-only means they would stay `NULL` forever, and the only
way to see the new help would be deleting `dashboard.db`.

**Change the rule to distinguish reference columns from state columns:**

| Table | Reference columns — **upserted** | State columns — **never touched** |
|---|---|---|
| `weeks` | phase, start_date, end_date, focus, sort_order | — |
| `rhythm` | weekdays, slot, sort_order | — |
| `categories` | label, icon, sort_order | — |
| `metric_defs` | slug, unit, baseline, target, definition, how_to_measure, sort_order | — |
| `checkpoints` | questions, helpers | **answers, completed_at** |
| `goals` | — | everything (insert-only, unchanged) |
| `tasks` | — | everything (insert-only, unchanged) |

`goals` and `tasks` keep the insert-only rule exactly as it is — they are the tables that hold
your progress. The reference tables hold nothing you can edit in the app, so upserting them is
how a plan amendment reaches an existing database at all.

**Order of work:** `docs/DATABASE.md` §8 first, then the loader, then its tests. Schema and
policy never change code-first (v2 §5).

**Done when:** a store/seed test seeds, mutates a `metric_defs` row and a checkpoint's
`answers`, re-seeds, and asserts the reference column was corrected while `answers` and
`completed_at` survived untouched. `TestApplyPreservesProgress` still passes unchanged.

---

## 1 · Schema

Into `docs/DATABASE.md` first, then `migrations/002_field_guide.sql`.

### `metric_defs` — three new columns

| Column | Type | Null | Notes |
|---|---|---|---|
| `slug` | TEXT | yes | `UNIQUE`. Stable identifier: `rejection_rate`, `p95_latency`. `name` stays the primary key and the display text. |
| `definition` | TEXT | yes | One line: what the number *is*. |
| `how_to_measure` | TEXT | yes | One line: how to produce it reproducibly. |

Nullable because SQLite cannot add a `NOT NULL` column to a populated table without a default,
and because a metric you invent through the Review screen's "other…" field has no definition.
The seed fills them for every catalogue metric.

`slug` is for stability, not display: renaming a metric's prose name later must not orphan its
help. Nothing in the UI shows it.

### `checkpoints` — one new column

| Column | Type | Null | Notes |
|---|---|---|---|
| `helpers` | TEXT | yes | JSON array, **index-aligned with `questions`**, same length. |

Question and helper travel together so they can never drift apart. The store must reject a
`helpers` array whose length differs from `questions` — same rule already applied to `answers`.

---

## 2 · Seed data

The help text is not in `master-plan-v5.md`, so `cmd/seedgen` has nothing to parse it from. It
lives in a **curated table in `cmd/seedgen`**, keyed by the metric's seeded name, and is merged
into `seed.json` beside the parsed rows. One seed pipeline, one committed artefact, help
reviewed in the diff like everything else.

Parsing stays strict: a curated entry whose key matches no parsed metric is a **hard error**,
not a silent drop. That is what catches the master plan's metric table being edited.

### Metric catalogue

`name` is the seeded prose name and must match exactly. `ttft` is new — the master plan names
TTFT in W8's steps but not in its Metrics targets table, so seedgen appends it.

| slug | name (as seeded) | unit | definition | how to measure |
|---|---|---|---|---|
| `rejection_rate` | Rejection rate v2 vs v1 | % | Share of golden-set summaries `ValidateAgainstEvidence` rejects | `make eval` over the frozen 30-window set; record per prompt × model |
| `p95_latency` | p95 latency (cached) | s | 95th-percentile request latency under the standard mixed profile | Grafana after a 10-min k6 run; never compare across different profiles |
| `p99_latency` | p99 latency | s | 99th-percentile, same profile | Same run as p95 — read both off one k6 pass |
| `ttft` | Time to first token | s | Request to first streamed token through the gateway | TTFT metric during the standard k6 profile |
| `cache_hit_rate` | Cache hit rate | % | hits ÷ (hits + misses) | Gateway cache metric over a 10-min replayed-traffic window |
| `cost_per_1k` | Cost per 1k queries | € | Euro cost per 1,000 requests | API pricing × tokens, or GPU-hour ÷ requests served — say which in the note |
| `throughput_concurrent` | Throughput, concurrent writers | msg/s | Messages per second with N concurrent writers on Postgres | Rerun the W5 load profile; put N in the note |
| `tuned_vs_base` | Fine-tuned vs base on golden set | ratio | Tuned ÷ base rejection rate on the golden set | Eval runner, both providers, same day |
| `oss_prs_merged` | OSS PRs merged | PRs | PRs you opened that a maintainer merged | Count from your GitHub PR list; merged only — open or closed-unmerged do not count |
| `applications_sent` | Applications sent | applications | Applications actually submitted | Count the 📮 entries in the Log for the period; the Log is the record, this is the roll-up |
| `medium_posts` | Medium posts | posts | Posts published, not drafted | Count live URLs; a draft is a Learned bullet, not a metric |

> The last three duplicate what the Log already counts. Kept deliberately — the Log is the
> event record, the metric is the periodic roll-up — and the *how to measure* line points at
> the Log so the two can never disagree about where the truth lives.

### Checkpoint helpers

Seeded questions stay as parsed (`interview-grade eval number?`); helpers are stored beside
them, in order:

| # | Question (as seeded) | Helper |
|---|---|---|
| 1 | interview-grade eval number? | `Good = the exact sentence you'd say out loud, with real X and Y. Not yet = name the one missing piece.` |
| 2 | chaos falsified anything? | `Name the belief that died. Nothing surprised you = you tested too gently — schedule the harder rerun before answering.` |
| 3 | PR merged or stale? | `Status + date of last maintainer contact + your next move.` |
| 4 | fourth repo? | `Yes/no. If yes: which one gets archived this week.` |
| 5 | still AI-infra path? | `Gut check, one paragraph max. Re-decide, don't re-litigate.` |

Both W12 and B7 share the same five, as they already share questions.

---

## 3 · API

`GET /api/metric-defs` (the existing endpoint — **not** `/api/metrics/defs`) gains `slug`,
`definition` and `how_to_measure` per row.

`GET /api/checkpoints/{week}` gains `helpers`, index-aligned with `questions`.

`docs/API.md` updated with both, before the handlers change.

---

## 4 · Frontend

### UX rules

1. Every input has a **one-line helper**, always visible, muted, under the label. No hover-only
   tooltips — phones have no hover.
2. Every text input's **placeholder is a realistic example entry**, never an instruction.
   "Enter text…" is banned.
3. Each section has an **ⓘ toggle** expanding 2–4 lines of full guidance. Collapsed by default;
   state not persisted.
4. Review's **empty state** shows one pre-filled example card so the first use teaches itself.
   It is non-interactive, visibly labelled as an example, and disappears once any review exists.
5. All strings live in **`web/src/copy/review.ts`** (one exported object). Components import
   copy; no helper strings inline in components.
6. **Metric help is data, not code** — it comes from the API, so a metric added later carries
   its own help. This is the one deliberate exception to rule 5.
7. Helpers stay ≤ ~90 characters, so they hold one line on a 380px viewport.

### Files

- `web/src/copy/review.ts` — exports `{ daily, weekPct, metrics, checkpoint }`. Holds the daily
  and section copy below. Does **not** hold metric definitions (rule 6) or checkpoint helpers
  (they come from the API).
- `web/src/components/FieldGuide.tsx` — the one shared ⓘ toggle, used by all three sections.
  Props: `{ children }` plus an accessible label. Button is ≥44px, `aria-expanded` reflects
  state.
- `web/src/pages/ReviewPage.tsx` — renders from copy; no literals.

### Unit field

Currently the unit input is free text and every seeded `unit` is `NULL`. After this change:

- Metric has a `unit` → prefilled and **read-only**, so a series cannot mix units.
- Metric has no `unit` (an "other…" metric) → editable, so a new number can still carry one.

---

## 5 · The copy (verbatim)

### Daily review — "Three bullets. Two minutes. The log is the record."

**Learned today**
- Helper: `One thing you understand now that you didn't this morning — a mechanism, a gotcha, a number. Not an activity.`
- Placeholder: `pgxpool MaxConns above Neon's cap just queues — saw it in the pool wait metric`
- ⓘ Full: `Good entries are specific enough that future-you can act on them without context. "Worked on the eval runner" is a task — it belongs in Today. "ValidateAgainstEvidence rejects on citation index, not text match" is a learning. If the day produced nothing, write what you'd try differently — that counts.`

**Blocker / issue**
- Helper: `What slowed or stopped you — technical, planning, or energy. The thing you'd warn yesterday-you about.`
- Placeholder: `vLLM OOMs at 8k ctx on the 24GB card — need 4k ctx or a bigger card for the bake-off`
- ⓘ Full: `Name it precisely enough that "Next step" can attack it. "None" is a valid, honest answer — don't invent friction. The same issue three days running is a signal to change the plan, not to push harder: raise it at Sunday review.`

**Next step**
- Helper: `The first action of your next session. One verb, one action, startable in under 2 minutes.`
- Placeholder: `Add -runs=5 flag to cmd/evalrun and rerun the golden set`
- ⓘ Full: `This field kills session start-up cost. Write it as an instruction to tomorrow-you: a file, a command, a target. "Continue eval work" fails the test. "Wire rejection-rate calc into evalrun's summary table" passes.`

**Focused minutes** *(optional)*
- Helper: `Minutes of actual focused work — not elapsed time. Rough is fine. Blank on rest days.`
- ⓘ Full: `Feeds the weekly load view against the 2–3h/day target. Under-counting beats flattering yourself — the number is for pacing, not judgment. An empty Sunday is the plan working, not failing.`

### Week completion (read-only)

- Helper under the % bar: `Done ÷ all tasks for the current plan week. A pace signal, not a scorecard — low by Thursday means apply the slip order (guardrail #4), not extend hours.`

### Metric quick-add — "Only measured numbers."

- Section helper: `From a run, a dashboard, or a bill. If you estimated it, it's a Learned bullet, not a metric.`
- **Metric** (picker): each option renders name + definition + how it's measured, from the API.
- **Value** — Helper: `Exactly as measured. Same load profile every time, or the series is garbage.`
- **Unit** — prefilled from the definition; read-only when the metric has one.
- **Note** — Helper: `One line of context so the number survives 3 months: profile, config, commit.`
  Placeholder: `mixed k6 profile · 2 concurrent writers · commit a1b2c3`

### Checkpoint form (W12 · B7)

- Section helper: `Written answers. 45 minutes. No editing afterwards — the log is the record.`
- Question helpers come from the API (§2), not from `copy/review.ts`.

---

## 6 · Tests

**Backend**
1. Loader upserts a changed `metric_defs.definition` onto an existing row.
2. Loader leaves `checkpoints.answers` and `completed_at` untouched across a re-seed that
   changes `questions`/`helpers`.
3. `TestApplyPreservesProgress` unchanged and passing — goals and tasks are still insert-only.
4. Store rejects a `helpers` array whose length differs from `questions`.
5. seedgen errors when a curated help entry matches no parsed metric.
6. Every seeded `metric_defs` row has a non-null `slug`, `definition` and `how_to_measure`.

**Frontend**
7. Each daily field renders its helper and its example placeholder.
8. `FieldGuide` toggles, and `aria-expanded` tracks it.
9. Metric picker shows definition + how-to-measure for a selected metric, from mocked API data —
   asserting the component reads the API, not a local constant.
10. Unit is read-only when the definition has a unit, editable when it does not.
11. Checkpoint questions render with their helpers on W12.
12. Empty state renders the example card, and disappears once a review exists.

---

## 7 · Acceptance

1. Every Review input shows a one-line helper and an example placeholder.
2. ⓘ expands and collapses full guidance per section.
3. Every seeded metric shows definition + how-to-measure in the picker, sourced from the API.
4. Checkpoint questions render with helpers on W12 and B7.
5. No helper copy inline in components — `rg -n "Helper:|placeholder=\"[A-Z]" web/src/pages` returns
   only bindings to `copy/review.ts`, never literals.
6. `docs/DATABASE.md` matches `migrations/002_field_guide.sql` exactly, and both landed before
   the code that reads the new columns.
7. An existing database picks up the new help on restart — **without** deleting `dashboard.db`,
   and without losing any goal status, task completion or checkpoint answer.
8. First-time test: a full Sunday review completed with zero references to plan docs.
# M6 · Log + Review

**Depends on:** M3, M4 (shell + mutation conventions).
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §1 "Screens", §2

Two screens. Log is the record of what happened; Review is the ritual that closes the week.

## Log screen

- Reverse-chronological feed from `GET /api/logs`, **grouped by Lisbon day** (not UTC — a
  00:30 entry belongs to the day you were awake for).
- Filter chips per category, same multi-select + URL-persistence pattern as M4's project chips.
- Weekly recap card pinned at top: "Week of Sep 21 — 2 applications · 4 modules · 1 benchmark ·
  1 OSS comment", from `GET /api/logs/summary?range=week`.
- Entries render title, note, and a note starting `http` as a tappable link, plus the linked
  goal's code when set.
- Infinite scroll on the `limit`/`cursor` pagination from M3.

**Deletion: delete only, with undo.** No edit path — the log is a record of what happened, not
a document you maintain. Deleting shows a brief undo toast (client-side hold, then the DELETE
fires). A mis-tapped delete on a phone is otherwise unrecoverable.

## Review screen

### 1 · Daily 3-bullet form

`learned / issue / next`, upsert by date via `POST /api/reviews`.

- Opens on **today** by default, with a **date picker to backfill**. The Sunday ritual often
  happens Monday morning; without backfill those reviews land on the wrong day and break the
  streak they exist to prove.
- Dates with an existing review are marked in the picker so you can see the gaps.

### 2 · Week completion bar

Done tasks / total tasks for the current plan week, from the dashboard bundle.

### 3 · Metric quick-add

- Name comes from a **picker seeded from `metric_defs`** (the master plan's Metrics targets
  table), showing baseline and target beside the field — "p95 latency (cached) · target <0.8s".
- **Free-text "other…" fallback** for a metric the plan doesn't list, so a new number never
  requires a rebuild.
- The picker exists to keep spelling consistent: one typo forks a series into `p95` and
  `P95 latency`, which is the failure a personal metrics tracker cannot survive.
- Value + optional unit + optional note → `POST /api/metrics`. Last few readings for that name
  shown inline as text (charts are stretch, per v2 §3).

### 4 · Checkpoint form — **built in v1**

On `W12` and `B7`, the Review screen renders the checkpoint's five questions from the
`checkpoints` table as a form; answers save as a JSON array and stamp `completed_at`.

The five, from W12: interview-grade eval number? · chaos falsified anything? · PR merged or
stale? · fourth repo? · still AI-infra path? B7 reuses the same five ("same 5-question format").

> **Note:** the first checkpoint is Nov 9 — long after this build window. It's built now because
> it's the same form machinery as the daily review, and by November the context is gone. The
> parse of those questions out of a prose task step is the brittle part; `seed.json` is
> committed and reviewed, so a bad split is caught at review, not in November.

## Tests

1. Feed day-grouping: a `23:30Z` and a `00:30Z` entry land in the correct **Lisbon** day groups.
2. Delete + undo: undo within the window issues no DELETE; letting it lapse issues exactly one.
3. Review upsert: saving twice for the same date updates rather than duplicating.
4. Backfill: picking an earlier date writes to that date, not today.
5. Metric picker: selecting a seeded name submits that exact string; "other…" accepts free text.
6. Checkpoint form renders on W12 and is absent on W5.

## Done when

1. The full Sunday ritual — 3 bullets + one metric + a glance at the weekly recap — works on
   the phone.
2. A review written Monday for Sunday lands on Sunday.
3. Deleting a log entry and hitting undo leaves it in place.

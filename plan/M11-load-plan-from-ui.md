# M11 · Load a plan from the UI

**Goal:** drop a `.md` plan into the dashboard from the browser — see exactly what it will do,
then apply it. No terminal, no `-plan` flag, no restart.

**Depends on:** M10 (parser is runtime-callable, vocabulary is data, `plan_meta` exists).
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Sibling:** [M10-any-plan.md](M10-any-plan.md)

---

## 0 · What M10 already built

Do not rebuild any of this:

| Exists | Where |
|---|---|
| Runtime-callable parser | `internal/plan` — `ParseWithProfile(source, profile)`, `ParseFile(path)` |
| Configurable grammar | `plan.Profile`: `Settings`, `Sections`, `Markers`, `WeekHeading`, `TaskBullet` |
| Plan identity + refusal | `plan_meta` table, checked by the seed loader |
| Vocabulary as data | `projects`, `phases`, `skill_tiers` (migration 004) |
| CLI equivalent | `-plan` and `-plan-reset` flags |

M11 is the **transport and the safety story**, not the parsing.

---

## 1 · The architectural reversal, and how to make it safe

`docs/DATABASE.md` §8 states: *"The server never reads the markdown, so a parser bug can never
prevent a boot."* That property is why `seed.json` is generated and committed. This feature
makes the server parse markdown, so it must not quietly lose that property.

**The rule that preserves it:** the server parses markdown **only on an explicit user action,
never at boot.** Boot keeps reading the stored document exactly as it does today. A plan that
fails to parse produces an error in the browser; it can never produce a server that won't start.

Record as **ADR-009 · Parse plan documents at upload, never at boot**, and amend DATABASE.md §8
to say the same thing in the same words.

---

## 2 · The flow: preview, then apply — never blind

Three steps, because applying a plan can destroy months of task completions.

```
  upload / paste  →  POST /api/plan/preview  →  review the diff  →  POST /api/plan/apply
                     (parses, changes nothing)                      (requires the preview's hash)
```

### `POST /api/plan/preview`

Body: the markdown, as `text/markdown` or a multipart file part. Response:

```json
{
  "source_sha256": "9f2c…",
  "plan": { "id": "master-plan-v6", "name": "Master Plan v6" },
  "current_plan": { "id": "master-plan-v5", "name": "Master Plan v5" },
  "mode": "replace",
  "parsed": { "weeks": 19, "goals": 18, "tasks": 66, "skills": 17, "projects": 8 },
  "changes": {
    "weeks":  { "added": 2, "updated": 17, "removed": 0 },
    "tasks":  { "added": 4, "unchanged": 62, "orphaned": 3 },
    "goals":  { "added": 1, "unchanged": 17 }
  },
  "at_risk": { "task_completions": 12, "goal_positions": 4, "checkpoint_answers": 1 },
  "errors": []
}
```

`mode` is `additive` when the plan id matches `plan_meta`, `replace` when it differs, `initial`
when the database has no plan.

**Preview changes nothing.** It parses in memory and diffs against the current tables. It is
also the validator: a document that fails to parse returns `errors` with line numbers and a
`200`, because a bad plan is an expected outcome of this endpoint, not a server error.

```json
{ "errors": [
  { "line": 143, "message": "unrecognised line in week W4", "excerpt": "- [x] leftover note" },
  { "line": 201, "message": "task has no skills", "excerpt": "- [ ] **[dash] Ship it**" }
] }
```

### `POST /api/plan/apply`

Body: the same markdown plus `source_sha256` from the preview, and — for `replace` —
`confirm_plan_name` typed by the user.

- Hash mismatch → `409`. You cannot apply a document you did not preview.
- `replace` without a matching `confirm_plan_name` → `422`.
- On success: snapshot first (below), then run the existing seed loader in one transaction.

---

## 3 · Replacing a plan must not delete your history

**This is the sharpest design issue in the feature.** Log entries, daily reviews and metrics are
*your* record of what you did. They are not plan data, and swapping plans must never remove
them — the Log screen exists precisely because it is the thing that survives.

| Table | On replace |
|---|---|
| `weeks`, `projects`, `phases`, `skill_tiers`, `categories`, `metric_defs`, `skills`, `checkpoints` | replaced by the new plan |
| `goals`, `tasks` | replaced — these belong to the plan |
| **`log_entries`, `daily_reviews`, `metrics`** | **kept, always** |

Two consequences that need explicit handling:

1. **`log_entries.category_id` is `ON DELETE RESTRICT`.** Deleting the old plan's categories is
   blocked by any log entry citing them — correctly, since that is what stops history being
   deleted as a side effect. So replace must **not** delete categories still referenced. Keep
   them, mark them retired (`categories.retired_at`), and hide retired ones from the quick-log
   bar while the Log feed still renders them.
2. **`log_entries.goal_id` and `log_skills`** already `SET NULL` / `CASCADE`. An entry whose
   goal or skill vanishes keeps its title, note and date — it just stops pointing at something
   that no longer exists. State this in the preview's `at_risk` block so it is not a surprise.

Migration 005 adds `categories.retired_at TEXT NULL`. `DATABASE.md` first.

### Snapshot before replace

Apply in `replace` mode calls the existing `VACUUM INTO` path (`internal/store`) to write
`backups/pre-plan-<planid>-<timestamp>.db` **before** touching anything, and reports the path in
the response. If the snapshot fails, the apply is refused — no backup, no replace.

---

## 4 · One file, no sidecar: profile in front-matter

M10 puts the grammar `Profile` in a sidecar `plan.yaml`. That does not survive a file picker:
the user uploads one file.

Support **YAML front-matter** at the top of the plan document, holding the same `Profile`:

```markdown
---
plan:
  id: master-plan-v6
  name: "Master Plan v6"
  start_year: 2027
  active_goal_limit: 3
markers:
  done: "Done"
  skills: "Skills"
---

# Master Plan v6
…
```

Rules:

- Front-matter is **optional**; absent means M10's defaults, so today's `master-plan-v5.md`
  uploads unchanged.
- A sidecar `plan.yaml` still works for `-plan` on the CLI. Front-matter wins when both exist.
- Front-matter lines must not shift the line numbers in parser errors — strip it by replacing
  those lines with blanks rather than removing them, or the excerpts in §2 point at the wrong
  line, which is worse than no line number at all.

**This amends M10 stage 4.** Update `plan/M10-any-plan.md` and `docs/PLAN-FORMAT.md` in the same
change.

---

## 5 · Storing the source

New table `plan_sources`: `id INTEGER PK · plan_id TEXT · sha256 TEXT · source TEXT ·
loaded_at TEXT`. Keep the last few per plan.

Worth the row because it makes three things possible that are otherwise guesswork: re-parsing
after a parser fix without asking the user to find the file again, diffing what actually changed
between two uploads, and answering "what exactly did I load in November" from the export. Plan
documents are tens of kilobytes; this is not a size concern.

`GET /api/plan` returns the current plan meta plus the stored source, so the UI can show it.

---

## 6 · UI

A new `/plan` route. **Not a sixth tab** — five is already the limit for a phone tab bar. Reach
it from a small "Plan" affordance in the app shell (desktop rail footer, and a link in the
Review screen's footer on mobile).

The screen, in order:

1. **Current plan** — name, id, when loaded, counts (weeks, goals, tasks, skills).
2. **Load a new plan** — two inputs to the same endpoint:
   - a file picker (`accept=".md,text/markdown,text/plain"`)
   - a paste textarea, because the iOS file picker is unreliable for `.md` and pasting from a
     notes app is often the faster path on a phone
3. **Preview panel**, after upload:
   - errors first, with line number and excerpt, if any — nothing else renders
   - otherwise the diff summary and, for `replace`, a red block naming exactly what is at risk
4. **Apply** — disabled until a clean preview exists. In `replace` mode the user types the new
   plan's name to enable it, and the button says what it will do: *"Replace Master Plan v5 with
   Master Plan v6"*.
5. **After apply** — the backup path, the counts loaded, and a link to Today.

Copy lives in `web/src/copy/plan.ts`, same rule as M6+ and M8.

---

## 7 · Transport safety

This is the first endpoint that accepts an arbitrary body.

- `http.MaxBytesReader` at **1 MB**; over that is `413` with a plain message.
- Reject non-UTF-8 and anything containing a NUL byte — a `.md` picker will happily hand over a
  `.docx`.
- The existing token middleware already covers `/api/*`; no exemption.
- Parse in a goroutine with a timeout, and cap catastrophic backtracking: the grammar is
  user-supplied regex (M10), so a pathological `week_heading` pattern is a plausible way to hang
  the server. Reject a profile regex that fails to compile, and bound parse time.

---

## 8 · Tests

1. Preview of a valid plan changes no rows — count every table before and after.
2. Preview of a malformed plan returns `200` with line numbers, and still changes nothing.
3. Front-matter does not shift reported line numbers.
4. Apply with a mismatched `source_sha256` → `409`.
5. Apply in `replace` mode without `confirm_plan_name` → `422`.
6. **Replace keeps history:** seed plan A, write log entries and a daily review, replace with
   plan B — entries and review survive, `goal_id` nulled where the goal is gone.
7. A category still cited by a log entry is retired, not deleted, and the delete is not
   attempted.
8. Replace writes a snapshot before mutating; a failing snapshot aborts the apply with nothing
   changed.
9. A 2 MB body is rejected with `413`; a binary file is rejected as not-UTF-8.
10. Boot with a corrupt stored plan still starts — the boot path never parses markdown.
11. Frontend: errors render before anything else; Apply stays disabled until a clean preview;
    `replace` requires the typed name.

## 9 · Acceptance

1. A plan `.md` is loaded from the browser on the phone, end to end, with no terminal.
2. A malformed plan shows line numbers in the UI and changes nothing.
3. Replacing a plan preserves every log entry, daily review and metric.
4. Replacing writes a restorable backup first, and names it in the response.
5. Applying a document that was never previewed is impossible.
6. `master-plan-v5.md` uploads unchanged, with no front-matter, and produces the same seed.
7. ADR-009 exists; `DATABASE.md` §8 and `PLAN-FORMAT.md` agree with the code.

## 10 · Explicitly out of scope

- **Editing the plan in the browser.** Upload replaces; authoring stays in your editor.
- **Merging two plans.** `additive` and `replace` are the only modes; a real merge needs a
  conflict model this app has no reason to own.
- **Undo after apply.** The backup is the undo, and it is a file you restore by hand — the same
  recovery path M7 already documents.

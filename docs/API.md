# API

JSON over HTTP, served by stdlib `net/http` ServeMux (ADR-005). Most JSON endpoints live under
`/api/`; `/pair/{code}` is the unauthenticated phone-pairing redemption path. Other non-API
paths serve the embedded SPA.

Schema for every entity: [DATABASE.md](DATABASE.md). Layering rules: [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 1. Conventions

- **Content type:** `application/json; charset=utf-8` on every request with a body and every
  response except `204`.
- **Timestamps in and out:** RFC3339 UTC (`2026-08-22T14:03:00Z`). Dates: `YYYY-MM-DD`, always
  a `Europe/Lisbon` calendar date.
- **Unknown query parameters are rejected** with `400`, not ignored. A typo'd filter that
  silently returns everything is worse than an error.
- **Unknown JSON fields are rejected** with `400` (`DisallowUnknownFields`).
- **Plan vocabulary is data.** Project, phase, category and skill-tier values come from the
  loaded plan, not compiled API enums. Unknown vocabulary references are rejected with `400`.
- **Trailing slashes** are not accepted.

### Authentication

If the server was started with `-token`, every request under `/api/` must carry a matching
`X-Token` header. Missing or wrong → `401`. `GET /api/health` is exempt so a liveness probe
works without the secret.

`GET /pair/{code}` is also exempt because the phone does not have the token yet. It accepts only
one-use, short-lived pairing codes minted by authenticated `POST /api/pair`; see ADR-010.
Request logs redact `/pair/*` and `/api/pair/*` so codes do not land in log files.

The real perimeter is the tailnet ACL (ADR-003); the token is a second lock, not the lock.

### Error envelope

Every non-2xx response body:

```json
{ "error": "title is required" }
```

`412` additionally carries the current server state so the client can reconcile without a
second round trip:

```json
{
  "error": "task was modified elsewhere",
  "current": { "id": 42, "version": 5, "status": "done" }
}
```

### Status codes

| Code  | Meaning                                                                                                      |
| ----- | ------------------------------------------------------------------------------------------------------------ |
| `200` | OK, body follows.                                                                                            |
| `201` | Created; body is the created resource; `Location` header set.                                                |
| `204` | Deleted, no body.                                                                                            |
| `400` | Validation failure — blank title, bad enum, unknown field or query param, malformed JSON.                    |
| `401` | `-token` is set and `X-Token` is missing or wrong.                                                           |
| `404` | No such id.                                                                                                  |
| `409` | Unique constraint, or plan apply hash/preview mismatch.                                                       |
| `412` | `If-Match` version is stale. Body includes `current`.                                                        |
| `413` | Plan upload body is over 1 MB.                                                                                |
| `422` | Well-formed but forbidden by the model — a task with no skills, or replace apply without exact confirmation. |
| `428` | `PATCH` on a versioned resource without an `If-Match` header.                                                |
| `500` | Anything unhandled. Logged once, at the boundary, with the wrapped cause. The client sees a generic message. |

### Optimistic concurrency

Applies to **`goals` and `tasks` only** (see DATABASE.md §6).

- Every representation of a goal or task includes its `version`.
- Single-resource responses set `ETag: W/"<id>-<version>"`.
- `PATCH` **requires** `If-Match: W/"<id>-<version>"`. Absent → `428`. Stale → `412`.
- A successful `PATCH` bumps `version` and returns the new `ETag`.

### Game envelope

Writes that award XP or unlock achievements include an optional `game` field in the same
response. The original resource fields remain at the top level, so older clients can ignore
`game`.

```json
{
  "id": 41,
  "status": "done",
  "game": {
    "xp_awarded": 10,
    "level_up": {
      "level": 7,
      "title": "Platform Engineer",
      "total_xp": 1400,
      "next_threshold": 1800
    },
    "unlocks": [
      {
        "id": 3,
        "code": "gatekeeper",
        "name": "Gatekeeper",
        "description": "Complete the W3 regression-gate-in-ci task.",
        "sort_order": 30,
        "unlocked_at": "2026-09-24T17:40:00Z"
      }
    ],
    "event": {
      "id": 12,
      "source_type": "task",
      "source_id": 41,
      "amount": 10,
      "created_at": "2026-09-24T17:40:00Z",
      "skill_names": ["Evaluation systems"]
    }
  }
}
```

`PATCH /api/tasks/{id}`, `PATCH /api/goals/{id}`, `POST /api/logs`,
`POST /api/reviews`, `POST /api/metrics`, and `PUT /api/checkpoints/{week}` can include it.
No `game` field means the write was successful but caused no new XP, level-up, or unlock.

---

## 2. Endpoint summary

| Method + path                 | Purpose                                                             |
| ----------------------------- | ------------------------------------------------------------------- |
| `GET /api/health`             | Liveness. Exempt from the token check.                              |
| `POST /api/pair`              | Mint a short-lived phone pairing URL for the current route.         |
| `GET /api/pair/{code}.svg`    | QR SVG for the minted pairing URL. Requires `X-Token` if configured. |
| `GET /pair/{code}`            | Redeem a one-use pairing code. Exempt from token check.             |
| `GET /api/dashboard`          | Today bundle: plan week, rhythm slot, week tasks, counters, streak. |
| `GET /api/goals`              | List, optionally filtered by status or project.                     |
| `POST /api/goals`             | Create.                                                             |
| `PATCH /api/goals/{id}`       | Move column / reorder / edit. Requires `If-Match`.                  |
| `DELETE /api/goals/{id}`      | Delete. Linked tasks and logs survive with `goal_id` nulled.        |
| `GET /api/tasks`              | Week list, filterable by week and project.                          |
| `POST /api/tasks`             | Add an ad-hoc task.                                                 |
| `PATCH /api/tasks/{id}`       | Toggle done / edit. Requires `If-Match`.                            |
| `DELETE /api/tasks/{id}`      | Delete a hand-created task.                                         |
| `GET /api/logs`               | The accomplishment feed, paginated.                                 |
| `POST /api/logs`              | The + button.                                                       |
| `DELETE /api/logs/{id}`       | Remove a typo entry.                                                |
| `GET /api/logs/summary`       | Counters per category.                                              |
| `GET /api/game/profile`       | Player level, XP, streak, and unlocked badges.                      |
| `GET /api/game/skills`        | Per-skill XP and tier projections.                                  |
| `GET /api/game/events`        | Recent XP ledger feed.                                              |
| `GET /api/projects`           | Plan project vocabulary for filters and task creation.              |
| `GET /api/categories`         | Plan quick-log buttons.                                             |
| `GET /api/plan`               | Current plan identity, counts, and latest stored uploaded source.   |
| `POST /api/plan/preview`      | Parse uploaded markdown and return a diff; changes no rows.         |
| `POST /api/plan/apply`        | Apply a clean preview. Replace mode writes a backup first.          |
| `GET /api/skills`             | Skill profile with derived stats, plus tasks still missing a skill. |
| `GET /api/skills/{id}`        | One skill: description, tagging rule, its tasks, its evidence.      |
| `GET /api/reviews`            | Daily reviews in a date range.                                      |
| `POST /api/reviews`           | Upsert by date.                                                     |
| `GET /api/metrics`            | Time series, filterable by name.                                    |
| `POST /api/metrics`           | Record a reading.                                                   |
| `GET /api/metric-defs`        | The metric-name catalogue, including Review help text.              |
| `GET /api/checkpoints/{week}` | Checkpoint questions, helpers, and answers.                         |
| `PUT /api/checkpoints/{week}` | Save checkpoint answers.                                            |
| `GET /api/export`             | Full JSON dump of every table.                                      |

---

## 3. Endpoints

### `GET /api/health`

```json
{ "status": "ok", "version": "0.1.0-3f2a1c9" }
```

---

### `POST /api/pair`

Mints a one-use pairing code for the route currently open on the desktop. Requires `X-Token` if
the server was started with `-token`.

```json
{ "route": "/goals?project=synapse" }
```

Response:

```json
{
  "url": "http://dash.tailnet.ts.net:8484/pair/7f3a9c2e1b",
  "svg_url": "/api/pair/7f3a9c2e1b.svg",
  "expires_at": "2026-08-22T22:41:30Z",
  "expires_in_seconds": 90
}
```

The URL host comes from `-public-url`, or from `-addr` when `-public-url` is unset.

### `GET /api/pair/{code}.svg`

Returns an SVG QR code for the pairing URL. This endpoint is under `/api/`, so it requires the
same `X-Token` as other API endpoints when a token is configured. The code is redacted from
request logs.

### `GET /pair/{code}`

Redeems the code, deletes it, and serves an HTML bootstrap that stores the token in
`localStorage["dashboard.token"]` before navigating to the minted route. This endpoint is
exempt from the token check because the phone does not have the token before redemption.

Unknown, expired, already-used, and rate-limited codes all produce the same invalid-code page.

---

### `GET /api/plan`

Returns the current loaded plan plus the newest stored source, if it was loaded through the UI.

```json
{
  "plan": { "id": "master-plan-v5", "name": "Master Plan v5", "loaded_at": "2026-08-22T19:10:00Z" },
  "source": null,
  "counts": { "weeks": 19, "goals": 18, "tasks": 66, "skills": 17, "projects": 8 }
}
```

### `POST /api/plan/preview`

Accepts `text/markdown`, `text/plain`, or multipart file fields named `file` or `plan`.
Maximum body size is 1 MB. Invalid UTF-8 or NUL bytes are rejected.

Preview changes no rows. Malformed markdown returns `200` with `errors` because authoring
mistakes are expected outcomes:

```json
{
  "errors": [
    { "line": 143, "message": "line 143: unrecognised line in week W4", "excerpt": "- [x] note" }
  ]
}
```

A clean preview returns `source_sha256`, `mode` (`initial`, `additive`, `replace`), parsed
counts, diff counts, and replace risk counts.

### `POST /api/plan/apply`

```json
{
  "source": "# My Plan\n...",
  "source_sha256": "9f2c...",
  "confirm_plan_name": "My Plan"
}
```

`source_sha256` must match a clean preview from this server process. Replace mode also requires
`confirm_plan_name` to equal the new plan name. Hash mismatch or no preview is `409`;
missing/wrong confirmation is `422`.

On replace, the server writes `backups/pre-plan-<planid>-<timestamp>.db` before changing rows.
Logs, daily reviews, and metrics are preserved.

---

### `GET /api/dashboard`

One request per app open. Everything the Today screen needs.

**Query:** `project` — optional CSV filter applied to `tasks` only (`?project=synapse,gateway`).
Every listed project must exist in `GET /api/projects`.

```json
{
  "today": "2026-09-24",
  "task_week": "W5",
  "week": {
    "code": "W5",
    "phase": "P1",
    "focus": "Chaos: fail-open / fail-static",
    "start_date": "2026-09-21",
    "end_date": "2026-09-27",
    "state": "active"
  },
  "rhythm": {
    "label": "Thu",
    "slot": "SAA prep (until exam) → then Udemy LLM (2h) + 30 min community"
  },
  "tasks": [
    {
      "id": 41,
      "week": "W5",
      "title": "Fail-open on provider timeout",
      "project": "gateway",
      "goal_id": 3,
      "steps": [
        "Add a 2s context deadline per provider call.",
        "On deadline, return the cached summary."
      ],
      "done_means": "a killed provider does not surface an error to the caller",
      "status": "todo",
      "done_at": null,
      "sort_order": 100,
      "version": 1
    }
  ],
  "completion": { "done": 2, "total": 5 },
  "counters": [
    {
      "category_id": "application",
      "label": "Application",
      "icon": "📮",
      "count": 3
    },
    {
      "category_id": "module",
      "label": "Course module",
      "icon": "📚",
      "count": 5
    }
  ],
  "streak": { "days": 12, "counts_today": false }
}
```

`week.state` is one of:

| State           | When                            | `code`, `focus`, dates |
| --------------- | ------------------------------- | ---------------------- |
| `not_started`   | before `W1` starts (2026-08-24) | `null`                 |
| `active`        | inside a `W`/`B` window         | populated              |
| `plan_complete` | after `B7` ends (2027-02-21)    | `null`                 |

The screen stays fully usable in every state; only the banner disappears. In `plan_complete`,
`tasks` carries the last block's list.

**`task_week` is the week `tasks` actually came from**, and it is set in every state: the active
week when there is one, the first week before the plan starts, the last week after it ends. It
is `null` only when the plan has no weeks at all.

Clients that need a week to *write* to — the add-task sheet — must use `task_week`, not
`week.code`. Using `week.code` leaves the UI unable to add a task outside the plan window, which
is precisely when the screen is still showing a perfectly good task list.

`counters` covers the current **Lisbon** week (Monday-based). `streak.counts_today` is false
until today has activity — the chip renders dimmed, not zero.

---

### `GET /api/goals`

**Query:** `status` (`backlog|active|done`), `project` (CSV). Both optional.

```json
{
  "goals": [
    {
      "id": 8,
      "code": "G7",
      "title": "OSS presence",
      "done_means": "2 PRs merged",
      "project": "oss",
      "phase": "P1",
      "status": "active",
      "target": "W9 / P2",
      "sort_order": 200,
      "log_count": 4,
      "version": 3,
      "created_at": "2026-08-24T09:00:00Z",
      "completed_at": null
    }
  ],
  "active_count": 4,
  "active_limit": 3
}
```

`log_count` is the "proof of motion" chip. `active_limit` is loaded from the plan config.
`active_count` > `active_limit` drives the warning badge — the API never blocks the move; see
ADR-004's rule that handlers hold no policy beyond validation.

### `POST /api/goals`

```json
{
  "title": "Ship the eval CI gate",
  "project": "synapse",
  "done_means": "CI fails on regression",
  "target": "W6",
  "status": "backlog"
}
```

`201`, `Location: /api/goals/19`, body is the created goal. `title` and `project` required;
`project` must exist in the loaded plan's project vocabulary. `status` defaults to `backlog`.
`code` cannot be set — it belongs to seeded goals only.

### `PATCH /api/goals/{id}`

Requires `If-Match`. All fields optional; send only what changes.

```json
{ "status": "active", "position": 1 }
```

`position` is the **0-based index in the target column, counted with the moved card removed** —
the same index the client computes when it drops a card onto another one. It is not a raw
`sort_order`. `status` and `position` must be sent together.

The server renumbers the affected columns in sparse steps of 100 inside one transaction and
returns **every card whose `sort_order` changed**, so the client never computes ordering:

```json
{
  "goal": {
    "id": 8,
    "status": "active",
    "sort_order": 200,
    "version": 4,
    "completed_at": null
  },
  "reordered": [
    { "id": 3, "status": "active", "sort_order": 100, "version": 7 },
    { "id": 8, "status": "active", "sort_order": 200, "version": 4 },
    { "id": 12, "status": "active", "sort_order": 300, "version": 2 },
    { "id": 5, "status": "backlog", "sort_order": 100, "version": 1 }
  ]
}
```

`reordered` covers **both** columns when the move crosses one: the target column, and the
column the card left, whose remaining cards close the gap. Each card appears at most once, and
the moved card is present in both `goal` and `reordered`. A client that applies `reordered`
wholesale therefore needs no follow-up read.

**Versions.** A move bumps the version of the moved card **only**, by one. Cards that were
merely renumbered keep their versions: `version` guards user edits, and being pushed down a
column because a neighbour moved is not an edit. Bumping them would invalidate the ETag of
every card in the column, so an unrelated goal would fail its next `If-Match` for no reason.

`If-Match` applies to the moved goal only.

Moving into `done` stamps `completed_at`; moving out clears it.

### `DELETE /api/goals/{id}`

`204`. Tasks and log entries that referenced it survive with `goal_id: null` — history is never
deleted as a side effect (DATABASE.md §4.7, §4.8).

---

### `GET /api/tasks`

**Query:** `week` (e.g. `W5`), `project` (CSV). Both optional; omitting `week` returns every
task, ordered by week then `sort_order`.

```json
{
  "tasks": [
    {
      "id": 41,
      "week": "W5",
      "title": "Fail-open on provider timeout",
      "project": "gateway",
      "goal_id": 3,
      "steps": ["Add a 2s context deadline per provider call."],
      "done_means": "a killed provider does not surface an error to the caller",
      "status": "todo",
      "done_at": null,
      "skill_ids": [3],
      "sort_order": 100,
      "version": 1
    }
  ]
}
```

### `POST /api/tasks`

**`skill_ids` is required and must be non-empty**: every task builds at least one skill
(DATABASE.md §4.7). An empty or absent list is **`422 Unprocessable Content`** — the request is
well-formed, the model forbids it. An id that matches no skill is `400`, because that is a
malformed reference rather than a policy refusal.

```json
{
  "week": "W5",
  "title": "Re-run k6 after the timeout change",
  "project": "gateway",
  "goal_id": 3,
  "skill_ids": [3]
}
```

`201` + the created task. `week`, `title`, `project` required. `week` must exist in `weeks`.
`project` must exist in the loaded plan's project vocabulary. `steps` defaults to `[]`,
`seed_key` is always null for API-created tasks.

### `PATCH /api/tasks/{id}`

`skill_ids` is optional here: **absent leaves the links alone**, present replaces them
wholesale. Present-but-empty is `422`, for the same reason as on create.

Requires `If-Match`.

```json
{ "status": "done" }
```

```json
{ "id": 41, "status": "done", "done_at": "2026-09-24T18:22:11Z", "version": 2 }
```

`done_at` is stamped by the server, never accepted from the client. Toggling back to `todo`
clears it.

### `DELETE /api/tasks/{id}`

`204`. Permitted for any task, seeded or not — but deleting a seeded task means the next boot
re-inserts it, because its `seed_key` no longer matches an existing row (DATABASE.md §8).

---

### `GET /api/logs`

**Query:** `category` (CSV of category ids), `from` / `to` (`YYYY-MM-DD`, Lisbon, inclusive),
`limit` (default `200`, max `500`), `cursor`.

```json
{
  "entries": [
    {
      "id": 310,
      "category_id": "application",
      "category_label": "Application",
      "icon": "📮",
      "title": "Applied — Platform Engineer @ Acme",
      "note": null,
      "url": "https://acme.example/careers/1234",
      "goal_id": 10,
      "goal_code": "G9",
      "occurred_at": "2026-09-24T17:40:00Z",
      "created_at": "2026-09-24T17:40:02Z"
    }
  ],
  "next_cursor": "2026-09-21T08:15:00Z"
}
```

Newest first. `next_cursor` is the `occurred_at` of the last row returned, or `null` when the
feed is exhausted. Pass it back as `cursor` for the next page.

Day grouping is the **client's** job, using the Lisbon date of `occurred_at`.

### `POST /api/logs`

The + button. This is the path optimised for `<5 taps`.

```json
{
  "category_id": "application",
  "title": "Applied — Platform Engineer @ Acme",
  "url": "https://acme.example/careers/1234",
  "goal_id": 10
}
```

`201` + the created entry. `category_id` and `title` required. `occurred_at` defaults to now
and may be backdated. `note` and `url` are independent optional fields; the quick-log sheet's
single input routes to `url` when the value parses as `http(s)://…`, otherwise to `note`.

`skill_ids` is optional on `POST /api/logs` — evidence links are a nice-to-have and the
quick-log path is the app's most-used action, so it must not grow a required choice.

### `DELETE /api/logs/{id}`

`204`. The only mutation the log supports — entries are never editable (v2 §1). Undo is a
client-side hold before the request fires, not a server concept.

### `GET /api/logs/summary`

**Query:** `range` — `week` (current Lisbon week, Monday-based) or `all`. Default `week`.

```json
{
  "range": "week",
  "from": "2026-09-21",
  "to": "2026-09-27",
  "counts": [
    {
      "category_id": "application",
      "label": "Application",
      "icon": "📮",
      "count": 2
    },
    {
      "category_id": "module",
      "label": "Course module",
      "icon": "📚",
      "count": 4
    },
    {
      "category_id": "number",
      "label": "Benchmark/number",
      "icon": "📊",
      "count": 1
    }
  ],
  "total": 7
}
```

Categories with a zero count are included, so the recap card has a stable shape.

---

### `GET /api/game/profile`

Derived player state from `xp_events` plus unlocked badges.

```json
{
  "total_xp": 340,
  "level": 3,
  "title": "Backend Engineer",
  "current_threshold": 300,
  "next_level": 4,
  "next_threshold": 500,
  "progress_xp": 40,
  "progress_required": 200,
  "streak": {
    "days": 6,
    "counts_today": true,
    "shielded": false
  },
  "badges": [
    {
      "id": 3,
      "code": "gatekeeper",
      "name": "Gatekeeper",
      "description": "Complete the W3 regression-gate-in-ci task.",
      "sort_order": 30,
      "unlocked_at": "2026-09-24T17:40:00Z"
    }
  ]
}
```

Level thresholds are `25 * n * (n + 1)`. The streak is based on days with at least one XP
event in `Europe/Lisbon`; zero-XP Sundays set `shielded: true` and do not break the streak.

### `GET /api/game/skills`

Per-skill XP and M9 tier projection.

```json
{
  "skills": [
    {
      "skill_id": 4,
      "code": "postgres",
      "name": "Postgres internals",
      "target_tier": "Expert",
      "xp": 340,
      "tier": "Adept",
      "tier_min": 300,
      "next_tier_xp": 500,
      "next_tier": "Expert"
    }
  ]
}
```

Skill XP mirrors the full event amount to each linked skill. It is derived from
`xp_event_skills`, not stored on `skills`.

### `GET /api/game/events`

**Query:** `limit` (default `20`, max `100`).

```json
{
  "events": [
    {
      "id": 12,
      "source_type": "log",
      "source_id": 310,
      "amount": 30,
      "created_at": "2026-09-24T17:40:02Z",
      "skill_ids": [4, 8],
      "skill_codes": ["postgres", "chaos"],
      "skill_names": ["Postgres internals", "Load & chaos testing"],
      "source_label": "Benchmark/number - k6 baseline"
    }
  ]
}
```

Newest first. `source_label` is display help only; the ledger identity is
`source_type + source_id`.

---

### `GET /api/categories`

```json
{
  "categories": [
    {
      "id": "application",
      "label": "Application",
      "icon": "📮",
      "sort_order": 1
    },
    { "id": "module", "label": "Course module", "icon": "📚", "sort_order": 2 }
  ]
}
```

---

### `GET /api/projects`

```json
{
  "projects": [
    { "id": "synapse", "label": "Synapse", "sort_order": 10 },
    { "id": "gateway", "label": "LLM Gateway", "sort_order": 20 }
  ]
}
```

The response is the loaded plan's project vocabulary in picker order.

---

### `GET /api/skills`

```json
{
  "skills": [
    {
      "id": 1,
      "code": "evals",
      "name": "Evaluation systems",
      "description": "Measuring LLM output quality: golden sets, rejection metrics, regression gates.",
      "associate_when": "the task creates, runs, or gates on an evaluation.",
      "target_tier": "Expert",
      "sort_order": 10,
      "task_count": 8,
      "task_done": 3,
      "evidence_count": 2,
      "last_activity": "2026-09-24T17:40:00Z"
    }
  ],
  "unlinked_tasks": []
}
```

`task_count`, `task_done`, `evidence_count` and `last_activity` are **derived from the
association tables on every read** and stored nowhere (ADR-007), so they cannot disagree with
the task list they summarise. `last_activity` is the later of the newest linked `tasks.done_at`
and the newest citing `log_entries.occurred_at`, or `null` if neither exists.

`unlinked_tasks` lists task ids with no skill at all — rows created before skills existed. The
UI shows them a "needs skill" badge rather than implying the invariant already holds.

### `GET /api/skills/{id}`

The skill, plus `tasks` (every task that builds it, ordered by week) and `evidence` (every log
entry citing it, newest first). Same shape as above with those two arrays added.

### `GET /api/reviews`

**Query:** `from` / `to` (`YYYY-MM-DD`, inclusive). Omitted → the last 30 days.

```json
{
  "reviews": [
    {
      "date": "2026-09-24",
      "learned": "WAL checkpointing blocks on a long read txn",
      "issue": "k6 run was CPU-bound on the laptop",
      "next": "move load gen to the other machine",
      "minutes": 45,
      "created_at": "2026-09-24T20:10:00Z",
      "updated_at": "2026-09-24T20:10:00Z"
    }
  ]
}
```

### `POST /api/reviews`

Upsert keyed by `date`.

```json
{
  "date": "2026-09-20",
  "learned": "…",
  "issue": "…",
  "next": "…",
  "minutes": 30
}
```

`200` (never `201` — the date is the key and may already exist) + the stored review.

`date` is optional and defaults to today in Lisbon. It may be **backdated**: the Sunday ritual
often happens on Monday, and without backfill those reviews land on the wrong day and break the
streak they exist to prove. Future dates are rejected with `400`.

At least one of `learned`, `issue`, `next` must be non-blank (DATABASE.md §4.9).

---

### `GET /api/metrics`

**Query:** `name` (exact), `from` / `to`, `limit` (default `100`).

```json
{
  "metrics": [
    {
      "id": 22,
      "name": "p95 latency (cached)",
      "value": 0.74,
      "unit": "s",
      "note": "after cache warm",
      "recorded_at": "2026-10-09T19:02:00Z"
    }
  ]
}
```

### `POST /api/metrics`

```json
{
  "name": "p95 latency (cached)",
  "value": 0.74,
  "unit": "s",
  "note": "after cache warm"
}
```

`201` + the created reading. `name` and `value` required. `recorded_at` defaults to now.

`name` is deliberately free text and not validated against `metric_defs` — the Review screen's
"other…" field must accept a new metric without a migration (DATABASE.md §4.10). Spelling
consistency is the picker's job, not the API's.

### `GET /api/metric-defs`

```json
{
  "metric_defs": [
    {
      "name": "p95 latency (cached)",
      "slug": "p95_latency",
      "unit": "s",
      "baseline": ">1s",
      "target": "<0.8s",
      "definition": "95th-percentile request latency under the standard mixed profile",
      "how_to_measure": "Grafana after a 10-min k6 run; never compare across different profiles",
      "sort_order": 2
    },
    {
      "name": "Cache hit rate",
      "slug": "cache_hit_rate",
      "unit": "%",
      "baseline": "<20%",
      "target": ">60%",
      "definition": "hits ÷ (hits + misses)",
      "how_to_measure": "Gateway cache metric over a 10-min replayed-traffic window",
      "sort_order": 4
    }
  ]
}
```

`definition` and `how_to_measure` drive the Review metric picker help. They come from seeded
reference data, not frontend copy, so a new catalogue metric carries its own guidance.

---

### `GET /api/checkpoints/{week}`

`week` is `W12` or `B7`. `404` for any other week.

```json
{
  "week": "W12",
  "questions": [
    "interview-grade eval number?",
    "chaos falsified anything?",
    "PR merged or stale?",
    "fourth repo?",
    "still AI-infra path?"
  ],
  "helpers": [
    "Good = the exact sentence you'd say out loud, with real X and Y. Not yet = name the one missing piece.",
    "Name the belief that died. Nothing surprised you = you tested too gently — schedule the harder rerun before answering.",
    "Status + date of last maintainer contact + your next move.",
    "Yes/no. If yes: which one gets archived this week.",
    "Gut check, one paragraph max. Re-decide, don't re-litigate."
  ],
  "answers": null,
  "completed_at": null
}
```

`helpers` is index-aligned with `questions`; if it is present, it has the same length.

### `PUT /api/checkpoints/{week}`

```json
{
  "answers": [
    "0.31 rejection rate, CI-gated",
    "yes — fail-static was wrong",
    "merged",
    "not started",
    "yes"
  ]
}
```

`200` + the stored checkpoint. `answers` must have exactly the same length as `questions`;
any other length is `400`. Empty strings are allowed — a blank answer is an answer. The first
save stamps `completed_at`; later saves update the answers and leave it.

`PUT`, not `PATCH`: the answer array is replaced whole, so there is no version to conflict on.

---

### `GET /api/export`

```
Content-Type: application/json
Content-Disposition: attachment; filename="dashboard-export-20260924.json"
```

```json
{
  "exported_at": "2026-09-24T21:00:00Z",
  "schema_version": 6,
  "plan_meta": [],
  "plan_config": [],
  "projects": [],
  "phases": [],
  "skill_tiers": [],
  "weeks": [],
  "rhythm": [],
  "categories": [],
  "metric_defs": [],
  "goals": [],
  "tasks": [],
  "log_entries": [],
  "daily_reviews": [],
  "metrics": [],
  "checkpoints": [],
  "skills": [],
  "task_skills": [],
  "log_skills": [],
  "xp_rules": [],
  "xp_events": [],
  "xp_event_skills": [],
  "achievements": [],
  "achievement_unlocks": []
}
```

Every current table from DATABASE.md §4, complete and unpaginated. This is the insurance
policy; restore is manual and out of scope for v1.

---

## 4. Middleware chain

Outermost first. These are the only decorators in the system (ADR-004).

1. **recover** — converts a panic into `500` + one `ERROR` log line with the stack.
2. **request log** — one line per call at `INFO`: method, path, status, duration, bytes.
3. **token** — the `X-Token` check, skipped for `GET /api/health`.
4. **mux** — routing and the handler.

Errors are logged **exactly once**, by the API layer's respond-error helper, with the full
wrapped `%w` chain. Nothing below `internal/api` logs at all.

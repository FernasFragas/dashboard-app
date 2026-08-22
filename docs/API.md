# API

JSON over HTTP, served by stdlib `net/http` ServeMux (ADR-005). Everything lives under
`/api/`; any other path serves the embedded SPA.

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
- **Trailing slashes** are not accepted.

### Authentication

If the server was started with `-token`, every request under `/api/` must carry a matching
`X-Token` header. Missing or wrong → `401`. `GET /api/health` is exempt so a liveness probe
works without the secret.

The real perimeter is the tailnet ACL (ADR-003); the token is a second lock, not the lock.

### Error envelope

Every non-2xx response body:

```json
{ "error": "title is required" }
```

`412` additionally carries the current server state so the client can reconcile without a
second round trip:

```json
{ "error": "task was modified elsewhere", "current": { "id": 42, "version": 5, "status": "done" } }
```

### Status codes

| Code | Meaning |
|---|---|
| `200` | OK, body follows. |
| `201` | Created; body is the created resource; `Location` header set. |
| `204` | Deleted, no body. |
| `400` | Validation failure — blank title, bad enum, unknown field or query param, malformed JSON. |
| `401` | `-token` is set and `X-Token` is missing or wrong. |
| `404` | No such id. |
| `409` | Unique constraint — duplicate `goals.code`. |
| `412` | `If-Match` version is stale. Body includes `current`. |
| `428` | `PATCH` on a versioned resource without an `If-Match` header. |
| `500` | Anything unhandled. Logged once, at the boundary, with the wrapped cause. The client sees a generic message. |

### Optimistic concurrency

Applies to **`goals` and `tasks` only** (see DATABASE.md §6).

- Every representation of a goal or task includes its `version`.
- Single-resource responses set `ETag: W/"<id>-<version>"`.
- `PATCH` **requires** `If-Match: W/"<id>-<version>"`. Absent → `428`. Stale → `412`.
- A successful `PATCH` bumps `version` and returns the new `ETag`.

---

## 2. Endpoint summary

| Method + path | Purpose |
|---|---|
| `GET /api/health` | Liveness. Exempt from the token check. |
| `GET /api/dashboard` | Today bundle: plan week, rhythm slot, week tasks, counters, streak. |
| `GET /api/goals` | List, optionally filtered by status or project. |
| `POST /api/goals` | Create. |
| `PATCH /api/goals/{id}` | Move column / reorder / edit. Requires `If-Match`. |
| `DELETE /api/goals/{id}` | Delete. Linked tasks and logs survive with `goal_id` nulled. |
| `GET /api/tasks` | Week list, filterable by week and project. |
| `POST /api/tasks` | Add an ad-hoc task. |
| `PATCH /api/tasks/{id}` | Toggle done / edit. Requires `If-Match`. |
| `DELETE /api/tasks/{id}` | Delete a hand-created task. |
| `GET /api/logs` | The accomplishment feed, paginated. |
| `POST /api/logs` | The + button. |
| `DELETE /api/logs/{id}` | Remove a typo entry. |
| `GET /api/logs/summary` | Counters per category. |
| `GET /api/categories` | The eight quick-log buttons. |
| `GET /api/reviews` | Daily reviews in a date range. |
| `POST /api/reviews` | Upsert by date. |
| `GET /api/metrics` | Time series, filterable by name. |
| `POST /api/metrics` | Record a reading. |
| `GET /api/metric-defs` | The metric-name catalogue for the picker. |
| `GET /api/checkpoints/{week}` | Checkpoint questions and answers. |
| `PUT /api/checkpoints/{week}` | Save checkpoint answers. |
| `GET /api/export` | Full JSON dump of every table. |

---

## 3. Endpoints

### `GET /api/health`

```json
{ "status": "ok", "version": "0.1.0-3f2a1c9" }
```

---

### `GET /api/dashboard`

One request per app open. Everything the Today screen needs.

**Query:** `project` — optional CSV filter applied to `tasks` only (`?project=synapse,gateway`).

```json
{
  "today": "2026-09-24",
  "week": {
    "code": "W5",
    "phase": "P1",
    "focus": "Chaos: fail-open / fail-static",
    "start_date": "2026-09-21",
    "end_date": "2026-09-27",
    "state": "active"
  },
  "rhythm": { "label": "Thu", "slot": "SAA prep (until exam) → then Udemy LLM (2h) + 30 min community" },
  "tasks": [
    {
      "id": 41, "week": "W5", "title": "Fail-open on provider timeout",
      "project": "gateway", "goal_id": 3,
      "steps": ["Add a 2s context deadline per provider call.", "On deadline, return the cached summary."],
      "done_means": "a killed provider does not surface an error to the caller",
      "status": "todo", "done_at": null, "sort_order": 100, "version": 1
    }
  ],
  "completion": { "done": 2, "total": 5 },
  "counters": [
    { "category_id": "application", "label": "Application", "icon": "📮", "count": 3 },
    { "category_id": "module", "label": "Course module", "icon": "📚", "count": 5 }
  ],
  "streak": { "days": 12, "counts_today": false }
}
```

`week.state` is one of:

| State | When | `code`, `focus`, dates |
|---|---|---|
| `not_started` | before `W1` starts (2026-08-24) | `null` |
| `active` | inside a `W`/`B` window | populated |
| `plan_complete` | after `B7` ends (2027-02-21) | `null` |

The screen stays fully usable in every state; only the banner disappears. In `plan_complete`,
`tasks` carries the last block's list.

`counters` covers the current **Lisbon** week (Monday-based). `streak.counts_today` is false
until today has activity — the chip renders dimmed, not zero.

---

### `GET /api/goals`

**Query:** `status` (`backlog|active|done`), `project` (CSV). Both optional.

```json
{
  "goals": [
    {
      "id": 8, "code": "G7", "title": "OSS presence",
      "done_means": "2 PRs merged", "project": "oss", "phase": "P1",
      "status": "active", "target": "W9 / P2", "sort_order": 200,
      "log_count": 4,
      "version": 3,
      "created_at": "2026-08-24T09:00:00Z", "completed_at": null
    }
  ],
  "active_count": 4,
  "active_limit": 3
}
```

`log_count` is the "proof of motion" chip. `active_count` > `active_limit` drives the warning
badge — the API never blocks the move; see ADR-004's rule that handlers hold no policy beyond
validation.

### `POST /api/goals`

```json
{ "title": "Ship the eval CI gate", "project": "synapse", "done_means": "CI fails on regression", "target": "W6", "status": "backlog" }
```

`201`, `Location: /api/goals/19`, body is the created goal. `title` and `project` required;
`status` defaults to `backlog`. `code` cannot be set — it belongs to seeded goals only.

### `PATCH /api/goals/{id}`

Requires `If-Match`. All fields optional; send only what changes.

```json
{ "status": "active", "position": 1 }
```

`position` is the **0-based index in the target column**, not a raw `sort_order`. The server
renumbers that column in sparse steps of 100 inside one transaction and returns every affected
card, so the client never computes ordering:

```json
{
  "goal": { "id": 8, "status": "active", "sort_order": 200, "version": 4, "completed_at": null },
  "reordered": [
    { "id": 3, "status": "active", "sort_order": 100, "version": 7 },
    { "id": 8, "status": "active", "sort_order": 200, "version": 4 },
    { "id": 12, "status": "active", "sort_order": 300, "version": 2 }
  ]
}
```

`If-Match` applies to the moved goal only; the renumbered siblings are server-driven and their
versions are bumped without a precondition.

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
      "id": 41, "week": "W5", "title": "Fail-open on provider timeout",
      "project": "gateway", "goal_id": 3,
      "steps": ["Add a 2s context deadline per provider call."],
      "done_means": "a killed provider does not surface an error to the caller",
      "status": "todo", "done_at": null, "sort_order": 100, "version": 1
    }
  ]
}
```

### `POST /api/tasks`

```json
{ "week": "W5", "title": "Re-run k6 after the timeout change", "project": "gateway", "goal_id": 3 }
```

`201` + the created task. `week`, `title`, `project` required. `week` must exist in `weeks`.
`steps` defaults to `[]`, `seed_key` is always null for API-created tasks.

### `PATCH /api/tasks/{id}`

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
      "id": 310, "category_id": "application", "category_label": "Application", "icon": "📮",
      "title": "Applied — Platform Engineer @ Acme",
      "note": null, "url": "https://acme.example/careers/1234",
      "goal_id": 10, "goal_code": "G9",
      "occurred_at": "2026-09-24T17:40:00Z", "created_at": "2026-09-24T17:40:02Z"
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

### `DELETE /api/logs/{id}`

`204`. The only mutation the log supports — entries are never editable (v2 §1). Undo is a
client-side hold before the request fires, not a server concept.

### `GET /api/logs/summary`

**Query:** `range` — `week` (current Lisbon week, Monday-based) or `all`. Default `week`.

```json
{
  "range": "week",
  "from": "2026-09-21", "to": "2026-09-27",
  "counts": [
    { "category_id": "application", "label": "Application", "icon": "📮", "count": 2 },
    { "category_id": "module", "label": "Course module", "icon": "📚", "count": 4 },
    { "category_id": "number", "label": "Benchmark/number", "icon": "📊", "count": 1 }
  ],
  "total": 7
}
```

Categories with a zero count are included, so the recap card has a stable shape.

---

### `GET /api/categories`

```json
{
  "categories": [
    { "id": "application", "label": "Application", "icon": "📮", "sort_order": 1 },
    { "id": "module", "label": "Course module", "icon": "📚", "sort_order": 2 }
  ]
}
```

---

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
      "created_at": "2026-09-24T20:10:00Z", "updated_at": "2026-09-24T20:10:00Z"
    }
  ]
}
```

### `POST /api/reviews`

Upsert keyed by `date`.

```json
{ "date": "2026-09-20", "learned": "…", "issue": "…", "next": "…", "minutes": 30 }
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
    { "id": 22, "name": "p95 latency (cached)", "value": 0.74, "unit": "s", "note": "after cache warm", "recorded_at": "2026-10-09T19:02:00Z" }
  ]
}
```

### `POST /api/metrics`

```json
{ "name": "p95 latency (cached)", "value": 0.74, "unit": "s", "note": "after cache warm" }
```

`201` + the created reading. `name` and `value` required. `recorded_at` defaults to now.

`name` is deliberately free text and not validated against `metric_defs` — the Review screen's
"other…" field must accept a new metric without a migration (DATABASE.md §4.10). Spelling
consistency is the picker's job, not the API's.

### `GET /api/metric-defs`

```json
{
  "metric_defs": [
    { "name": "p95 latency (cached)", "unit": "s", "baseline": ">1s", "target": "<0.8s", "sort_order": 2 },
    { "name": "Cache hit rate", "unit": "%", "baseline": "<20%", "target": ">60%", "sort_order": 4 }
  ]
}
```

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
  "answers": null,
  "completed_at": null
}
```

### `PUT /api/checkpoints/{week}`

```json
{ "answers": ["0.31 rejection rate, CI-gated", "yes — fail-static was wrong", "merged", "not started", "yes"] }
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
  "schema_version": 1,
  "weeks": [], "rhythm": [], "categories": [], "metric_defs": [],
  "goals": [], "tasks": [], "log_entries": [], "daily_reviews": [],
  "metrics": [], "checkpoints": []
}
```

Every table from DATABASE.md §4, complete and unpaginated. This is the insurance policy;
restore is manual and out of scope for v1.

---

## 4. Middleware chain

Outermost first. These are the only decorators in the system (ADR-004).

1. **recover** — converts a panic into `500` + one `ERROR` log line with the stack.
2. **request log** — one line per call at `INFO`: method, path, status, duration, bytes.
3. **token** — the `X-Token` check, skipped for `GET /api/health`.
4. **mux** — routing and the handler.

Errors are logged **exactly once**, by the API layer's respond-error helper, with the full
wrapped `%w` chain. Nothing below `internal/api` logs at all.

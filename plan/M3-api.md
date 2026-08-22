# M3 · API

**Depends on:** M2. **Unblocks:** M4, M5, M6.
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §2

Every endpoint in `docs/API.md`, built on stdlib `http.ServeMux` (ADR-005).

## Middleware chain (outermost → innermost)

`recover` → `request log` (slog: method, path, status, duration, bytes) → `token` → mux.
Per ADR-004 these are the **only** decorators in the system.

Token: if `-token` is set, every `/api/*` request must carry a matching `X-Token` header;
missing or wrong → `401` with the error envelope. `GET /api/health` is exempt so a liveness
probe works without the secret.

## Error envelope

`{"error": "message"}` with the proper status. Validation lives in the handlers: title
required and non-blank, enum values checked against the CHECK sets, unknown query params
rejected rather than ignored. Store sentinels map to status:

| Store error | Status |
|---|---|
| `ErrNotFound` | 404 |
| `ErrConflict` (unique) | 409 |
| `ErrConflict` (stale version) | 412 |
| validation failure | 400 |
| anything else | 500, logged once at the boundary with the wrapped `%w` chain |

## Optimistic concurrency — ETag / If-Match

Applies to **`goals` and `tasks` only** (the mutable resources; logs/reviews/metrics don't need it).

- `GET` of a goal or task, and every goal/task inside a list or the dashboard bundle, carries
  its `version`; single-resource `GET` also sets `ETag: W/"<id>-<version>"`.
- `PATCH` **must** send `If-Match`. Missing header → `428 Precondition Required`.
  Stale version → `412 Precondition Failed`, body includes the current resource so the client
  can reconcile without a second round trip.
- Successful `PATCH` bumps `version` and returns the new `ETag`.

> **Trade accepted:** this is real plumbing for a single-user, single-writer app — the only
> conflict it can catch is phone and laptop open at the same time. Chosen deliberately; the
> cost lands in M4/M5, where every optimistic mutation must handle a 412 rollback.

## Plan-week computation

Pure function, no DB, unit-tested. Windows come from `master-plan-v5.md`:

- `W1`…`W12`: Aug 24 2026 → Nov 15 2026, 7-day windows.
- `B1`…`B7`: Nov 16 2026 → Feb 21 2027, 14-day windows.
- Before Aug 24 → `{"week": null, "state": "not_started"}`.
- After Feb 21 2027 → `{"week": null, "state": "plan_complete"}` — the app stays fully usable;
  Today shows the task list for the last block and no week banner.

Boundaries are evaluated in **`Europe/Lisbon`**, not UTC.

## Streak computation

Pure function over log-entry and daily-review timestamps, converted to Lisbon days.

- A day counts if it has **≥1 log entry OR a daily review**. Completed tasks do **not** count —
  the Log screen exists precisely because checkboxes are the weaker signal.
- The streak is the run of consecutive counting days ending at **today or yesterday**. An
  empty today does not zero it; two empty days do.
- Response includes `{"streak": n, "counts_today": bool}` so the UI can show the chip dimmed
  until today is earned.

## Endpoints

As the table in `dashboard-plan-v2.md` §1 / `docs/API.md`. Refinements:

- `GET /api/dashboard` — one bundle: plan week + state, today's rhythm slot, this week's tasks
  (with `steps` and `done_means`), weekly category counters, streak. One request per app open.
- `GET /api/logs` — `?category=&from=&to=&limit=&cursor=`. Default `limit=200`, newest first,
  cursor = `occurred_at` of the last row. Not a scale concern; it keeps the feed from growing
  unbounded on a phone.
- `GET /api/export` — full JSON dump of every table, `Content-Disposition: attachment`,
  filename `dashboard-export-YYYYMMDD.json`.
- `PATCH /api/goals/{id}` — a Kanban move sends `{"status": "active", "position": 2}` where
  `position` is the **0-based index in the target column**, not a raw `sort_order`. The server
  renumbers that column in sparse steps of 100 inside **one transaction** and returns every
  affected card. `If-Match` applies to the moved card only. Rationale and card layout: M5.
  Moving to `done` stamps `completed_at`; moving out of `done` clears it.

## Tests — httptest suite **and** README curl checklist

1. **httptest suite** (`internal/api`): full mux + a real temp SQLite file. Per endpoint assert
   status, body shape, and the error envelope. Explicitly cover: missing/blank title → 400,
   bad enum → 400, unknown id → 404, `PATCH` without `If-Match` → 428, stale `If-Match` → 412,
   wrong `X-Token` → 401, health without token → 200.
2. **Plan-week table test**: Aug 23 / Aug 24 / Nov 15 / Nov 16 / Feb 21 / Feb 22, plus a
   23:30-Lisbon case that would fall on the wrong day in UTC.
3. **Streak table test**: empty today with yesterday active → streak holds; two empty days →
   streak breaks.
4. **README curl checklist**: create goal → move it → add task → toggle → log entry → summary.
   Kept as documentation for future-you; the httptest suite is what CI enforces.

## Done when

1. `go test ./internal/api` green, including all the error cases above.
2. The README curl checklist passes end to end against a fresh binary.
3. `make check` green.

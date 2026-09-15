---
paths:
  - "cmd/**/*.go"
  - "internal/**/*.go"
---

# Go backend rules

## Layering (ADR-004)

| Package | Does | Never |
|---|---|---|
| `cmd/server` | flags, wiring, lifecycle | business logic |
| `internal/api` | decode → validate → store call(s) → encode; maps errors to status; the only place that logs | SQL, `database/sql` |
| `internal/store` | SQL, transactions, version checks; returns wrapped errors + sentinels | log, format for HTTP |
| `internal/plan` | parse markdown into `seed.Document` | touch the DB or a logger |
| `internal/seed` | embedded seed, upsert/replace loader | delete rows or overwrite user progress |
| `internal/web` | embedded SPA + index.html fallback | know about the API |
| `internal/backup` | `VACUUM INTO` ticker, prune | kill the server when a backup fails |

The only decorators are the middleware in `internal/api/server.go`:
`recover → requestLog → tokenAuth → mux`. No service layers, repository interfaces, or store
decorators. If a second backend is ever needed, extract an interface then.

The import boundaries in this table are enforced by depguard in `.golangci.yml`;
`golangci-lint run ./...` names the broken rule and what to do instead.

## API handlers (`internal/api`)

- Register routes in `NewMux` with Go 1.22+ patterns:
  `mux.HandleFunc("PATCH /api/tasks/{id}", s.patchTask)`. No trailing slashes.
- Bodies: `decodeJSON(r, &dst)` (unknown fields rejected, exactly one object). Every GET handler
  starts with `requireKnownQuery(r, <allowed names>)`, even when it accepts none. Ids: `pathID(r)`.
  CSV filters: `splitCSV`.
- `internal/api/contract_test.go` fails when `NewMux` and the endpoint table in `docs/API.md` §2
  disagree, or when a GET handler accepts unknown query parameters. Fix the doc or the handler,
  never the test.
- Validate plan vocabulary against the store (`s.validateProjectID`, `s.validateCategoryID`,
  `s.validatePhaseID`, `s.validateSkillIDs`), never against a Go literal. Workflow enums use
  `allowedGoalStatuses` / `allowedTaskStatuses`.
- Respond with `s.writeJSON`, `s.writeError`, `s.writeNoContent`, `s.writeConflictCurrent`
  (412 + `current`). Unexpected store errors go through `s.handleStoreError`, which logs once.
- Versioned resources: `ETag` via `etag(id, version)`; `If-Match` via `parseIfMatch`
  (428 missing, 400 malformed).
- Status codes follow `docs/API.md` §1: 400 malformed or unknown reference, 404 missing id,
  409 unique conflict or plan hash mismatch, 412 stale version, 413 plan upload over 1 MB,
  422 well-formed but forbidden (task with no skills, replace without confirmation),
  428 missing `If-Match`.
- Writes that can award XP return the resource fields at the top level plus optional `game`
  (`taskWriteResponse`, `logWriteResponse`, … in `game.go`) so older clients can ignore it.
- Use `s.now()` and `s.location` — never `time.Now()` or `time.Local` in handlers.
- Pure derived values (`PlanWeekFor`, streaks in `time.go`) take `now`, `loc` and data as
  arguments.

## Store (`internal/store`)

- One concrete `*Store`; methods grouped by table across files; handlers depend on it directly.
- Wrap driver errors with `classify("op description", err)` so `ErrNotFound`, `ErrConflict`
  and `ErrConstraint` survive `errors.Is`.
- Multi-statement writes use `s.tx(ctx, func(tx execer) error { … })`. **Inside the callback use
  `tx`, never `s.db`**: the pool has exactly one connection (`SetMaxOpenConns(1)`), so a nested
  `s.db` call blocks forever.
- Timestamps come from `s.utcNow()` (RFC3339 UTC). Tests pin the clock with `SetClock`.
- Optimistic update: `UPDATE … SET …, version = version + 1 WHERE id = ? AND version = ?`;
  zero rows → `ErrConflict`. A Kanban move bumps only the moved card's version; renumbered
  neighbours keep theirs.
- `(*sql.Rows).Close` is exempt from errcheck; always check `rows.Err()`.
- The handle opens with WAL, `busy_timeout=5000`, `foreign_keys=ON`, `synchronous=NORMAL` via the
  DSN and verifies them. Never open the SQLite file any other way.

## Startup (`cmd/server`)

Fail fast: a migration error, locked DB, bad timezone, or unparseable seed/plan logs at ERROR
with the wrapped cause and exits non-zero. No degraded mode.

## Logging (ADR-006)

`log/slog` only: text handler for `-log-format=text`, JSON for `json`. Structured pairs
(`"path", p, "error", err`). Never log tokens or pairing codes; request logs already redact
`/pair/*` and `/api/pair/*`.

## Tests

- Real temp SQLite, never mocks.
- Store: `newTestStore(t)` (migrated, fixtures inserted with raw SQL so tests don't depend on
  plan content) or `newEmptyStore(t)`; clock pinned to `fixedNow`. Migration error paths use
  `mapFS`.
- API: `newTestAPI(t, token)` with `srv.request` / `srv.requestWithHeaders`, `assertStatus`,
  `decodeBody`; clock `apiNow`.
- Tests holding plan-shaped constants (seed keys, week codes, counts):
  `internal/plan/parse_test.go` (`TestParseRealMasterPlan`), `internal/seed/load_test.go`,
  `internal/api/api_test.go`, `internal/api/game_test.go`. Change them deliberately when the plan
  changes.
- Exported identifiers keep doc comments (revive). CI fails on anything `gofmt -l` reports.

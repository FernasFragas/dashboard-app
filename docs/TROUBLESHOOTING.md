# Troubleshooting

Failures that are silent, errors that mislead, and tooling surprises. For plan-document problems,
[PLAN_CHANGE_RUNBOOK.md](PLAN_CHANGE_RUNBOOK.md) is the detailed companion.

## Fails silently

| Symptom | Cause | Fix |
|---|---|---|
| Edited the plan, nothing changed | `make seed-gen` was not run; the app seeds from `internal/seed/seed.json`. | Validate, run `make seed-gen`, restart. |
| Duplicate tasks after a reseed | A week was renumbered or a task retitled, so its `seed_key` changed and the additive loader inserted a new row. | No un-merge: reset with a backup, or replace through `/plan`. |
| A whole plan section vanished | A heading collides by prefix (`## Skills tracker`), or a table outside a week became prose. | Check the `make seed-gen` counts; rename the heading or restore the table. |
| An achievement never unlocks | `internal/api/game.go` names a seed key or checkpoint week that no longer exists (TD-3). | Retarget the key; list real keys from `internal/seed/seed.json`. |
| A Kanban drag springs back and sends no request | Collision detection resolved to the source column. Unit tests stay green. | Manual drag pass; see `plan/M5-followups.md` F3. |
| A deleted seeded task comes back | The next boot re-inserts it because its `seed_key` is missing. | Expected; remove it from the plan instead. |
| `ON DELETE SET NULL` does nothing | `foreign_keys` is off. The store sets it through the DSN and verifies it. | Only open the database through `store.Open`. |

## Fails loudly but confusingly

| Message | Cause | Fix |
|---|---|---|
| `UNIQUE constraint failed: metric_defs.slug` at boot | A metric was renamed but kept its slug. | Reset, or restore the old metric name. |
| `database already holds plan "X", refused to load "Y"` | The plan id changed. | Another `-data` directory, a reset, or a `/plan` replace. |
| `-plan-reset requires an existing backup` | Deliberate guard. | Create `dashboard.db.backup` first. |
| `frontend is not embedded; run make dev or build with make build` | You opened :8484 on a development build. | Use the Vite URL, or `make build`. |
| golangci-lint refuses to lint the module | The linter binary was built with Go older than 1.26. | `make doctor` prints the `go install` command. |
| A request hangs forever | A store method used `s.db` inside `s.tx`; the pool has one connection. | Use the `execer` passed to the callback. |
| `412` on a goal nobody edited | Should not happen: renumbering does not bump versions (M5 F1). | Treat it as a regression. |
| `dev-instance: server exited during startup` | Migration, seed or flag error; the script prints the last log lines. | Read `<data>/server.log`. |
| `check-migrations: applied migrations were modified or deleted` | A merged migration changed. | Restore it from the base commit and add a new migration. |
| `check-migrations: base '…' is not a known commit` | No local copy of the base branch. | `git fetch origin main`. |
| `doccheck: stale path …` | A doc names a file that moved or never existed. | Fix the doc, or restore the file if the move was a mistake. |
| depguard: `import 'log/slog' is not allowed …` | A layering rule was broken; the message says what to do instead. | Follow the message; see ADR-004. |

## Tooling and environment

- `make check` **rewrites** files (`gofmt -w`, `prettier --write`). Use `make check-ci` to verify
  without modifying anything.
- A leftover `make dev` holds a port: `lsof -ti:8484 | xargs kill`, or use `make dev-instance`.
- The `-addr` default `:8484` listens on every interface, and `pnpm dev` runs Vite with
  `--host 0.0.0.0`. Fine on a development laptop; production must pass the tailnet IP.
- Vite proxies only `/api`. In development, `/pair/{code}` has to reach Go on its own port.
- `PRAGMA foreign_keys` cannot change inside a transaction, and the migration runner wraps every
  migration in one.
- `web/dist/` and `internal/web/dist/` are build outputs and are gitignored.

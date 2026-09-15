---
name: run-dashboard
description: Start an isolated instance of the dashboard (throwaway database, free ports, logs on disk) and verify behaviour against it — the API with curl, the UI in a browser, request logs and database state. Use whenever a change must be seen working rather than only tested, and never use make dev or ~/dashboard-data for this.
---

# Run the dashboard in isolation

`make dev` uses fixed ports and the user's real database. For verification, always use the isolated
instance.

## 1. Start it

Run in the background (Bash with `run_in_background: true`) from the repository root:

```sh
scripts/dev-instance.sh              # API + Vite
scripts/dev-instance.sh --api-only   # API only
scripts/dev-instance.sh --plan /path/to/plan.md
```

Read the task output until `dev-instance: ready`. It prints the API URL, the web URL, the data
directory and the log paths. The same values are in `<data>/instance.env`; load them with
`set -a; . <data>/instance.env; set +a`.

If it prints `server exited during startup`, the last log lines follow: that is a real boot failure
(migration, seed or flag), not a harness problem.

## 2. Verify

- **API:** `curl -fsS "$DASHBOARD_API_URL/api/health"`. Writes need
  `-H 'Content-Type: application/json'`. A `PATCH` on goals or tasks needs
  `-H "If-Match: W/\"<id>-<version>\""`; take the value from the `ETag` of a `GET`.
- **UI:** open `$DASHBOARD_WEB_URL` with the claude-in-chrome skill. Check about 390 px and ≥768 px
  wide, exercise the changed flow, and read the console for errors.
- **Logs:** `server.log` is JSON, one line per request with status and duration:
  `jq -c 'select(.status >= 400)' "$DASHBOARD_SERVER_LOG"` and
  `jq -c 'select(.level == "ERROR")' "$DASHBOARD_SERVER_LOG"`.
- **State:** `curl -fsS "$DASHBOARD_API_URL/api/export"`, or read-only queries with
  `sqlite3 "$DASHBOARD_DATA/dashboard.db"`.

## 3. Stop it

Stop the background task (TaskStop), or send SIGTERM to the script. It stops the server and Vite
and keeps the data directory for inspection.

## 4. Report

State what you verified and show the evidence: the commands, the relevant response or log lines,
and what you saw in the browser. Say what you did not verify.

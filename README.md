# Dashboard App

Single-user productivity dashboard. Go API plus a Vite React frontend, one binary, SQLite.

## How To Run

Install Go 1.26, Node 24 (see `.nvmrc`) and pnpm 11. Then, from the repo root:

```sh
make setup   # once: checks the toolchain, installs frontend dependencies
make dev
```

Open **http://localhost:5173**. `Ctrl-C` stops both processes.

That is the whole thing. `make dev` runs the Go API on `:8484` and Vite on `:5173`, which
proxies `/api` to it. On first boot the server creates `~/dashboard-data/dashboard.db`, applies
migrations, and seeds the plan.

Everything else in this file is optional: building a single binary, running it over Tailscale,
installing it on a phone, backups.

### If something is wrong

| Symptom | Cause |
|---|---|
| Port already in use | An older `make dev` is still running. `Ctrl-C` it, or `lsof -ti:8484 \| xargs kill`. |
| Page loads, API calls fail | The Go half is not up. Check the `[go]` lines in the same terminal. |
| Nothing to check off | Normal before the plan's first week. The banner says when it starts. |

More detail, including the optional API token and a temporary database:
[docs/RUN_LOCALLY.md](docs/RUN_LOCALLY.md).

Writing or swapping a plan document: [docs/PLAN-FORMAT.md](docs/PLAN-FORMAT.md) ·
[docs/NEW_PLAN_STRUCTURE.md](docs/NEW_PLAN_STRUCTURE.md) ·
[docs/CHANGE_OR_ADD_PLAN.md](docs/CHANGE_OR_ADD_PLAN.md).
When a plan edit fails to validate, fails to boot, or seeds the wrong thing:
[docs/PLAN_CHANGE_RUNBOOK.md](docs/PLAN_CHANGE_RUNBOOK.md).

Working on the code, yourself or with a coding agent: start at [AGENTS.md](AGENTS.md).

## Requirements

- Go 1.26.x
- Node 24.x, pinned in `.nvmrc`
- pnpm 11.x
- golangci-lint 2.x, only for `make check`

## Build One Binary

Build the production binary from the repo root:

```sh
make build
```

This runs the frontend build, copies `web/dist` into the Go embed directory, and compiles
`bin/dashboard` with `CGO_ENABLED=0`. The build fails if the embedded frontend is missing.

Run it manually:

```sh
./bin/dashboard -addr 127.0.0.1:8484 -data ~/dashboard-data -tz Europe/Lisbon -log-format=text
```

Useful flags:

- `-addr`: listen address, for example `127.0.0.1:8484` locally or `100.x.y.z:8484` on Tailscale.
- `-data`: directory containing `dashboard.db`, `backups/`, and logs.
- `-token`: optional `X-Token` required for `/api/*` except `/api/health`.
- `-tz`: IANA timezone for day boundaries.
- `-log-format`: `text` for manual runs, `json` under launchd.
- `-public-url`: base URL used in phone-pairing QR codes. Set this to the tailnet URL if `-addr`
  is a wildcard or localhost address.

## Run On Tailscale

Find the Mac's tailnet IP:

```sh
tailscale ip -4
```

Run or configure launchd with that IP:

```sh
./bin/dashboard -addr 100.x.y.z:8484 -data ~/dashboard-data -tz Europe/Lisbon -log-format=json
```

Do not bind production to `0.0.0.0`. The security model is the tailnet ACL plus the optional
`-token`, so the process should listen only on the Tailscale address.

What a tailnet is, what it protects, and what it does not:
[docs/TAILSCALE.md](docs/TAILSCALE.md).

## launchd

Copy the built binary and install the template:

```sh
mkdir -p ~/bin ~/dashboard-data/logs ~/Library/LaunchAgents
cp bin/dashboard ~/bin/dashboard
cp deploy/launchd/com.fernando.dashboard.plist ~/Library/LaunchAgents/com.fernando.dashboard.plist
```

Edit `~/Library/LaunchAgents/com.fernando.dashboard.plist` and replace:

- `__BIN_PATH__` with `/Users/fernando/bin/dashboard`
- `__ADDR__` with your Tailscale IP and port, for example `100.x.y.z:8484`
- `__DATA_DIR__` with `/Users/fernando/dashboard-data`
- `__TZ__` with `Europe/Lisbon`

Load or unload it:

```sh
launchctl load ~/Library/LaunchAgents/com.fernando.dashboard.plist
launchctl unload ~/Library/LaunchAgents/com.fernando.dashboard.plist
```

Logs go to:

```text
~/dashboard-data/logs/dashboard.out.log
~/dashboard-data/logs/dashboard.err.log
```

## Install On iPhone

Open `http://100.x.y.z:8484` in Safari while connected to Tailscale, then use Share > Add to
Home Screen. The app has a manifest and icons, but no service worker; if the tailnet is not
reachable, the app should fail plainly instead of showing stale data.

To avoid typing the token on the phone, open the Plan screen on desktop and use **Pair phone**.
The QR expires after 90 seconds and can be used once.

## Backups And Export

The server writes a nightly SQLite snapshot at 03:00 local time:

```text
~/dashboard-data/backups/dashboard-YYYYMMDD.db
```

The date is the previous local day, and the newest 14 `dashboard-*.db` files are kept. Plan
replacement backups named `pre-plan-*.db` are not pruned by the nightly job.

For on-demand JSON insurance:

```sh
curl -fsS http://100.x.y.z:8484/api/export > dashboard-export.json
```

## Checks

```sh
make check
```

`make check` runs Go formatting, vet, golangci-lint (including the layering rules), frontend
linting, TypeScript checking, Vitest, Go tests, the documentation link check and the
applied-migration check. `make check-ci` runs the same checks without rewriting files.

## API Curl Checklist

Run the server against a fresh data directory:

```sh
tmpdir="$(mktemp -d)"
go run ./cmd/server -data "$tmpdir"
```

In another shell:

```sh
goal_json="$(curl -fsS -X POST http://localhost:8484/api/goals \
  -H 'Content-Type: application/json' \
  -d '{"title":"Ship the eval CI gate","project":"synapse","done_means":"CI fails on regression","target":"W6"}')"
goal_id="$(printf '%s' "$goal_json" | jq -r '.id')"
goal_version="$(printf '%s' "$goal_json" | jq -r '.version')"

curl -fsS -X PATCH "http://localhost:8484/api/goals/$goal_id" \
  -H 'Content-Type: application/json' \
  -H "If-Match: W/\"$goal_id-$goal_version\"" \
  -d '{"status":"active","position":0}'

task_json="$(curl -fsS -X POST http://localhost:8484/api/tasks \
  -H 'Content-Type: application/json' \
  -d "{\"week\":\"W5\",\"title\":\"Re-run k6 after the timeout change\",\"project\":\"gateway\",\"goal_id\":$goal_id,\"skill_ids\":[1]}")"
task_id="$(printf '%s' "$task_json" | jq -r '.id')"
task_version="$(printf '%s' "$task_json" | jq -r '.version')"

curl -fsS -X PATCH "http://localhost:8484/api/tasks/$task_id" \
  -H 'Content-Type: application/json' \
  -H "If-Match: W/\"$task_id-$task_version\"" \
  -d '{"status":"done"}'

curl -fsS -X POST http://localhost:8484/api/logs \
  -H 'Content-Type: application/json' \
  -d "{\"category_id\":\"portfolio\",\"title\":\"Dashboard API checklist passed\",\"goal_id\":$goal_id}"

curl -fsS http://localhost:8484/api/logs/summary
```

# Dashboard App

Local productivity dashboard scaffolded as a Go API plus a Vite React frontend.

## Requirements

- Go 1.26.x
- Node 24.x
- pnpm 11.x
- golangci-lint 2.x

## Setup

```sh
cd web
pnpm install
```

## Development

```sh
make dev
```

The Go API listens on `http://localhost:8484`. Vite listens on `http://localhost:5173` and
proxies `/api` to the Go server. The first screen fetches `GET /api/health` live.

## Checks

```sh
make check
```

`make check` runs Go formatting, vet, golangci-lint, frontend linting, TypeScript checking,
Vitest, and Go tests.

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
  -d "{\"week\":\"W5\",\"title\":\"Re-run k6 after the timeout change\",\"project\":\"gateway\",\"goal_id\":$goal_id}")"
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

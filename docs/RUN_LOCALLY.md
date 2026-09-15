# Run Locally

This project runs as two local processes during development:

- A Go API server on `http://localhost:8484`
- A Vite React frontend on `http://localhost:5173`

Vite proxies `/api` requests to the Go server, so you open the frontend URL in the browser and
the app still talks to the local API.

## Requirements

Install these first:

- Go 1.26.x
- Node 24.x, pinned in `.nvmrc`
- pnpm 11.x
- golangci-lint 2.x, only needed for `make check`

Check the installed versions:

```sh
go version
node --version
pnpm --version
golangci-lint version
```

If you use `nvm`, run this from the repo root before installing frontend dependencies:

```sh
nvm use
```

## Install Frontend Dependencies

From the repo root:

```sh
cd web
pnpm install
cd ..
```

This installs the React/Vite dependencies used by the frontend. The Go dependencies are handled
by Go modules automatically when you run Go commands. Re-run `pnpm install` when
`web/pnpm-lock.yaml` changes.

## Start The App

From the repo root:

```sh
make dev
```

This starts both processes:

- `[go]` logs are from the API server
- `[vite]` logs are from the frontend dev server

Open:

```text
http://localhost:5173
```

Use `Ctrl-C` in the terminal running `make dev` to stop both processes.

## Local Data

By default the server stores local data here:

```text
~/dashboard-data/dashboard.db
```

On first boot the server creates the data directory, runs migrations, and seeds the database with
the plan vocabulary, categories, metric definitions, skills, tasks, goals, and checkpoints.

To run against a temporary database instead of your normal local data:

```sh
tmpdir="$(mktemp -d)"
go run ./cmd/server -data "$tmpdir"
```

That starts only the Go API. For the full app with Vite hot reload, use `make dev`.

## Use A Different Plan

By default the server uses the embedded `master-plan-v5.md` seed. The browser path is now the
safest way to try another markdown plan: open `http://localhost:5173/plan`, paste or choose the
`.md` file, preview it, then apply it.

For CLI-only testing, start the Go server with `-plan`:

```sh
go run ./cmd/server -data "$tmpdir" -plan /path/to/my-plan.md
```

If a `plan.yaml` file sits beside the markdown file, the parser reads it automatically. Validate
a plan before starting the server:

```sh
go run ./cmd/server plan validate /path/to/my-plan.md
```

The database remembers the loaded plan id. If you point an existing data directory at a
different plan, startup refuses so two plans are not merged accidentally. Use a new `-data`
directory for experiments.

To intentionally replace the plan in a data directory, first create `dashboard.db.backup` beside
`dashboard.db`, then start once with `-plan-reset`. This deletes the old database and reseeds it.

## Optional API Token

Local development does not require a token by default.

If you start the server with a token, every `/api/*` request must send `X-Token`. The frontend
can send it from `VITE_DASHBOARD_TOKEN` or from browser local storage key `dashboard.token`.

Example with a manual Go server:

```sh
go run ./cmd/server -token "dev-secret"
```

Then start Vite in another terminal:

```sh
cd web
VITE_DASHBOARD_TOKEN="dev-secret" pnpm dev
```

## Run Checks

Run the full project check from the repo root:

```sh
make check
```

This runs formatting, Go vet, golangci-lint, frontend linting, TypeScript checking, Go tests,
Vitest, the documentation link check and the applied-migration check.

To run a second, isolated copy of the app (free ports, throwaway database, logs on disk), use
`make dev-instance`.

For frontend-only checks:

```sh
cd web
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

## Common Issues

If `make dev` says a port is already in use, another server is probably still running. Stop it or
change the port manually.

If the API cannot write the database, check that `~/dashboard-data` exists and is writable:

```sh
ls -ld ~/dashboard-data
```

If the frontend loads but API calls fail, check that the Go server is running on `:8484`. The Vite
proxy depends on that port.

You can test the API directly:

```sh
curl -fsS http://localhost:8484/api/health
curl -fsS http://localhost:8484/api/dashboard
```

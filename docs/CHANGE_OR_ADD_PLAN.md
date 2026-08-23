# Change Or Add A Plan

Use this when you want to edit the current roadmap or run the app with a different one.

## Change The Default Plan

Edit:

```text
master-plan-v5.md
```

Then validate it:

```sh
go run ./cmd/server plan validate master-plan-v5.md
```

Regenerate the committed default seed:

```sh
make seed-gen
```

Start the app:

```sh
make dev
```

Why: the default app loads `internal/seed/seed.json`, and that file is generated from
`master-plan-v5.md`.

## Add A New Plan

Create a new markdown plan anywhere, for example:

```text
~/plans/my-plan.md
```

Optionally add `plan.yaml` beside it to set the plan id, name, start year, and active goal limit.
See `docs/PLAN-FORMAT.md` for the exact format.

Validate it:

```sh
go run ./cmd/server plan validate ~/plans/my-plan.md
```

To load it from the browser, open the app and go to:

```text
/plan
```

Paste the markdown or choose the `.md` file, preview it, then apply it.

Why: preview shows parse errors, diff counts, and replace risk before any database rows change.

## CLI Alternative

Run the API with that plan:

```sh
tmpdir="$(mktemp -d)"
go run ./cmd/server -data "$tmpdir" -plan ~/plans/my-plan.md
```

In another terminal, run the frontend:

```sh
cd web
pnpm dev
```

Open:

```text
http://localhost:5173
```

Why: `make dev` uses the embedded default plan. A custom plan needs the Go server started with
`-plan`.

## Existing Data

The database remembers the loaded plan id.

- Same plan id: restart the app and new reference data is applied safely.
- Different plan id: use a new `-data` directory so two plans are not mixed.
- Intentional replacement: back up `dashboard.db`, then start once with `-plan-reset`.

Example replacement:

```sh
cp ~/dashboard-data/dashboard.db ~/dashboard-data/dashboard.db.backup
go run ./cmd/server -plan ~/plans/my-plan.md -plan-reset
```

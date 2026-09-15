# Recipes

Step-by-step procedures for common changes. Each ends with the checks that prove it. The risk levels
come from [PLAYBOOK.md](PLAYBOOK.md) §4.

## Add or change an API endpoint (medium)

1. Write the contract in `docs/API.md`: a row in the §2 endpoint table and a §3 section, with status
   codes from §1.
2. Store: add or extend a method in the matching `internal/store/*.go` file, wrap driver errors with
   `classify`, and add a store test using `newTestStore(t)`.
3. Handler: add it to `internal/api/handlers.go` or the feature file (`skills.go`, `plan.go`,
   `pair.go`, `game.go`) and register it in `NewMux` in `internal/api/server.go`. Start GET
   handlers with `requireKnownQuery`; use `decodeJSON`, `pathID`, the vocabulary validators and
   `handleStoreError`.
4. If the write can earn XP, return the matching `…WriteResponse` wrapper with `game`.
5. API test with `newTestAPI` in `internal/api/api_test.go` or the feature's test file.
6. Frontend: a typed function and types in `web/src/api/client.ts`; the query key goes in
   `web/src/lib/queryKeys.ts` if more than one page uses it.
7. Verify: `make check-ci` (the contract test fails if §2 and `NewMux` disagree), then `curl` the
   endpoint on `make dev-instance`.

## Change the schema (high)

1. Edit `docs/DATABASE.md`: the §4 table section, §5 if an index is added, §8 if the table is
   plan-owned.
2. Add a new `migrations/` file with the next number. Never edit a committed one.
3. Update `internal/store/models.go` and the store methods; add the table to
   `internal/store/export.go` and to the export example in `docs/API.md`.
4. Plan-owned table: update `internal/seed/document.go`, both the additive and replace paths in
   `internal/seed/load.go`, and the parser if the data comes from markdown.
5. Verify: `make check-ci` (the schema contract test fails on an undocumented or unexported table),
   then boot `make dev-instance` and read `server.log` for the new schema version.

## Add a frontend screen or feature (medium)

1. Page in `web/src/pages/` as `<Name>Page.tsx`. Add its `<Route>` in `web/src/App.tsx`, and a
   `navigation` entry if it belongs in the tab bar.
2. Strings in a file under `web/src/copy/`.
3. Data through `web/src/api/client.ts` and TanStack Query. Mutations are optimistic and handle
   `412`; follow `web/src/pages/TodayPage.tsx`.
4. A `<Name>Page.test.tsx` that mocks `../api/client`.
5. Verify: `cd web && pnpm lint && pnpm typecheck && pnpm test`, then check the screen on
   `make dev-instance` at about 390 px and at ≥768 px.

## Add an achievement or tune XP (medium)

- New badge: a migration inserting into `achievements` (`code`, `name`, `description`,
  `sort_order`), a condition in `gameAchievementConditions` in `internal/api/game.go` keyed by that
  code, and a case in `TestGameAchievementConditions`.
- Reword a badge: a migration running `UPDATE achievements SET description = … WHERE code = …`.
  Codes never change. `migrations/008_v6_achievement_text.sql` is the model.
- Change XP amounts: a migration updating `xp_rules`. Existing `xp_events` keep their amounts.
- Conditions that name seed keys must use real keys from `internal/seed/seed.json` (TD-3).

## Edit the tracked plan (high)

Follow `docs/PLAN_CHANGE_RUNBOOK.md`, in its order: validate → `make seed-gen` (read the counts) →
choose additive, reset or replace → `go test ./...`. If week codes or task titles moved, retarget the
seed keys in `internal/api/game.go` and the plan-shaped test constants.

Try a different plan without touching real data: `scripts/dev-instance.sh --plan ~/plans/my-plan.md`.

## Add a dependency (medium, high if it shapes architecture)

1. Confirm the standard library or an existing dependency cannot do it.
2. Add it (`go get` or `cd web && pnpm add`), keeping `CGO_ENABLED=0` builds working.
3. Add a row to `docs/DEPENDENCIES.md` with why and the behaviour to know; write an ADR if it
   shapes the architecture.
4. Verify: `make check-ci` and `make build`.

## Turn recurring feedback into a check (low to medium)

1. Pick the cheapest layer that can see the problem: depguard (Go imports), ESLint (frontend code
   patterns), a contract test (code versus docs), `cmd/doccheck` (docs versus files), or a Claude Code
   hook (edit-time guard).
2. Write the failure message as: what is wrong, why the rule exists, what to do instead.
3. Prove it fires on a deliberate violation, then remove the violation.
4. Add a row to "Feedback already turned into checks" in `docs/TECH_DEBT.md`.

## Build and release on the Mac (high; ask first)

```sh
make build
# if the release contains a migration, snapshot the live database first
sqlite3 ~/dashboard-data/dashboard.db "VACUUM INTO '$HOME/dashboard-data/backups/pre-release-$(date +%Y%m%d%H%M).db'"
cp bin/dashboard ~/bin/dashboard
launchctl unload ~/Library/LaunchAgents/com.fernando.dashboard.plist
launchctl load   ~/Library/LaunchAgents/com.fernando.dashboard.plist
tail -n 50 ~/dashboard-data/logs/dashboard.out.log   # expect "database ready" and "server starting"
```

Migrations run against the live database on that restart. The nightly prune only removes
`dashboard-*.db`, so a `pre-release-*` snapshot is kept.

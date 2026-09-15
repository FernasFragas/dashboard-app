# Technical debt

Known shortcuts, drift and gaps, with what each costs and the direction of a fix. Add an entry when
you knowingly leave something behind; remove it in the pull request that fixes it.

_Last reviewed: 2026-09-16._

## Open

| ID | Debt | Cost | Fix direction |
|---|---|---|---|
| TD-1 | `dashboard-plan-v2.md` is cited by `plan/`, `docs/` and code comments but is not in the repository. | "See v2 §4" leads nowhere. | Commit it, or replace the citations with links to ADRs and [PRODUCT.md](PRODUCT.md). |
| TD-2 | `master-plan-v5.md` holds Master Plan v6. | Confusing name; invites a "fix" that breaks the build. | Rename to `master-plan.md` in one change: `Makefile`, tests, docs, `rootFiles` in `cmd/doccheck/main.go`. |
| TD-3 | Achievement conditions in `internal/api/game.go` hardcode seed keys and a checkpoint week. | Renumbering the plan silently makes badges unreachable; no test notices. | Test that every seed key referenced by a condition exists in `internal/seed/seed.json`, or move the keys into plan data. |
| TD-4 | Achievement code `boss_w12` now targets W16. | Misleading name. | Accept: codes are frozen join keys. Documented here and in the runbook. |
| TD-5 | Goal G11 spans two projects but is stored under one (`docs/DATABASE.md` §3). | Hidden when the board is filtered to `gateway`. | Accepted until a second multi-project goal appears; then add a join table. |
| TD-6 | Kanban drag has no automated browser test; M5 F3 re-test is still open. | dnd-kit wiring can break with every unit test green (it has before). | Playwright smoke test for one cross-column drag, or a recorded manual pass per release. |
| TD-7 | Observability is request logs only: no metrics endpoint, no traces. | Latency questions need log parsing (`jq` over JSON logs). | Deliberate for a single-user local app. Revisit with M13 hosting. |
| TD-8 | Write endpoints ignore unknown query parameters; only `GET` rejects them. | None today: no write handler reads the query string. | Apply `requireKnownQuery` if a write handler ever starts reading query parameters. |
| TD-9 | `cmd/doccheck` checks file paths and links, not heading anchors or Go identifiers named in docs. | A renamed function mentioned in a doc goes unnoticed. | Extend it to `#anchor` targets first; identifiers only if drift recurs. |
| TD-10 | `docs/API.md` §3 examples use v5-era dates and week focus text. | Examples look inconsistent with the current plan. | Refresh examples when the API next changes; they are illustrative, not contractual. |

## Feedback already turned into checks

Recurring review feedback belongs in automation, so the same mistake cannot be repeated. When you
add a check, add a row.

| Recurring problem | Check that now catches it | Added |
|---|---|---|
| Store, plan or seed importing a logger or HTTP; the API layer importing `database/sql` | depguard rules in `.golangci.yml` | 2026-09-16 |
| Routes added without documentation, or documented routes that do not exist | `TestEveryRouteIsDocumented` in `internal/api/contract_test.go` | 2026-09-16 |
| GET handlers silently ignoring typo'd query parameters (11 found and fixed) | `TestGetEndpointsRejectUnknownQueryParameters` | 2026-09-16 |
| Tables missing from `docs/DATABASE.md` or from the export | `internal/store/schema_contract_test.go` | 2026-09-16 |
| Edits to applied migrations | `scripts/check-migrations-immutable.sh` in CI; Claude Code hook at edit time | 2026-09-16 |
| Docs naming files that no longer exist (a removed package, a renamed ADR file) | `cmd/doccheck` via `make check-docs` | 2026-09-16 |
| Components calling `fetch` directly | `no-restricted-globals` in `web/eslint.config.js` | 2026-09-16 |
| Unformatted code reaching CI | `make fmt-check`; Claude Code format hook | 2026-09-16 |

## Candidates for the next check

- Seed keys referenced by achievement conditions exist (TD-3).
- No project, category or skill id literals in `web/src/` outside tests.
- User-facing strings live in `web/src/copy/`, not inline in JSX.

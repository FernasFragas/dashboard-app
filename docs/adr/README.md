# Architecture decision records

Each ADR records one decision, the options that were rejected, and the consequences to live with.
Read the full record before proposing to reverse one.

| ADR | Decision | Consequence to respect |
|---|---|---|
| [001](ADR-001-sqlite-over-postgres-and-flat-json.md) | SQLite through pure-Go `modernc.org/sqlite`, not Postgres or flat JSON | Zero ops. `CGO_ENABLED=0` must keep building. A backup is one file made with `VACUUM INTO`, never a raw copy of a WAL database. |
| [002](ADR-002-single-binary-with-embedded-frontend.md) | One Go binary with the frontend embedded (`go:embed`, build tag `embed_frontend`) | No Node at runtime. `make build` fails when `dist` is missing rather than shipping a blank page. |
| [003](ADR-003-auth-by-network-perimeter-not-accounts.md) | Authenticate by network perimeter (Tailscale ACL) plus an optional `X-Token`; no accounts | Bind to the tailnet IP only, never `0.0.0.0`, never Funnel. No login or session code. |
| [004](ADR-004-pragmatic-two-layer-over-hexagonal.md) | Two layers (api, store) plus pure functions, not hexagonal | No service layer, interfaces, store decorators or mocks. No logging below `internal/api`. Tests use real SQLite. Enforced by depguard. |
| [005](ADR-005-stdlib-servemux-over-chi-and-echo.md) | stdlib `http.ServeMux` with Go 1.22+ patterns, not chi, Echo or Gin | Hand-written middleware: recover → request log → token. No router dependency. |
| [006](ADR-006-slog-text-in-dev-json-in-prod.md) | `log/slog`: text handler in development, JSON under launchd | Chosen with `-log-format`. No zap or zerolog. |
| [007](ADR-007-skill-stats-are-derived-never-stored.md) | Skill stats derived from `task_skills` and `log_skills` on every read | No counters on `skills`; nothing to recompute after a backfill. |
| [008](ADR-008-xp-is-a-ledger.md) | XP is an append-only ledger (`xp_events`) with amounts in `xp_rules` | Level, title, skill XP, tiers and streak are derived. Awards are idempotent per `(source_type, source_id)`. Unchecking a task deletes its event. Achievement conditions live in Go, keyed by `code`; only unlock timestamps are stored. |
| [009](ADR-009-parse-plan-documents-at-upload-never-at-boot.md) | Parse uploaded plans only on preview and apply, never from stored source at boot | A bad stored source cannot block boot. `plan_sources` is audit data. |
| [010](ADR-010-pairing-uses-one-time-code.md) | Pair phones with a one-time 90 s code, not a raw token in the QR | Store only `sha256(code)`, burn it on redeem, answer unknown, expired and used codes identically, redact codes from logs, rate limit. |

## Technical decisions without their own ADR

Recorded in `dashboard-plan-v3.md` and the milestone specs under `plan/`:

- Tailwind v4, CSS-first; no `tailwind.config.js`.
- wouter instead of a full router; TanStack Query for server state; dnd-kit on desktop only.
- The day boundary is `Europe/Lisbon`, overridable with `-tz`.
- No service worker: offline fails plainly instead of showing stale data.
- launchd on macOS, not systemd.

Product rules (what the app does and refuses to do) live in [../PRODUCT.md](../PRODUCT.md).

## Adding or changing a decision

- New decision: copy the structure of ADR-001 (status table, context, ranked drivers, options,
  decision, consequences), take the next number, and add a row above.
- Changed circumstances without a reversal: append a dated `> **Amended …**` note under the status
  table, as ADR-004 does. Never rewrite the original decision text.
- Reversal: write a new ADR that supersedes the old one, and set the old one's status to
  `SUPERSEDED by ADR-0NN`.

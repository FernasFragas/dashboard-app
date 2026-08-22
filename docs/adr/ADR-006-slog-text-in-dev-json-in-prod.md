# ADR-006: Log with log/slog, text handler in dev and JSON under launchd

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-004, ADR-005, docs/ARCHITECTURE.md §1 |

## 1. Context and Problem Statement

The plan reserves an `internal/log` package but never says what goes in it. Logging matters
here in two very different situations: while building, when logs are read live in a terminal
next to the code; and in production, when the app runs unattended under launchd for months and
logs are read only after something went wrong — a failed migration, a backup that stopped
running, a 500 seen on the phone.

ADR-004 already fixed *where* logging happens: exactly once, at the API boundary, with nothing
below `internal/api` logging at all. This ADR decides the library and the output format.

## 2. Decision Drivers (ranked)

1. **Readable while building.** Logs are read live during development; noise is a direct cost.
2. **Machine-readable when unattended.** Diagnosing a failure weeks later means filtering by
   field — status, path, error — not eyeballing prose.
3. **Dependency count.** ADR-002 ships one self-contained binary; ADR-005 already declined a
   dependency for a larger need.
4. **Fit with the single-logging-site rule.** Structured key/value fields must be natural at
   the one place errors are logged.

## 3. Options Considered

- Option 0 — stdlib `log` with plain `Printf` lines *(status quo)*
- Option 1 — `log/slog` with a handler chosen at startup: text in dev, JSON in prod
- Option 2 — `log/slog` with the JSON handler always
- Option 3 — `zerolog` or `zap`

### Option 0: stdlib `log`, plain lines

- **Description** — `log.Printf("PATCH /api/tasks/42 -> 412 (3ms)")`.
- **Pros** — Nothing to add (#3). Readable in a terminal (#1).
- **Cons / risks** — No fields, so post-hoc filtering means grep and regex over prose (fails
  #2). Structure drifts as each call site formats its own message. No levels, so there is no way
  to separate a request line from a failure.
- **Cost** — None now; the cost is paid the first time something breaks unobserved.

### Option 1: `log/slog`, text in dev / JSON in prod

- **Description** — `internal/log` builds a `*slog.Logger` from a `-log-format=text|json` flag,
  default `text`. The rest of the app takes the logger as a dependency.
- **Pros** — Stdlib since Go 1.21, so no dependency (#3). Aligned key/value text output during
  development (#1). JSON under launchd, filterable with `jq` by status, path, or error (#2).
  Levels distinguish the `INFO` request line from an `ERROR` failure. `slog.Attr` fits the
  single respond-error helper exactly (#4).
- **Cons / risks** — Two output paths means a formatting bug could in principle appear in only
  one. The handler is chosen once at startup and cannot change without a restart.
- **Cost** — One small package and one flag.

### Option 2: `log/slog`, JSON always

- **Description** — One handler, JSON everywhere.
- **Pros** — One code path, no divergence between environments (#2, #4). No dependency (#3).
- **Cons / risks** — Every line during development is a JSON object, including the per-request
  line. Reading the terminal while building becomes squinting or piping through a formatter
  (fails #1) — and #1 is the situation that occurs daily for the next three months.
- **Cost** — None, and it taxes every development session.

### Option 3: `zerolog` or `zap`

- **Description** — A third-party structured logger.
- **Pros** — Faster in allocation-heavy paths; richer built-in output options (#2).
- **Cons / risks** — A dependency (#3) bought for performance that is irrelevant at a few
  requests a day. Non-stdlib API in every signature, for capability `slog` already covers.
- **Cost** — One dependency, permanently, for no reachable benefit.

### Comparison

| Driver (ranked) | Opt 0 stdlib log | Opt 1 slog dual | Opt 2 slog JSON | Opt 3 zerolog/zap |
| --- | --- | --- | --- | --- |
| 1. Readable while building | ✅ | ✅ | ❌ | ⚠️ |
| 2. Machine-readable unattended | ❌ | ✅ | ✅ | ✅ |
| 3. Dependency count | ✅ | ✅ | ✅ | ❌ |
| 4. Fits single-logging-site rule | ⚠️ | ✅ | ✅ | ✅ |

## 4. Decision

> **We will use `log/slog` with the handler selected at startup by a `-log-format` flag — text
> by default, JSON in the launchd plist — because it is the only option that is readable during
> the three months of daily development (#1) and filterable when something fails unattended
> (#2), with no dependency (#3).**

We accept maintaining two output paths. The divergence is confined to handler construction in
`internal/log`; every call site is identical, because a `slog.Attr` renders in both. What we
trade away is a single uniform log format across environments — deliberately, because the two
environments have genuinely different readers: a person watching a terminal, and `jq` reading a
file weeks later.

## 5. Consequences

**Positive**
- One `INFO` line per request with method, path, status, duration, and bytes as fields — the
  request log is queryable rather than merely present.
- The respond-error helper attaches the full wrapped `%w` chain as one `error` field, so
  ADR-004's log-exactly-once rule produces a genuinely complete record.
- No logging dependency; the binary stays self-contained.

**Negative**
- A formatting problem could appear in only one handler. *Mitigation:* a test that logs one
  record through each handler and asserts the expected fields are present in both.
- The format is fixed at startup. *Mitigation:* accepted — changing it is a flag and a restart,
  and the app is restarted by launchd anyway.
- Structured logs are more verbose per line than prose. *Mitigation:* levels — request lines at
  `INFO`, failures at `ERROR`; a quiet run is a few lines a day.

**Follow-up work created**
- `internal/log` with the handler selection and the `-log-format` flag (M0/M3).
- Request-log middleware emitting the field set above (M3).
- `-log-format=json` and a log destination in the launchd plist (M7).

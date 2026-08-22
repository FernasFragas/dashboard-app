# ADR-004: Use a two-layer handler/store split instead of ports-and-adapters

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-001, ADR-005, ADR-006, docs/ARCHITECTURE.md §2 |

## 1. Context and Problem Statement

The app is roughly twelve HTTP endpoints over ten SQLite tables, with three derived values
(plan week, streak, week completion). The surrounding portfolio includes LLMGateway-Go, which
*does* use hexagonal architecture with provider decorators — so the temptation is to apply the
same structure here for consistency, and to have it visible in a repo a reviewer might read.

This decides the internal package structure and where logging happens. It does **not** decide
the router (ADR-005), the log handler (ADR-006), or the storage engine (ADR-001).

## 2. Decision Drivers (ranked)

1. **Hand-written ceremony per feature.** Every layer is code written and maintained by hand;
   Go has no AOP and no code generation in this project.
2. **Testability of the parts that can actually be wrong.** Date-boundary logic and SQL are
   where the bugs live.
3. **Logging discipline.** The same failure must not be logged three times at three layers.
4. **Cost of retrofitting later** if a second backend or per-operation telemetry appears.

## 3. Options Considered

- Option 0 — Handlers talk to `database/sql` directly; no store package *(status quo)*
- Option 1 — Two layers: `internal/api` (transport) and `internal/store` (persistence), plus
  `internal/plan` for pure functions
- Option 2 — Full ports-and-adapters: a store interface, a SQLite adapter, and decorators for
  logging and metrics

### Option 0: Handlers talk to the database directly

- **Description** — SQL inline in each HTTP handler.
- **Pros** — Absolutely minimal indirection (#1). Everything about one endpoint is in one place.
- **Cons / risks** — SQL becomes untestable without spinning HTTP (#2). Query duplication
  across endpoints that read the same rows — the dashboard bundle alone reads four tables that
  other endpoints also read. Transaction boundaries end up in transport code.
- **Cost** — Cheapest to start; the duplication compounds from the third endpoint onward.

### Option 1: Two layers + pure functions

- **Description** — `internal/api` decodes, validates, calls the store, encodes. `internal/store`
  owns SQLite, transactions, and typed CRUD on one `Store` struct. `internal/plan` holds pure
  functions that take `now`, a location, and data, and return values.
- **Pros** — Store tests run against a real temp SQLite file with no HTTP (#2). Plan-week and
  streak — the logic most likely to be subtly wrong — become table tests with no I/O at all
  (#2). One obvious home for every kind of code (#1). One place to log (#3).
- **Cons / risks** — No interface at the store boundary, so handler tests cannot substitute a
  fake store; they use a real temp database instead. Swapping the backend would mean touching
  the concrete type.
- **Cost** — One package boundary and a set of sentinel errors.

### Option 2: Ports-and-adapters with decorators

- **Description** — A `Store` interface, a `sqliteStore` implementation, and decorator types
  wrapping it for logging and metrics.
- **Pros** — Handlers testable against fakes (#2). A second backend is a new implementation
  (#4). Per-operation telemetry is a decorator, not a scatter of call sites (#3).
- **Cons / risks** — Ten tables produce roughly twenty store methods. Each decorator must
  hand-implement all twenty to satisfy the interface, so one logging decorator is on the order
  of two hundred lines of mechanical code that must be updated with every method added (#1).
  There is no domain to isolate here — the "domain" is CRUD plus three date functions — so the
  port protects nothing. No second adapter is planned, and ADR-001 says a backend change would
  mean the app has become multi-user, which is a rewrite, not a swap.
- **Cost** — The highest per feature, permanently, for benefits that arrive only in a scenario
  this project has explicitly ruled out.

### Comparison

| Driver (ranked) | Opt 0 Direct SQL | Opt 1 Two layers | Opt 2 Hexagonal |
| --- | --- | --- | --- |
| 1. Ceremony per feature | ✅ | ✅ | ❌ |
| 2. Testability where bugs live | ❌ | ✅ | ✅ |
| 3. Logging discipline | ⚠️ | ✅ | ✅ |
| 4. Retrofit cost later | ⚠️ | ⚠️ | ✅ |

## 4. Decision

> **We will use two layers — `internal/api` for transport and `internal/store` for persistence,
> with derived values as pure functions in `internal/plan` — because it makes the code that can
> actually be wrong directly testable (#2) without paying hundreds of lines of hand-written
> decorator ceremony for a port that isolates nothing (#1).**

The rules that keep this honest:

1. Handlers are transport only: decode, validate, call the store, encode. No SQL.
2. The store is persistence only. No HTTP concepts.
3. Derived values are pure functions — `now`, the location, and the data are arguments.
4. **Zero log calls below `internal/api`.** The store and pure functions wrap errors with `%w`
   and return them; one respond-error helper logs each failure exactly once, at the boundary.
5. The only decorators in the system are the HTTP middleware chain: recover, request log, token.
6. Store tests run against a real temp SQLite file, never mocks.

We accept that handler tests need a real database rather than a fake store. In practice this
costs one `t.TempDir()` per test and buys tests that exercise the actual constraints and
`ON DELETE SET NULL` rules, which a fake would not.

Hexagonal architecture with provider decorators remains correct in LLMGateway-Go, where the
port is narrow, genuinely abstract, and has multiple real implementations. Applying it here
would be cargo-culting a shape whose preconditions are absent.

## 5. Consequences

**Positive**
- Adding an endpoint is one handler plus one store method — no interface, no fake, no decorator
  to update.
- Plan-week and streak are pure table tests, including the Lisbon/UTC boundary cases that are
  the most likely source of a silent bug.
- Every error appears in the log exactly once, with the full `%w` chain intact.

**Negative**
- Handlers cannot be tested against a fake store. *Mitigation:* an `httptest` suite over a real
  temp database, which is closer to reality anyway.
- Per-operation store telemetry would require touching call sites today. *Mitigation:* accepted
  — extract the interface then and wrap it; that is a mechanical retrofit with nothing to
  redesign, since the store is already the only thing handlers depend on.
- A reviewer expecting hexagonal in every repo may read this as under-engineering. *Mitigation:*
  this ADR is the answer, and the contrast with LLMGateway-Go is the point.

**Follow-up work created**
- Sentinel errors `store.ErrNotFound` and `store.ErrConflict`, and their status mapping (M3).
- The respond-error helper as the single logging site (M3).
- A lint rule or review check that `internal/store` never imports `internal/log`.

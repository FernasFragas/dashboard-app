# ADR-001: Store data in SQLite rather than Postgres or flat JSON files

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-002, ADR-003, docs/DATABASE.md |

## 1. Context and Problem Statement

The dashboard is a single-user accountability app that must run unattended on a personal Mac
and be reachable from a phone over Tailscale. It is the record of a 12-week plan: job
applications, completed tasks, daily reviews, benchmark numbers. Losing that data quietly is
the one failure the app cannot survive — it exists to be trusted as a record. There is no
existing storage; this decision starts from nothing.

It decides where the data lives and what the durability and backup story is. It does **not**
decide the schema (docs/DATABASE.md), the layering above it (ADR-004), or how the process is
supervised (ADR-002).

## 2. Decision Drivers (ranked)

1. **Zero operational surface.** Nothing to start, patch, or babysit besides the app itself.
2. **Durability under abrupt termination.** `kill -9` mid-write must not lose or corrupt data.
3. **Trivial backup and restore.** A backup must be one artefact, verifiable by opening it.
4. **Query power.** Weekly counters, per-category summaries, streaks over date ranges, and a
   metric time series are all aggregate queries.
5. **Portability.** Moving to a home server later should be a copy, not a migration project.

## 3. Options Considered

- Option 0 — Flat JSON files on disk *(status quo: no database at all)*
- Option 1 — SQLite via `modernc.org/sqlite`
- Option 2 — Postgres in Docker

### Option 0: Flat JSON files

- **Description** — One JSON file per entity, rewritten on change; the app holds state in
  memory and flushes.
- **Pros** — Nothing to install (#1). Readable and diffable by hand. Backup is `cp` (#3).
- **Cons / risks** — No transactions: a crash during a rewrite truncates the file and loses
  everything in it (fails #2 outright). Every counter, summary, and streak becomes hand-written
  Go over slices (#4). Concurrent requests need a hand-rolled lock.
- **Cost** — Lowest to start, and the cost grows with every aggregate query.

### Option 1: SQLite (`modernc.org/sqlite`)

- **Description** — One database file at `~/dashboard-data/dashboard.db`, WAL mode, opened by
  the app at boot. Pure-Go driver, so `CGO_ENABLED=0`.
- **Pros** — ACID transactions; a killed process loses at most the in-flight transaction (#2).
  Nothing to run alongside the binary (#1). Full SQL for counters and time series (#4). Backup
  is `VACUUM INTO` producing one openable file (#3). Pure-Go driver cross-compiles to Linux
  with a flag (#5).
- **Cons / risks** — Single writer; concurrent writes serialise. No network access to the data
  without going through the app. Type affinity is loose, so enums need explicit `CHECK`
  constraints. `foreign_keys` is off by default and must be set per connection — an easy,
  silent mistake.
- **Cost** — One dependency, one open call, a migration runner. WAL and `busy_timeout` are two
  PRAGMAs.

### Option 2: Postgres in Docker

- **Description** — A container alongside the app, with a volume and a connection string.
- **Pros** — Strongest data types and constraints. Real concurrency (#2, #4). Same engine as
  the Synapse project, so the operational skill transfers.
- **Cons / risks** — Docker must be running before the app, which turns a login into a
  dependency chain and breaks the launchd auto-restart story (fails #1). Backup becomes
  `pg_dump` plus a retention policy (#3). Version upgrades become a task. Every one of these
  costs is paid for concurrency this app will never have — one user, one writer.
- **Cost** — Highest ongoing: a second process to supervise, patch, and back up.

### Comparison

| Driver (ranked) | Opt 0 JSON | Opt 1 SQLite | Opt 2 Postgres |
| --- | --- | --- | --- |
| 1. Zero operational surface | ✅ | ✅ | ❌ |
| 2. Durability under `kill -9` | ❌ | ✅ | ✅ |
| 3. Trivial backup/restore | ✅ | ✅ | ⚠️ |
| 4. Query power | ❌ | ✅ | ✅ |
| 5. Portability | ✅ | ✅ | ⚠️ |

## 4. Decision

> **We will store all data in a single SQLite database opened by the app itself, using the
> pure-Go `modernc.org/sqlite` driver, because it is the only option that satisfies both
> zero-operations (#1) and crash durability (#2) while still giving real SQL for the app's
> aggregate queries (#4).**

We accept SQLite's single-writer limitation in exchange for having no second process to run.
That trade is free here and only here: this is a one-user app whose write rate is a few taps a
day. If it ever became multi-user, this ADR is the first thing to revisit — and the two-layer
store (ADR-004) is where that change would land.

We also accept SQLite's loose type affinity, paid for with explicit `CHECK` constraints on
every enum in docs/DATABASE.md.

## 5. Consequences

**Positive**
- The app is one process. launchd starting the binary is the entire deployment (ADR-002).
- A backup is one file. `VACUUM INTO` yields a snapshot that can be opened and inspected
  directly, so backups are verifiable rather than hoped-for.
- `CGO_ENABLED=0` holds, so a later move to a Linux home server is a cross-compile, not a port.

**Negative**
- Single writer under concurrent load. *Mitigation:* `busy_timeout=5000` and WAL; with one
  user this is theoretical.
- No external tooling can query the live data over a network. *Mitigation:* `GET /api/export`
  dumps every table as JSON on demand.
- `foreign_keys` off by default silently disables the `ON DELETE SET NULL` rules that protect
  history. *Mitigation:* the PRAGMA is applied on every connection and covered by a store test
  that deletes a goal and asserts its tasks and logs survive.

**Follow-up work created**
- Migration runner and `001_init.sql` (M2).
- Nightly `VACUUM INTO` backup ticker with keep-14 pruning (M7).
- A store test asserting `foreign_keys` is genuinely on.

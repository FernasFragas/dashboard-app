# M1 · Architecture & technical docs

**Depends on:** M0. **Unblocks:** M2, M3 (both are built from `docs/`, not from the plan).
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §2

Write the decisions down before the schema exists. **Authoring:** the coding agent drafts all of
`docs/` from v2 + this file; Fernando reviews and corrects before M2 starts. Drafting is
transcription — the decisions are already made.

## Decisions closed here (open in v2)

- **ADR-005 → stdlib `net/http` ServeMux.** Go 1.22+ patterns (`GET /api/tasks/{id}`,
  `r.PathValue("id")`), zero deps, hand-written ~15-line middleware chain (recover, request
  log, token).
- **ADR-006 (new) → `log/slog`**, text handler in dev / JSON handler in prod, chosen by
  `-log-format=text|json` (default `text`). This is what `internal/log/` contains.

## Files to produce

```
docs/
├─ ARCHITECTURE.md
├─ DATABASE.md
├─ API.md
└─ adr/
   ├─ ADR-001-sqlite-over-postgres-and-flat-json.md
   ├─ ADR-002-single-binary-with-embedded-frontend.md
   ├─ ADR-003-auth-by-network-perimeter-not-accounts.md
   ├─ ADR-004-pragmatic-two-layer-over-hexagonal.md
   ├─ ADR-005-stdlib-servemux-over-chi-and-echo.md
   └─ ADR-006-slog-text-in-dev-json-in-prod.md
```

1. **ARCHITECTURE.md** — component diagram from v2 §1; each component's responsibility; the
   request flow (tap → HTTP → SQLite tx → query invalidation); stack list with a one-line
   rationale per choice.
2. **DATABASE.md** — mermaid ERD; per table every column with type, nullability, default, CHECK
   enums; FK delete rules (goal deleted → `goal_id` SET NULL on tasks and logs, history
   survives); each index paired with the exact query it serves; migration policy (numbered,
   forward-only, applied at boot); seed policy (only when `goals` is empty).
   **This document is the source `001_init.sql` is transcribed from — schema changes start here.**
3. **API.md** — the v2 endpoint table plus one request/response JSON example per endpoint, the
   error envelope `{"error":"msg"}`, and status codes.
4. **The 6 ADRs**, using the template below.

## ADR template (fixed — all 6 use it, no filler prose)

Title names the **decision**, not the topic ("Route with stdlib http.ServeMux instead of chi or
Echo", not "Routing"). One decision per ADR — if you can't state it in one sentence, split it.
Every option gets at least one honest con; if you can't name a rejected alternative, it isn't a
decision, it's a fact for §1. Once `DECIDED` the file is immutable — supersede, don't edit.

```markdown
# ADR-00X: <decision-oriented title>

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | YYYY-MM-DD |
| **Author** | @FernasFragas |
| **Related** | ADR-00Y, docs/ARCHITECTURE.md |

## 1. Context and Problem Statement
3–6 sentences: trigger, current state, and explicitly what this does NOT decide.

## 2. Decision Drivers (ranked)
1. …  2. …  3. …

## 3. Options Considered
- Option 0 — status quo / do nothing
- Option 1 — <name>
- Option 2 — <name>

### Option N: <name>
- **Description**
- **Pros** — tied to driver numbers ("wins on #1, #4")
- **Cons / risks**
- **Cost**

### Comparison
| Driver (ranked) | Opt 0 | Opt 1 | Opt 2 |
| --- | --- | --- | --- |

## 4. Decision
> **We will X, because <the 2–3 drivers that dominated>.**

One paragraph naming what was traded away, on purpose.

## 5. Consequences
- **Positive**
- **Negative** — each with its mitigation
- **Follow-up work created**
```

## Done when

1. M2 and M3 could be built from `docs/` alone, without reading any `plan/` file.
2. `001_init.sql` is later written 1:1 from `DATABASE.md` with no invention.
3. `docs/adr/` holds 6 ADRs — v2 acceptance check #7 updates from 5 to 6.

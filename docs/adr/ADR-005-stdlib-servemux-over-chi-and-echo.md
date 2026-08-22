# ADR-005: Route with the stdlib http.ServeMux instead of chi or Echo/Gin

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-004, docs/API.md §4 |

## 1. Context and Problem Statement

The API is about twelve endpoints (docs/API.md §2). It needs method-aware routing, one path
parameter shape (`/api/goals/{id}`), a handful of query parameters, and a three-item middleware
chain: recover, request log, optional token check. Go 1.22 gave the standard library's
`ServeMux` both method matching and path wildcards, which removed the historical reason to
reach for a router immediately.

The real axis is not feature count — every candidate can serve twelve routes. It is whether
handlers stay `http.HandlerFunc`-shaped or adopt a framework's own context type, because that
choice propagates into every handler, every middleware, and every test in the project.

This decides routing and handler signatures only. It does **not** decide layering (ADR-004) or
logging (ADR-006).

## 2. Decision Drivers (ranked)

1. **Reversibility.** Whichever way this goes, backing out should not mean rewriting every
   handler and test.
2. **Dependency count.** Each dependency is a thing to update and a thing to justify (ADR-002
   ships one self-contained binary).
3. **Features actually required.** Method routing, one path param, one middleware chain.
4. **What it demonstrates.** This is a portfolio repo; the choice is visible.

## 3. Options Considered

- Option 0 — stdlib `net/http.ServeMux` *(status quo: already used for `/api/health` in M0)*
- Option 1 — chi
- Option 2 — Echo or Gin

### Option 0: stdlib `ServeMux`

- **Description** — `mux.HandleFunc("GET /api/tasks/{id}", h)` with `r.PathValue("id")`.
  Middleware is a hand-written chain of `func(http.Handler) http.Handler`.
- **Pros** — Handlers are `http.HandlerFunc`, the universal Go shape, so any other router can
  adopt them unchanged (#1). Zero dependencies (#2). Covers every required feature (#3). Shows
  the standard library is understood rather than reflexively wrapped (#4).
- **Cons / risks** — Route grouping must be done by convention rather than by API, so a
  `/api`-wide prefix is repeated in each pattern. The middleware chain is ~15 lines written by
  hand. No built-in request binding or validation helpers — decode and validate are explicit in
  each handler. Pattern-conflict panics at registration are less descriptive than chi's.
- **Cost** — The ~15-line chain helper; everything else is stdlib.

### Option 1: chi

- **Description** — A stdlib-compatible router: same `http.HandlerFunc` signatures, plus
  `Route`/`Group` and a middleware stack.
- **Pros** — Handler shape identical to stdlib, so it is equally reversible (#1). Route groups
  make an `/api` subtree and per-subtree middleware explicit (#3). Mature and small.
- **Cons / risks** — One dependency to gain grouping and a chain helper this project can write
  in fifteen lines (#2). At twelve endpoints, the grouping ergonomics are close to invisible.
- **Cost** — One dependency; no code changes to migrate to or from.

### Option 2: Echo or Gin

- **Description** — A framework with its own `Context` type: `func(c echo.Context) error`.
- **Pros** — Binding, validation, and rendering helpers included (#3). Large ecosystem of
  ready-made middleware (#4, arguably).
- **Cons / risks** — Every handler and every middleware is written against a framework type, so
  leaving means rewriting all of them along with their tests — the one option that is genuinely
  hard to back out of (fails #1). Brings a dependency tree well beyond the routing need (#2).
  Its own error-handling convention would sit awkwardly beside ADR-004's single-logging-site
  rule.
- **Cost** — Lowest per handler once adopted; highest to reverse.

### Comparison

| Driver (ranked) | Opt 0 ServeMux | Opt 1 chi | Opt 2 Echo/Gin |
| --- | --- | --- | --- |
| 1. Reversibility | ✅ | ✅ | ❌ |
| 2. Dependency count | ✅ | ⚠️ | ❌ |
| 3. Required features | ✅ | ✅ | ✅ |
| 4. What it demonstrates | ✅ | ⚠️ | ⚠️ |

## 4. Decision

> **We will route with the standard library's `http.ServeMux`, because it meets every actual
> requirement with zero dependencies (#2, #3) while keeping handlers in the universal
> `http.HandlerFunc` shape, so the decision stays reversible (#1).**

We accept writing the middleware chain and the route-prefix repetition by hand. That is roughly
fifteen lines and twelve string literals — a smaller cost than a dependency, and the one thing
we would have bought from chi.

chi is the honest runner-up: it costs one dependency and is a drop-in in both directions. If
route grouping or per-subtree middleware ever becomes real, adopting chi is a change to the
wiring file and nothing else — that reversibility is exactly why the stdlib choice is safe to
make now.

## 5. Consequences

**Positive**
- No routing dependency to track or update; ADR-002's binary stays lean.
- Handlers are ordinary `http.HandlerFunc`, so `httptest` works with no framework harness.
- Migrating to chi later, if grouping becomes worth it, requires changing only the mux
  construction.

**Negative**
- `/api` is repeated in every route pattern. *Mitigation:* routes are registered in one file;
  the repetition is visible and greppable, not scattered.
- No built-in binding or validation. *Mitigation:* explicit `json.Decoder` with
  `DisallowUnknownFields` plus per-handler validation — which docs/API.md already requires as
  behaviour, not as convenience.
- Route-conflict panics are terse. *Mitigation:* all routes are registered together, so a
  conflict surfaces at boot on the first run.

**Follow-up work created**
- The `func(http.Handler) http.Handler` chain helper and the three middlewares (M3).
- One route-registration file, so conflicts and coverage are inspectable in one place (M3).

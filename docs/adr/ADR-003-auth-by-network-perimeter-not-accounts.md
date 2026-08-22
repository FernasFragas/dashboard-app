# ADR-003: Authenticate by network perimeter (Tailscale ACL) instead of building accounts

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-002, docs/API.md §1 |

## 1. Context and Problem Statement

The app has exactly one user and holds personal but not sensitive data: task progress, job
applications, daily notes, benchmark numbers. It must be reachable from a phone away from home,
which means it cannot simply bind to `localhost`. Tailscale is already installed on both the
host and the phone, so the machine already sits on an authenticated private network.

This decides how requests are authorised. It does **not** decide transport encryption (Tailscale
provides it) or whether the app is ever exposed publicly — it is not, and this ADR is the reason.

## 2. Decision Drivers (ranked)

1. **No credential surface.** Every password, session, or token store is something that can
   leak, expire, or lock you out of your own accountability app on a Sunday night.
2. **Reachable from the phone, away from home.** A localhost-only bind is not viable.
3. **Build cost.** Auth is not the point of this project; hours spent here are hours not spent
   on the screens.
4. **Blast radius if misconfigured.** The failure mode of a wrong setting should be "not
   reachable", not "reachable by everyone".

## 3. Options Considered

- Option 0 — No auth, bound to `localhost` only *(status quo)*
- Option 1 — Tailscale ACL as the perimeter, plus an optional `X-Token` header
- Option 2 — Username/password accounts with server-side sessions
- Option 3 — OAuth via an external identity provider

### Option 0: Localhost only, no auth

- **Description** — Bind `127.0.0.1:8484`. Reachable only from the host machine.
- **Pros** — Zero credential surface (#1). Nothing to build (#3). Smallest possible blast
  radius (#4).
- **Cons / risks** — Not reachable from the phone, which is where the app is meant to be used
  (fails #2 outright — the entire quick-log design targets a thumb).
- **Cost** — None, and it does not deliver the product.

### Option 1: Tailnet perimeter + optional `X-Token`

- **Description** — Bind the Tailscale IP (`-addr 100.x.y.z:8484`). Only devices on the tailnet
  can route to it, and Tailscale has already authenticated them at the device level. An optional
  `-token` flag adds an `X-Token` header check as a second lock.
- **Pros** — The credential is the tailnet device identity, which already exists and is already
  managed (#1). Works from the phone anywhere (#2). Effectively free: a bind address and a
  fifteen-line middleware (#3). Encrypted in transit by WireGuard.
- **Cons / risks** — The security depends entirely on the bind address being right. Binding
  `0.0.0.0` on a network with port forwarding would expose an app with no auth. Anyone with
  access to an unlocked device on the tailnet has full access. The `X-Token` is a static shared
  secret — meaningful only as defence in depth.
- **Cost** — A flag, a middleware, and a README line.

### Option 2: Accounts with sessions

- **Description** — A users table, password hashing, login page, session cookies, logout.
- **Pros** — Independent of network topology; the app could be exposed publicly later (#2 in a
  broader sense).
- **Cons / risks** — Builds a credential store for one user (fails #1: now there is a hash to
  protect and a session to invalidate). Adds a login screen between a thumb and a log entry,
  attacking the app's core interaction. Meaningful hours for zero users gained (#3).
- **Cost** — High relative to everything else in the plan.

### Option 3: OAuth via an external provider

- **Description** — Delegate identity to Google/GitHub.
- **Pros** — No password to store (#1). Well-understood.
- **Cons / risks** — Requires a stable public callback URL, which contradicts the whole
  not-publicly-exposed posture (#4). Adds an external runtime dependency to opening your own
  todo list — provider down or token expired means no logging. Contradicts ADR-002's
  self-contained artefact.
- **Cost** — Moderate build, plus a permanent external dependency.

### Comparison

| Driver (ranked) | Opt 0 Localhost | Opt 1 Tailnet + token | Opt 2 Accounts | Opt 3 OAuth |
| --- | --- | --- | --- | --- |
| 1. No credential surface | ✅ | ✅ | ❌ | ⚠️ |
| 2. Reachable from phone | ❌ | ✅ | ✅ | ✅ |
| 3. Build cost | ✅ | ✅ | ❌ | ⚠️ |
| 4. Blast radius | ✅ | ⚠️ | ⚠️ | ❌ |

## 4. Decision

> **We will treat the tailnet as the authentication boundary — binding only to the Tailscale IP,
> with an optional `X-Token` header as a second lock — because it reuses an identity system that
> already exists and is already trusted (#1) while making the app reachable from the phone (#2)
> at essentially no build cost (#3).**

We accept that the app's security is a property of its **bind address** rather than of its code.
That is a real transfer of risk from something reviewable to something configured, and it is the
reason the port must never be exposed publicly and `0.0.0.0` must never be used. The README and
the launchd plist both state the bind explicitly rather than defaulting to a wildcard.

## 5. Consequences

**Positive**
- No login screen between opening the app and logging an application — the `<5 taps` target
  survives.
- Nothing to rotate, expire, or reset; no password reset flow to build for a single user.
- Traffic is encrypted by WireGuard without terminating TLS in the app.

**Negative**
- A misconfigured bind address exposes an unauthenticated app. *Mitigation:* the default
  `-addr` is `:8484` for local dev only; the launchd plist pins the tailnet IP; the README
  states the rule; `-token` is available as defence in depth.
- Any unlocked device on the tailnet has full access. *Mitigation:* accepted — the tailnet is a
  personal device set, and device-level compromise defeats a session cookie equally.
- The `X-Token` is a static shared secret with no rotation story. *Mitigation:* it is
  explicitly a second lock, never the only one; changing it is a flag change and a restart.

**Follow-up work created**
- Token middleware with `GET /api/health` exempted (M3).
- launchd plist pinning the tailnet bind address, and README security note (M7).

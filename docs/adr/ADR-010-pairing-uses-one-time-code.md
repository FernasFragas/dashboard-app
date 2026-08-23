# ADR-010: Pair phones with one-time codes, not raw tokens

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-003, docs/API.md |

## Context

The app has no accounts or sessions. ADR-003 makes the tailnet the authentication boundary, and
`-token` is an optional second lock. A phone pairing QR therefore cannot transfer a session; it
can only hand over the reachable URL, the current route, and the optional token.

Putting the raw token directly in the QR would work, but a photo or screen share would create a
permanent credential leak. The QR is meant to sit on a desktop screen long enough to scan, so the
credential should be short-lived and single-use.

## Decision

Pairing uses a randomly generated one-time code:

- `POST /api/pair` mints a code with a 90 second TTL.
- The QR points to `/pair/<code>`, not to a URL containing the token.
- The database stores `sha256(code)`, never the plaintext code.
- `GET /pair/<code>` burns the row and serves a tiny bootstrap page that stores the token and
  redirects to the requested route.
- Unknown, expired, and already-used codes return the same response.
- Request logging redacts `/pair/*` and `/api/pair/*` paths.

## Consequences

The phone can be paired without typing the token, while a stale QR photo is useless. There is
one unauthenticated endpoint, but it accepts only high-entropy, short-lived, one-use codes and is
rate limited. This extends ADR-003; it does not add accounts, sessions, or a public login flow.

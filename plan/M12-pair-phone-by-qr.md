# M12 · Pair the phone by QR

**Goal:** on the desktop, show a QR. Scan it with the phone. The phone opens the dashboard, on
the same screen you were looking at, already carrying the token — no typing a tailnet IP, no
typing a secret on a phone keyboard.

**Depends on:** M7 shipped (the token and the tailnet bind are real by then).
**Sequencing:** post-v1. **Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md)

---

## 0 · What "session" means here, and what a QR can and cannot do

This app has **no accounts and no sessions** (ADR-003: the tailnet ACL is the authentication;
`-token` is an optional second lock). So there is no session object to transfer. What the phone
actually lacks is three things:

| The phone needs | Where it lives on the desktop today |
|---|---|
| A reachable URL | the tailnet host — currently typed by hand |
| The token, if `-token` is set | `localStorage["dashboard.token"]`, or a build-time `VITE_DASHBOARD_TOKEN` |
| The screen you were on | the current route and query, e.g. `/goals?project=synapse` |

**What a QR cannot do:** put the phone on the tailnet. If the phone is not already on the
tailnet, the scan opens a URL that does not resolve. The pairing screen must say so plainly
rather than showing a code that silently fails — an unreachable QR looks like a broken feature.

---

## 1 · The one real decision: how the token travels

Encoding the raw token in a QR means anyone who photographs your screen has permanent access to
the tailnet-reachable app. On a personal machine that is a small risk, but it is a **permanent**
one, and it is avoidable for little work.

| Option | How | Cost |
|---|---|---|
| **A · Raw token in the QR** | `…/#token=<token>`, stripped by the client on load | Simplest. Screen photo = permanent credential. |
| **B · One-time pairing code** *(recommended)* | QR carries a short random code; the phone redeems it once for the token | ~100 lines and one table. A photo of a stale screen is worthless. |
| **C · URL only** | The phone still types the token | No new exposure, and no real improvement over today. |

**Recommendation: B.** The whole point is that the QR sits on a screen for a while, and screens
get photographed, shoulder-surfed and screen-shared. A credential that expires in a minute and
can be redeemed once is the difference between "a convenience" and "a convenience that quietly
widens the blast radius ADR-003 deliberately kept narrow".

Record as **ADR-010 · Pairing uses a one-time code, not the token itself**, and reference
ADR-003 — this is an extension of that decision, not a departure from it.

---

## 2 · The flow

```
desktop                          server                          phone
   │  POST /api/pair               │                               │
   │─────────────────────────────► │  mint code, TTL 90s, 1 use    │
   │  ◄──── {code, url, expires_at}│                               │
   │  render QR of url             │                               │
   │                               │  ◄──── GET /pair/<code> ──────│  (scan)
   │                               │  burn code, serve the SPA     │
   │                               │  with the token in the page   │
   │                               │  ─────── SPA + token ────────►│
   │                               │                               │  store token,
   │                               │                               │  navigate to route
```

### `POST /api/pair` — mint

Authenticated like everything else. Body carries the screen to hand over:

```json
{ "route": "/goals?project=synapse" }
```

Response:

```json
{
  "url": "http://dash.tailnet.ts.net:8484/pair/7f3a9c2e1b",
  "expires_at": "2026-08-22T22:41:30Z",
  "expires_in_seconds": 90
}
```

### `GET /pair/{code}` — redeem

**This is the one endpoint exempt from the token check** — necessarily, since the phone does not
have the token yet. That makes it the most security-sensitive route in the app, so:

- **Single use.** Redeeming deletes the row, in the same transaction that reads it.
- **Short TTL.** 90 seconds. Long enough to pick up a phone, short enough that a photograph
  taken later is useless.
- **High-entropy code.** 128 bits from `crypto/rand`, base32 without padding. Never a counter,
  never `math/rand`.
- **Constant-time comparison**, and an identical response for expired, unknown and already-used
  codes — a distinguishable "expired" reply tells an attacker their guess had the right shape.
- **Rate limited**: after ~10 failed redemptions in a minute, refuse all redemptions for a
  minute and log at `WARN`. On a tailnet this is belt-and-braces, but the endpoint is
  unauthenticated and cheap to protect.
- **Never logged.** The request-log middleware must redact the path for `/pair/` — otherwise the
  code lands in `~/dashboard-data/logs` in plaintext, which defeats the TTL entirely.

Redemption serves the SPA with a small inlined bootstrap — token and target route — rather than
returning JSON, so the phone lands in the app in one navigation rather than two.

### `plan_meta`-style storage

New table `pairing_codes`: `code_hash TEXT PK · route TEXT · expires_at TEXT · created_at TEXT`.

Store the **hash**, not the code, for the same reason password hashes exist: a stolen database
file should not yield a usable credential. A dropped `dashboard.db` is a plausible event —
backups are copied around by design (M7).

A boot-time sweep deletes expired rows; so does every mint. The table should almost always be
empty.

---

## 3 · Rendering the QR

**Server-side SVG, no new frontend dependency.** `GET /api/pair/{code}.svg` returns an SVG QR
of the pairing URL, and the desktop renders `<img src="…">`.

Why not a JS library: ADR-002 keeps the artefact self-contained, the frontend bundle is already
~100 KB gzipped, and this needs one image on one screen. A pure-Go QR encoder is a single small
dependency with no runtime cost on the client. If a Go dependency is unwelcome, the fallback is
`qrcode` on npm — but the payload then passes through JS, which is a slightly wider path for a
credential-bearing URL.

The SVG endpoint takes the code, so it is authenticated like the mint that produced it.

---

## 4 · UI

A **"Pair phone"** section on the existing `/plan` page — it is already the settings-shaped
screen, and this needs no tab of its own.

1. **Before minting:** a button, plus one line saying the phone must already be on the tailnet.
2. **After minting:** the QR, the URL as selectable text (for a phone that cannot scan), and a
   live countdown — *"Expires in 1:12"*.
3. **On expiry:** the QR greys out and the button becomes *"Show a new code"*. Never leave a
   dead QR looking live.
4. **Reachability warning:** if the desktop is browsing `localhost` rather than a tailnet
   address, the minted URL will not work from the phone. Detect it and say so:
   *"You're on localhost — the phone can't reach this. Open the dashboard on its tailnet
   address first."*
5. Copy in `web/src/copy/pair.ts`, per the existing rule.

**The URL in the QR comes from the server, not `window.location.origin`.** The desktop may well
be on `localhost:5173` in dev, and a QR encoding `localhost` is worse than no QR at all. Add a
`-public-url` flag (defaulting to the `-addr` host when it is not a wildcard) so the server
states its own reachable address.

---

## 5 · Tests

1. A minted code redeems once; the second redemption fails.
2. An expired code fails, and the response is byte-identical to an unknown code's.
3. The stored row holds a hash, not the code.
4. The request log does not contain the code — assert on captured log output.
5. `/pair/{code}` works without `X-Token`; every other `/api/` route still refuses.
6. Redemption hands over the route from the mint, and the phone lands there.
7. Rate limiting trips after the threshold and recovers.
8. Expired rows are swept at boot and on mint.
9. Frontend: countdown reaches zero and the QR is visibly dead; a `localhost` origin shows the
   reachability warning instead of a useless code.
10. `-public-url` unset with `-addr :8484` produces a URL with a real host, never `:8484` alone.

## 6 · Acceptance

1. Scan the QR on the phone with `-token` set; the app opens, authenticated, on the desktop's
   current screen, without typing anything.
2. Scanning the same QR twice fails the second time.
3. Waiting out the TTL and scanning fails, and the desktop offers a fresh code.
4. The pairing code appears in no log file and no database row in plaintext.
5. ADR-010 exists and references ADR-003.
6. `docs/API.md` documents both endpoints, including the token exemption and why it is safe.

## 7 · Out of scope

- **Transferring UI state beyond the route.** Draft quick-log text, expanded rows, scroll
  position — all local, none worth syncing.
- **Pairing without a token configured.** If `-token` is empty there is no credential to hand
  over, and the QR degrades to a plain deep link. Still useful; the flow just skips the code.
- **Anything resembling a login.** v2 §4 rules out accounts permanently. This hands over an
  existing secret; it does not establish an identity.

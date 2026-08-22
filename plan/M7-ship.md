# M7 · Ship

**Depends on:** M4, M5, M6.
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §2

One binary, on the tailnet, on your home screen, backing itself up.

## Build

- `go:embed web/dist` behind a build tag; `make build` runs `pnpm build` first and copies
  `dist` into the embedded directory. **The build fails loudly if `dist` is missing** — an
  embedded-empty binary that serves a blank page is the failure mode to design out (v2 §5).
- `CGO_ENABLED=0` → one static binary. `modernc.org/sqlite` makes this work with no CGO, which
  is the whole reason it was chosen (ADR-001): cross-compiling to a home server later is a flag
  change, not a project.
- The SPA fallback: any non-`/api/` path serves `index.html` so a deep link like
  `/?project=synapse` or `/goals` works on a cold open.

## Run — launchd

A `~/Library/LaunchAgents/com.fernando.dashboard.plist` starting the binary at login with
`KeepAlive`, so a crash or a reboot brings it back. This is macOS — v2's "systemd unit" doesn't
apply here.

- Binds the Tailscale IP: `-addr 100.x.y.z:8484`. **Never `0.0.0.0`** — the tailnet ACL is the
  auth (ADR-003).
- `-log-format=json`, stdout/stderr to `~/dashboard-data/logs/`.
- README documents `launchctl load/unload` and where to look when it's down.

## Startup failure — fail fast, exit non-zero

Migration failure, locked DB, unparseable seed: log the error at `ERROR` with the wrapped
cause, exit non-zero. No degraded mode. launchd's restart backoff makes a genuine failure loop
visible in the log. A half-started app serving 500s is worse than one plainly down — you would
trust data that isn't there.

## Backups

In-app ticker, nightly:

- `VACUUM INTO 'backups/dashboard-YYYYMMDD.db'` — SQLite's atomic snapshot. A plain file copy
  of a WAL database can produce a corrupt backup; this is consistent by construction and the
  output opens directly.
- Prune to the newest 14.
- Backup failures log at `ERROR` and never kill the server.
- `GET /api/export` stays the on-demand JSON insurance. Restore remains a future concern —
  the file and the JSON are the recovery path.

## PWA — manifest and icons only

`manifest.webmanifest` (name, dark theme colors, `display: standalone`), icons at 192/512 and
an Apple touch icon. **No service worker.** Over Tailscale the app is either reachable or
you're off the tailnet; a cache would mostly serve stale data and a confusing offline state,
and stale-shell bugs after a rebuild are a bad trade for a single-user app.

## README

1. Build and run, the flags (`-addr -data -token -tz -log-format`).
2. Tailscale: finding the tailnet IP, why binding to it (not `0.0.0.0`) is the security model.
3. `launchctl` install/uninstall, log locations.
4. Add-to-home-screen steps on iOS.
5. The M3 curl checklist.
6. Backup and export: where files land, how many are kept.

## Done when

1. One binary running on the host machine under launchd; reboot → it comes back by itself.
2. The phone opens it by tailnet name and installs to the home screen.
3. A deep link (`/goals`) opens correctly from a cold start.
4. Next morning, `backups/` contains yesterday's `.db` and it opens in a SQLite browser.
5. All seven v2 §6 acceptance checks pass — with check #7 amended to **6** ADRs.

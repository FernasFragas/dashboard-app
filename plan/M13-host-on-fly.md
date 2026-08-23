# M13 · Host on Fly.io, joined to the tailnet

**Goal:** the dashboard runs in the cloud so the phone works when the laptop is closed — without
putting a single byte on the public internet.

**Depends on:** M7 (single binary with embedded frontend — already true: `internal/web/dist_prod.go`).
**Sequencing:** post-v1. **Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md)

---

## 0 · What this does and does not change

The app joins the tailnet **from inside the Fly machine** using `tsnet`, so it is a tailnet node
that happens to run on Fly. It never listens on a public address.

| Decision | Status |
|---|---|
| **ADR-003** — tailnet ACL is the authentication, never expose the port publicly | **Unchanged.** Still true, still enforced. |
| **v2 §4** — "public deployment" out of scope | **Not crossed.** This is a cloud deployment that is not a public one. |
| **ADR-001** — SQLite, zero ops, backup is a file | Holds, with one addition: the file now lives on a volume that can vanish, so §3 adds off-volume replication. |
| **ADR-002** — one binary, no runtime deps | Holds and pays off: the container is `FROM scratch` plus the binary. |
| **M12** — QR pairing | Unaffected; the pairing URL is just a different tailnet host. |

---

## 1 · Read this before anything else: mirror or primary?

**Decision taken: the Mac stays primary, Fly is a mirror.** That is only coherent if Fly is
**strictly read-only** — two writable copies of an unsynced SQLite file is not a deployment, it
is a coin flip about which day's work survives.

**The consequence, stated plainly:** a read-only mirror cannot log anything. Logging an
application, checking off a task, writing the Sunday review — all refused. And the moment you
most want the phone to work is exactly when the Mac is closed, which is exactly when the mirror
is stale and read-only. **A read-only tracker is a viewer, not a tracker.**

So this plan builds the mirror as specified, and §8 documents the one-flag flip to make Fly
primary when you decide the mirror isn't earning its keep. Expect to want that flip. The mirror
is genuinely useful for one thing — glancing at the board and the plan week from anywhere,
without waking the laptop — and useless for the app's core action.

Everything below assumes **mirror** unless it says otherwise.

---

## 2 · Joining the tailnet with `tsnet`

`cmd/server` grows a serving mode:

- `-listen=tcp` (default) — today's behaviour, `-addr` as-is. Nothing changes for the Mac.
- `-listen=tsnet` — the app calls `tsnet.Server.Listen` and serves on the tailnet instead.
  `-tsnet-hostname` sets the node name (`dash-fly`), `-tsnet-state` the state directory.

Two things that bite if missed:

1. **`tsnet` state must live on the volume.** It holds the node identity. On ephemeral storage
   every deploy registers a *new* tailnet node, so you accumulate dead `dash-fly-1`,
   `dash-fly-2` entries and the hostname you bookmarked stops resolving.
2. **The auth key is a Fly secret** (`TS_AUTHKEY`), never a flag — flags land in `fly.toml`, in
   git, and in `fly machine status` output.

The token middleware still applies. On a tailnet-only listener it is defence in depth exactly as
ADR-003 describes.

### The machine cannot auto-stop

Fly's usual auto-stop works because Fly's proxy holds the connection and wakes the machine. With
`tsnet` there is **no Fly proxy in the path** — the tailnet connects directly to the running
process. A stopped machine is simply an offline tailnet node, and nothing can wake it.

So: `auto_stop_machines = false`, `min_machines_running = 1`. This machine runs 24/7, and that
is the cost of the design. State it in the README so it is not discovered on a bill.

---

## 3 · Durability: Litestream to object storage

The Fly volume is one disk in one region. M7's nightly `VACUUM INTO` writes to that **same
volume**, so a volume loss takes the database and every backup with it. That is not a backup, it
is a copy.

**Mac (primary) — replicate:**

Litestream streams the WAL from `~/dashboard-data/dashboard.db` to Tigris (or S3). Runs as a
second launchd agent beside the app's.

**Fly (mirror) — restore:**

1. On boot, if the volume has no database, `litestream restore` it from object storage.
2. Then refresh on a schedule — a periodic restore into a fresh file and an atomic swap, because
   the mirror has no live replication and would otherwise show boot-time state forever.

Refresh cadence is a tradeoff worth stating in the README: every few minutes is fine for
glancing at a board; it is not a sync protocol and must not be described as one.

**The mirror never seeds.** The seed loader writes, and its additive upsert would diverge the
mirror from the primary. With `-read-only`, boot runs migrations only if the restored schema is
behind, and skips seeding entirely.

---

## 4 · Enforcing read-only

Three layers, because one is not enough:

1. **Middleware**: any method other than `GET`/`HEAD` on `/api/` returns **`405`** with a plain
   body — `"this instance is a read-only mirror; write on the primary"`. Outermost, so no handler
   can be reached by accident.
2. **Store**: `-read-only` opens SQLite with `mode=ro` in the DSN. The database itself refuses a
   write even if a code path slips past the middleware.
3. **UI**: `GET /api/health` reports `"read_only": true`; the frontend disables every write
   affordance and shows one persistent banner — *"Read-only mirror. Log on the primary."*
   Disabling controls without explaining why is how M11's add-task button got reported as broken.

---

## 5 · Container and `fly.toml`

`Dockerfile`, multi-stage:

- Stage 1: `node` — `pnpm install --frozen-lockfile && pnpm build` into `web/dist`.
- Stage 2: `golang` — `CGO_ENABLED=0 go build` with the embed tag, so `dist` is compiled in.
- Stage 3: `FROM scratch` — the binary, CA certificates, and the Litestream binary. Nothing else.
  ADR-001's pure-Go SQLite driver is what makes a `scratch` image possible at all.

`fly.toml`:

- **No `[http_service]` block.** There is no public service — that absence is the security
  property, and it deserves a comment in the file saying so, or someone will "fix" it later.
- `[mounts]` the volume at `/data`; `-data /data`, `-tsnet-state /data/tsnet`.
- `[[vm]]` the smallest size that holds the working set; SQLite plus this app is tiny.
- `auto_stop_machines = false`, `min_machines_running = 1` (§2).
- One machine, one region. **Never scale to two** — two machines mounting two volumes is two
  divergent databases, and Fly will happily let you.

`.dockerignore` must exclude `web/node_modules`, `*.db`, `backups/`, and `.git`.

Secrets: `TS_AUTHKEY`, `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY`, and
`DASHBOARD_TOKEN` if the token is used.

---

## 6 · Which host does the phone use?

Both the Mac and Fly are tailnet nodes with different names — `dash` and `dash-fly`. The phone
does not merge them; you choose by URL, and the home-screen icon points at one.

Recommendation: keep the home-screen icon on the **Mac** (it is where you write) and bookmark
the mirror separately. The read-only banner from §4 is what stops you wondering why a tap did
nothing.

---

## 7 · Verification

1. Deploy, then `tailscale status` shows one node named as configured — and still one after a
   second deploy, proving state persisted.
2. `curl` the Fly node from the phone over the tailnet: `200`.
3. `curl` from off-tailnet: no route. There is no public address to try.
4. `POST /api/logs` on the mirror → `405` with the explanatory body.
5. Destroy the volume, redeploy: the database restores from object storage and the row counts
   match the primary at its last replication point.
6. Write on the Mac, wait one refresh interval, read on the mirror: the change is there.
7. `fly machine stop` then a tailnet request: fails cleanly, and the README says this is expected
   because there is no proxy to wake it.
8. `docker history` shows no source, no `node_modules`, no `.git`.

## 8 · Making Fly primary later — the flip

When the mirror stops being enough (see §1):

1. Stop the Mac agent. It must not write again.
2. Final Litestream replication from the Mac; confirm the object-storage generation is current.
3. On Fly: drop `-read-only`, enable seeding, start Litestream in **replicate** mode instead of
   restore.
4. Point the phone's home-screen icon at the Fly node.
5. The Mac becomes a client, or a read-only mirror by the same mechanism in reverse.

The important part is step 1: **exactly one writer, always.** Everything else is configuration.

## 9 · Out of scope

- **Multi-region, LiteFS, or read replicas with write-forwarding.** One user, one writer.
- **Making the app publicly reachable.** If that is ever wanted it is a different decision, and
  it supersedes ADR-003 rather than extending it.
- **Automatic failover between Mac and Fly.** There is no leader election here, and inventing
  one for a single-user tracker is how you get two primaries.

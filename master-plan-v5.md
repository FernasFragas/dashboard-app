# Master Plan v6 — Final, Executable, Tagged · Robotics Integrated

Supersedes v5. **What changed:** the robotics transition (RoboticsTransition-Plan-2026-08) is front-loaded as W1–W4; everything else shifts +4 weeks; Phase 1 is now 16 weeks (Aug 24 → Dec 13). Nov 9 becomes a mid-flight review; the full checkpoint moves to Dec 13. OSS target is decided: **foxglove/mcap (Go), 3 merged PRs**. Medium posts renumbered (fleet post is #1). `mcap-tools` is the one sanctioned repo exception (guardrail #1).
**Dashboard reseed warning:** week codes shifted +4 — if seed_keys embed week codes, an additive reseed will duplicate moved tasks. Reset the db now while it's nearly empty; don't merge two calendars.

**Tags:** `[synapse]` synapsePlatform · `[gateway]` LLMGateway-Go · `[dash]` dashboard app · `[mcap]` mcap-tools (sanctioned exception) · `[oss]` foxglove/mcap · `[learn]` courses/reading/certs · `[write]` Medium · `[career]` CV/applications/networking · `[all repos]`

## Projects

| id | label |
|---|---|
| synapse | Synapse |
| gateway | LLM Gateway |
| dash | Dashboard |
| mcap | mcap-tools |
| oss | Open source |
| learn | Learning |
| write | Writing |
| career | Career |
| all | All repos |

## North star

Backend → Platform/Distributed → **AI × Robotics infrastructure — the fleet data plane** → Founder/CTO.
Thesis (from the robotics plan): not entering robotics — entering the infrastructure layer robotics companies buy or build, where the problems are ones already solved at Emma.
Target roles: **Rerun (apply W3)** · Foxglove · Viam · Northern.tech · Sift · (NEURA, ARX if relocating) — alongside the AI-infra leg (Nebius, Langfuse, E2B). If geography blocks robotics, the AI-infra leg lands first and the robotics hop comes in a year, from strength.
Comp staging: now → €70–120k → €120–200k → equity upside.

## Operating system

| Day | Slot |
|---|---|
| Mon | Theory: DDIA ~1 ch + 1 ByteByteGo case (1–2h) |
| Tue–Wed | Project work (2–3h) |
| Thu | SAA prep (until exam) → then Udemy LLM (2h) + 30 min community |
| Fri | Project work + log metrics (2h) |
| Sat | Deep build (4–6h) |
| Sun | Review, 3-bullet log, Medium draft. Rest. |

Daily: 55 min learn · 10 break · 55 build · 15–30 wrap (tests pass, commit, 3 bullets).
Cadence: daily log → Sunday review → biweekly metrics delta → **mid-flight review Nov 9 · checkpoint Dec 13 · checkpoint Mar 21** → reality check every 6 months.

## Log categories (the + buttons)

| id | label | icon |
|---|---|---|
| application | Application | 📮 |
| module | Course module | 📚 |
| portfolio | Portfolio | 🔧 |
| post | Medium post | ✍️ |
| oss | OSS event | 🔀 |
| number | Benchmark/number | 📊 |
| network | Networking | 🤝 |
| exam | Exam/cert | 🎓 |

## Goals (Kanban seed — codes stable, targets updated; G18–G21 new)

| ID | Goal | Project | Done means | Target |
|---|---|---|---|---|
| G0 | Dashboard live | dash | v1 on tailnet, seeded | Aug 31 |
| G1 | Eval harness | synapse | Rejection-rate number, 30-window golden set **incl. fleet domain**, 2 models, CI gate | W7 |
| G2 | Reliability proven | gateway | RELIABILITY.md: 5 experiments w/ graphs + 1 pprof fix | W11 |
| G3 | Postgres migration | synapse | Pool/isolation/index justified + honest concurrent number | W14 |
| G4 | pgvector search | synapse | Top-k semantic query live | W14 |
| G5 | K8s deploy | synapse | On kind: probes, HPA | P2-B1 |
| G6 | AWS SAA | learn | Passed | P2-B1 |
| G7 | OSS presence | oss | **3 merged PRs in foxglove/mcap** | W13 |
| G8 | Writing | write | 4 posts: fleet W4 · eval W8 · chaos W11 · fine-tune B2 | B2 |
| G9 | Career round 1 | career | CV+LinkedIn rebuilt, 10 applications (Rerun already sent W3) | W16 |
| G10 | Model serving | gateway | vLLM behind gateway: batching + streaming, bake-off | W12 |
| G11 | Fine-tune | synapse + gateway | LoRA + quantize + eval vs base on golden set | W15 |
| G12 | Kafka depth | synapse | DLQ, retries, backpressure, idempotency live | P2-B3 |
| G13 | Experiment infra | synapse | W&B/MLflow + nightly eval regression alert | P2-B2 |
| G14 | IaC | synapse (infra/) | Terraform: cloud PG (W13) → full stack (W16) | W16 |
| G15 | Security pass | all repos | OWASP review, gosec/govulncheck clean, OAuth | W16 / P2-B6 |
| G16 | Network | career | 1 conference attended | P2-B6 |
| G17 | Role | career | Platform / AI-infra / **robotics-infra** offer signed | Q2 2027 |
| G18 | Robotics literacy + mcap-tools | mcap | Read a public bag cold; mcap-tools reads/writes valid MCAP (`mcap doctor` passes) | W1 |
| G19 | Fleet repoint | synapse | MCAP source behind same interface, 5 mapped types, fleet dashboard, timeline endpoint, grounded AI fleet summary, recording in README | W3 |
| G20 | Rerun application | career | Applied, artifact + recording linked | W3 |
| G21 | MQTT third source | synapse | QoS-1 duplicate-convergence demo | W4 |

---

# Phase 1 — 16 weeks (Aug 24 → Dec 13)

## W1 · Aug 24–30 — Robotics on-ramp (G18) + dashboard (G0)

- [ ] **[learn] ROS 2 literacy (~3h, weekday slots)**
  1. Learn the seven concepts to one-sentence depth: node, topic, message type, service, action, bag, TF.
  2. Install the `mcap` CLI + Foxglove (free tier); open a Foxglove sample bag; 20 minutes of just looking.
  3. Rehearse the six "why robot data is hard" points out loud; deliver #5 (intermittent connectivity) and connect it to the Emma partner-outage work in the same breath.
  → **Done =** you can describe a public bag's topics, types and rates without looking anything up.
  → **Skills =** ros2

- [ ] **[mcap] mcap-tools (Saturday, ≤1 weekend, <500 lines — the sanctioned exception)**
  1. `mcap info` / `mcap cat` on the IEEE DataPort bag (`rosbag2_2024_07_19_11_01_37`: 53k+ msgs, 100+ topics).
  2. Reader: iterate messages, print channel/topic/log-time/publish-time/size; counts match `mcap info`.
  3. Read the embedded Schema → Channel → Message records — same shape as a schema registry.
  4. Writer: synthetic MCAP from scratch; `mcap doctor` passes.
  5. Decode one CDR type (`BatteryState` or `Twist`) into a Go struct — once is enough.
  → **Done =** repo reads a bag, prints a topic/rate summary, writes valid MCAP; frozen after this week.
  → **Skills =** mcap

- [ ] **[oss] foxglove/mcap contribution setup**
  1. Watch issues; read CONTRIBUTING + the last 10 merged PRs in `go/`.
  2. Bookmark 3 candidate issues (docs, conformance tests, reader/writer edge cases).
  3. Note the rules: ~+100/−100 lines, issue-first for anything non-trivial, no bot PRs, draft while iterating, CI needs maintainer approval — expect latency.
  → **Done =** 3 candidates bookmarked.
  → **Skills =** oss, mcap

- [ ] **[learn] Subscriptions + DDIA ch.1** — O'Reilly + ByteByteGo, 2 newsletters, DDIA ch.1 with 3 bullets logged.
  → **Skills =** distsys
- [ ] **[dash] Dashboard v1 (G0)** — finish per `dashboard-plan-v3.md`; hard stop at the timebox.
  → **Skills =** tooling

## W2 · Aug 31–Sep 6 — Fleet repoint Stage 1

- [ ] **[synapse] MCAP ingest source (Stage 1)**
  1. Add an `mcap` source behind the same source interface as Kafka — **do not fork the pipeline**; the source-agnostic normaliser is the architectural argument.
  2. Map five types → domain events: `BatteryState`→FleetBatteryReading · `Twist`→RobotVelocityCommand · `Odometry`→RobotPose · `DiagnosticArray`→RobotDiagnostic · `rosgraph Log`→RobotLogEvent.
  3. Deliberately skip point clouds and camera frames — store reference + size only; write the *why* (cost; the interesting events are elsewhere) into the README now.
  4. Ingest the IEEE bag end to end; normalised events land in the store.
  → **Done =** one real bag flows through the existing pipeline, unforked.
  → **Skills =** mcap, distsys

- [ ] **[learn] AWS SAA started** — buy one course (Cantrill or Maarek, pick in 10 min); module 1 Thursday; exam sits in B1 (December).
  → **Skills =** aws
- [ ] **[oss] PR #1 (docs or conformance)** — small, learns the review culture. → **Done =** opened, responding to review within 24h.
  → **Skills =** oss

## W3 · Sep 7–13 — Fleet semantics + AI leg + **apply to Rerun** (G19, G20)

- [ ] **[synapse] Fleet semantics (Stage 2)**
  1. Add a `robot_id` dimension; ingest the same bag ×3 under different ids to simulate a fleet.
  2. Idempotency keys on `(robot_id, topic, log_time)` — a replayed bag converges instead of duplicating. Say the partner-identity transplant explicitly in the README.
  3. Fleet metrics: `fleet_robots_reporting` · `fleet_last_seen_seconds{robot_id}` · `fleet_events_ingested_total{robot_id,topic}` · `fleet_battery_percent{robot_id}` · `fleet_dlq_depth`.
  4. **One** Grafana dashboard: robots reporting, last-seen heatmap, battery per robot, ingest rate by topic, DLQ depth.
  5. `GET /fleet/{robot_id}/timeline?from=&to=`.
  → **Done =** replay converges; the dashboard is live.
  → **Skills =** distsys, observability

- [ ] **[synapse] AI leg (Stage 3, half a day)**
  1. Point the bounded-evidence summariser at fleet events: "Summarise robot-02, 14:00–14:30."
  2. Confirm unsupported-citation rejection fires on fleet windows too.
  → **Done =** one grounded fleet summary, demo-ready. Lead with it.
  → **Skills =** evals

- [ ] **[synapse + career] Ship + apply**
  1. README rewritten around fleet telemetry, problem-mapping table near the top (partner outage → robot offline · idempotent partner keys → device convergence · DLQ/replay → store-and-forward backfill).
  2. `docker compose up` + a documented one-liner that ingests a public bag and populates the dashboard.
  3. 60–90s screen recording (dashboard + one AI summary), linked in README.
  4. CV synapse entry re-nouned; **apply to Rerun (Infrastructure)** with artifact + recording.
  → **Done =** application sent with the artifact linked (G20). Never slip this.
  → **Skills =** career, writing

## W4 · Sep 14–20 — MQTT + Medium #1 (G21)

- [ ] **[synapse] MQTT third source**
  1. Paho Go client + embedded `mochi-mqtt` broker for tests.
  2. Publish a subset of normalised events at QoS 1, consume back; duplicates converge via the same keys — the source abstraction proven real.
  3. Know cold: QoS 0/1/2 (and why teams avoid 2), persistent sessions/clean start, retained messages, **Last Will and Testament as the offline detector**, Paho's `Store` requirement, topic wildcards vs Kafka partitioning.
  → **Done =** convergence demo; you can explain QoS 1 vs at-least-once Kafka handled identically.
  → **Skills =** mqtt, distsys

- [ ] **[write] Medium #1** — "What robot fleets and third-party logistics have in common," straight from the mapping table. → **Done =** live URL.
  → **Skills =** writing
- [ ] **[career] CV skills + robotics summary variant** — add MCAP, ROS 2 (literacy, stated honestly), MQTT; summary variant leading with *delivering state reliably to systems you can't reach*. → **Done =** variant saved.
  → **Skills =** career

## W5 · Sep 21–27 — Golden set (now four domains)

- [ ] **[synapse] Golden set committed (then frozen forever)**
  1. Create `eval/golden/`; exporter (`cmd/goldenexport`) dumps event windows to JSON, one file per window.
  2. Pick 16–26 real windows across Energy / Finance / Monitoring **/ Fleet** (fleet windows exist since W3 — this is the synergy).
  3. Hand-write 4 nasty ones: empty window, single event, one domain flooding, near-identical timestamps.
  4. `eval/golden/README.md`: one line per file. Commit.
  → **Done =** set in git, read-only from now on.
  → **Skills =** evals

- [ ] **[synapse] summary.v2 prompt in a versioned prompts dir**
  1. `prompts/summary.v1.txt` (current) + `summary.v2.txt` with one deliberate change, diff noted in header.
  2. `SummaryPromptVersion` loads by version via config.
  → **Done =** either version runs by flipping one config value.
  → **Skills =** evals

- [ ] **[oss] PR #1 shepherd + substantive issue** — merge PR #1 if pending; pick the substantive Go reader/writer issue for PR #2; comment intent. → **Done =** issue claimed.
  → **Skills =** oss

## W6 · Sep 28–Oct 4 — Eval runner + metrics

- [ ] **[synapse] Eval runner** — `cmd/evalrun`: {v1,v2} × {model A, model B} × every golden file; capture raw output + `ValidateAgainstEvidence`; one row per combo to `eval/results/<date>.json`; `make eval`. → **Done =** committed results file.
  → **Skills =** evals
- [ ] **[synapse] Core metrics + baseline** — rejection rate, schema-validity %, evidence-coverage %; summary table; log headline numbers (📊). → **Done =** baseline in git + dashboard.
  → **Skills =** evals

## W7 · Oct 5–11 — Ship the number (G1)

- [ ] **[synapse] Stability + cost + latency** — `-runs=5` per window at prod temperature; drift %; tokens + latency → € per summary. → **Done =** columns in results.
  → **Skills =** evals
- [ ] **[synapse] Regression gate in CI** — `make verify` fails if v2 rejection > v1 baseline; GitHub Actions job; prove it red on a broken prompt, green on revert. → **Done =** you watched CI fail.
  → **Skills =** evals
- [ ] **[synapse] The sentence** — "v1→v2 dropped rejection X%→Y% on a 30-window golden set (incl. fleet), 2 models." README top + CV + 📊. → **Done =** real X and Y.
  → **Skills =** evals, writing

## W8 · Oct 12–18 — Write-up + observability + vLLM up

- [ ] **[write] Medium #2** — the eval harness, number in the title, results table + 1 diagram. → **Done =** live URL.
  → **Skills =** writing
- [ ] **[gateway] Prometheus + Grafana** — `/metrics`; histogram (route, provider), slots gauge, Redis-up, JWKS-cache-age; compose Prometheus + Grafana; one dashboard, JSON exported. → **Done =** p99 moves under test load.
  → **Skills =** observability
- [ ] **[gateway] OTel on the hot path** — spans auth → ratelimit → slot → provider; Jaeger in compose; screenshot to `docs/`. → **Done =** full trace saved.
  → **Skills =** observability
- [ ] **[gateway] vLLM serving a small model** — rent GPU if needed (RunPod/Lambda, ~€0.3–0.7/h); `vllm serve` a Qwen2.5-1.5B-class model; curl streamed completion; launch command in docs. → **Done =** streamed completion.
  → **Skills =** serving
- [ ] **[oss] PR #2 (substantive)** — implement + test per the project's checklist; open referencing the issue. → **Done =** open, CI green.
  → **Skills =** oss

## W9 · Oct 19–25 — Chaos: fail-open / fail-static

- [ ] **[gateway] k6 mixed-load baseline** — fast high-rps + slow streaming clients, different keys; 10 min; snapshot to `docs/reliability/baseline.png`. → **Done =** saved.
  → **Skills =** chaos
- [ ] **[gateway] Kill Redis mid-load** — traffic keeps succeeding; degradation gauge captured; limits re-engage on restart; annotated graph. → **Done =** flat traffic through the outage.
  → **Skills =** chaos
- [ ] **[gateway] Kill JWKS mid-load** — cached keys keep validating; staleness metric visibly rising; annotated graph. → **Done =** evidence in `docs/reliability/`.
  → **Skills =** chaos

## W10 · Oct 26–Nov 1 — Chaos: slots + failover → vLLM

- [ ] **[gateway] Slot starvation** — slow client holds every slot; measure fast client (queue time, 429s, latency); 5 lines on what slots showed that rate limits can't. → **Done =** graph + notes.
  → **Skills =** chaos
- [ ] **[gateway] vLLM wired as provider `local-small`** — one proxied request; metrics/traces carry `provider=vllm`. → **Done =** visible in Grafana.
  → **Skills =** serving, observability
- [ ] **[gateway] Failover lands on vLLM** — break the primary under load; failover latency from traces; response names the actual model. → **Done =** number + attribution proof.
  → **Skills =** chaos, serving

## W11 · Nov 2–8 — Profile + RELIABILITY.md (G2)

- [ ] **[gateway] pprof: one bottleneck** — 30s CPU + heap; hypothesis; fix; `go test -bench` + k6 p50/95/99 before/after. → **Done =** table committed.
  → **Skills =** go-perf
- [ ] **[gateway] RELIABILITY.md** — 5 sections: claim → method → graph → result + "what surprised me" each; nothing surprising = re-run harder. → **Done =** merged, 5 graphs.
  → **Skills =** chaos, writing
- [ ] **[write] Medium #3 + honest CV numbers** — "I chaos-tested my own README"; replace "~750 msg/s" with measured numbers. → **Done =** URL + CV updated.
  → **Skills =** writing, career
- [ ] **[oss] PR #2 shepherd; line up PR #3.**
  → **Skills =** oss

## W12 · Nov 9–15 — **Mid-flight review (Mon Nov 9)** + serving week (G10)

- [ ] **[dash] Mid-flight review (30 min, Mon Nov 9)** — eval number real? robotics pipeline responding (Rerun/others)? PRs merging? scope for W13–16 still right? Adjust in writing. → **Done =** written answers.
  → **Skills =** career
- [ ] **[gateway] Batching + token streaming** — 20 concurrent vs sequential throughput; SSE pass-through flushes per chunk; TTFT metric. → **Done =** `curl -N` streams; TTFT on dashboard.
  → **Skills =** serving
- [ ] **[gateway] Bake-off table** — 3–4 configs (FP16, AWQ, hosted API, optional size-up); identical k6 profile ×10 min; p50/95/99, TTFT, tokens/s, €/1k; `docs/serving-bakeoff.md`. → **Done =** ≥3 measured rows. *(Overflow → W13 Mon/Thu slots.)*
  → **Skills =** serving

## W13 · Nov 16–22 — Postgres, provisioned by Terraform

- [ ] **[synapse] Terraform module #1: cloud Postgres** — `infra/` inside synapsePlatform, pinned providers, Neon or RDS; connection string output; secrets via gitignored `.tfvars`; `destroy && apply` reproduces; psql connects. → **Done =** two commands rebuild the DB.
  → **Skills =** terraform
- [ ] **[synapse] Postgres behind sqlc + pool + migrations** — pgx/v5, explicit `MaxConns` with written reasoning; sqlc engine switch; goose `001_init`; full suite green on cloud PG. → **Done =** tests green; reasoning written.
  → **Skills =** postgres
- [ ] **[oss] PR #3** — open; **G7 target: 3 merged by end of this week** (8 weeks from start, per the robotics plan). → **Done =** 3 merged or explicit status + next move.
  → **Skills =** oss
- [ ] **[learn] SAA practice exam #1** — score logged; 3 weakest domains queued.
  → **Skills =** aws

## W14 · Nov 23–29 — Postgres depth + pgvector (G3, G4)

- [ ] **[synapse] EXPLAIN → index → EXPLAIN** — pg_stat_statements; slowest hot-path query; `EXPLAIN (ANALYZE, BUFFERS)` before/after a goose-migrated index. → **Done =** both plans + delta committed.
  → **Skills =** postgres
- [ ] **[synapse] Isolation level, chosen on purpose** — pick vs the ingestion path's actual anomalies; set explicitly; 10-line why; one test demonstrating the anomaly. → **Done =** level + why + test.
  → **Skills =** postgres
- [ ] **[synapse] The honest number** — re-run the W9 load profile with N concurrent writers on Postgres; publish next to the retired 750 (📊). → **Done =** public, even if worse.
  → **Skills =** postgres, chaos
- [ ] **[synapse] pgvector top-k** — extension via migration; batch-embed summaries through the gateway; `<=>` top-k endpoint; HNSW index, latency before/after. → **Done =** sane neighbors, latency recorded.
  → **Skills =** retrieval
- [ ] **[learn] Postgres internals reading** — autovacuum, BRIN/GIN, partitioning; 5 bullets on what synapse will need. → **Done =** notes logged.
  → **Skills =** postgres

## W15 · Nov 30–Dec 6 — Fine-tune week (G11)

- [ ] **[synapse] Dataset curated** — 500–2,000 input→accepted-summary pairs into `ml/data/` (fleet summaries eligible); dedupe, strip sensitive, instruct JSONL, 90/10 split; freeze val; never train on the golden set. → **Done =** train/val committed with counts.
  → **Skills =** finetuning
- [ ] **[synapse] LoRA fine-tune → quantize** — same GPU class as W8; base = the served model; Axolotl/Unsloth 1–3 epochs, stop when val flattens; merge; AWQ; configs to `ml/models/`. → **Done =** quantized model + config saved.
  → **Skills =** finetuning
- [ ] **[gateway + synapse] Serve tuned, eval vs base** — provider `local-tuned`; run the W6 eval runner on the golden set + bake-off cost/latency; write the true verdict (📊). → **Done =** table + verdict, whatever they say.
  → **Skills =** finetuning, evals, serving
- [ ] **[learn] SAA practice exam #2.**
  → **Skills =** aws

## W16 · Dec 7–13 — **Checkpoint** + Terraform stack + career (G9, G14)

- [ ] **[dash] Checkpoint (45 min, Sat Dec 13)** — interview-grade eval number? chaos falsified anything? 3 PRs merged or stale? fifth repo started? **robotics-infra vs AI-infra vs PM/founder — which leads Q1 applications?** Any bad answer → adjust B1 now. → **Done =** written answers.
  → **Skills =** career
- [ ] **[synapse] Terraform full stack (infra/)** — small VM module (Hetzner/EC2), cloud-init Docker + compose (gateway, synapse, Prometheus, Grafana); W13 PG outputs wired; `destroy && apply` + smoke test; one-command bring-up in `infra/README.md`. → **Done =** fresh env end-to-end from zero.
  → **Skills =** terraform
- [ ] **[all repos] Security pass** — gosec + govulncheck on synapse/gateway/dash, fix or triage; trufflehog history scan, rotate finds; 30-min OWASP walk of the gateway. → **Done =** `docs/security.md` clean-or-triaged.
  → **Skills =** security
- [ ] **[all repos] Polish + conference** — README top per repo: what it is → the numbers → one diagram → quickstart; pin repos; profile README with headline numbers; book conference. → **Done =** 2-minute story per repo; ticket booked.
  → **Skills =** writing, career
- [ ] **[career] CV + 10 applications** — 1 page: fleet repoint, 3 numbers, 3-PR OSS line, serving + fine-tune; LinkedIn featuring posts #1–3; send 10 across robotics-infra (Foxglove, Viam, Northern.tech, Sift ± NEURA/ARX) + AI-infra (Nebius, Langfuse, E2B, Go platform), each logged (📮). → **Done =** 10 logged.
  → **Skills =** career

*Slip order if a week breaks: Terraform full stack → K8s → bake-off extras → MQTT slides one week. Never slip OSS or the W3 Rerun application.*

---

# Phase 2 — 2-week blocks (Dec 14 → Mar 21, 2027)

*Re-scope at the Dec 13 checkpoint. Holiday-aware. Steps kept lighter on purpose.*

## B1 · Dec 14–27 — SAA + K8s (holiday-light Dec 24–27)

- [ ] **[learn] Sit the SAA (G6):** 1. practice exam #3 → 2. book + sit before Dec 20 → 3. log it (🎓). **Done =** pass.
  → **Skills =** aws
- [ ] **[synapse] K8s on kind (G5):** 1. `kind create cluster` → 2. Deployments + Services + liveness/readiness probes in `deploy/` → 3. HPA on CPU, load until it scales. **Done =** you watched a scaling event.
  → **Skills =** k8s

## B2 · Dec 28–Jan 10 (holiday-light)

- [ ] **[learn]** Udemy LLM course starts in the freed Thursday slot; finish DDIA. Rest counts as progress this block.
  → **Skills =** serving, distsys
- [ ] **[synapse] Eval regression alerts (G13):** 1. pick W&B (free tier) or MLflow → 2. log every eval run (params + metrics) → 3. nightly GitHub Action runs the eval and alerts on regression → 4. test the alert once on purpose. **Done =** one real alert received.
  → **Skills =** evals, observability
- [ ] **[write] Medium #4 (G8):** "fine-tuning vs prompting, measured" — the W15 verdict table becomes the post. **Done =** live URL (✍️).
  → **Skills =** writing

## B3 · Jan 11–24 — Kafka depth (G12)

- [ ] **[synapse] DLQ + replay CLI:** 1. create `<topic>.dlq` → 2. consumer: N retries with backoff, then publish to DLQ with error headers → 3. small CLI to inspect + replay. **Done =** a poison message lands in the DLQ and replays clean.
  → **Skills =** distsys
- [ ] **[synapse] Idempotency proof:** 1. event UUID + unique constraint / processed table → 2. test: duplicate delivery causes exactly one effect. **Done =** the test proves it.
  → **Skills =** distsys
- [ ] **[synapse] Backpressure:** 1. bound in-flight work (worker pool; pause/resume the consumer on depth) → 2. slow the DB on purpose: lag grows, memory stays flat. **Done =** the graph shows exactly that.
  → **Skills =** distsys

## B4 · Jan 25–Feb 7 — Gateway hardening

- [ ] **[gateway] Result caching:** Redis cache keyed by the existing cache key, TTL configurable; hit-rate metric; tune to >60% on replayed traffic. **Done =** the panel shows >60%.
  → **Skills =** serving
- [ ] **[gateway + synapse] Timeouts on every outbound call:** deadlines everywhere; retry+jitter where idempotent; cost budget alert. **Done =** grep finds no call without a deadline.
  → **Skills =** distsys
- [ ] **[gateway] Per-key quotas:** quota table + middleware + 429 with headers. **Done =** k6 proves the cap.
  → **Skills =** security, chaos

## B5 · Feb 8–21

- [ ] **[gateway] Per-tenant cost dashboard:** tokens × price per key → Grafana panel. **Done =** top-3 spenders visible.
  → **Skills =** observability
- [ ] **[learn] GPU basics:** read one serving-GPU primer; profile vLLM GPU util during a bake-off rerun. **Done =** 5-bullet notes.
  → **Skills =** serving
- [ ] **[learn]** System-design drills ×2 (multi-region inference).
  → **Skills =** distsys

## B6 · Feb 22–Mar 7

- [ ] **[oss] One more substantive mcap PR (stretch):** G7's three already merged by W13 — this one is depth, not the target. **Done =** merged, or dropped on purpose.
  → **Skills =** oss, mcap
- [ ] **[gateway] OAuth/OIDC (G15):** client-credentials flow via a Keycloak container; retire the static-key path behind a flag; OWASP recheck. **Done =** token-based auth works end to end.
  → **Skills =** security
- [ ] **[career] Conference (G16):** goal = 5 real conversations, 3 follow-ups within 48h (🤝).
  → **Skills =** career

## B7 · Mar 8–21

- [ ] **[career]** 10 more applications (📮) · 2 mock interviews.
  → **Skills =** career
- [ ] **[synapse]** Tag v0.3 + **[write]** case-study post.
  → **Skills =** writing
- [ ] **[dash] Checkpoint Mar 21:** written review, same 5-question format.
  → **Skills =** career

# Phase 3 — Quarters (2027)

**Q2:** interview loop → land the role (G17) — robotics-infra or AI-infra per the Dec 13 call; 1 serving-depth block/week continues. **Q3:** on-the-job scale; GPU depth; pprof/eBPF habit. **Q4:** 1 post/quarter; mentoring; evaluate the €120–200k move. **2028+:** staff scope → founder/CTO review. *(Expanded at the Mar 21 checkpoint.)*

## Metrics targets

| Metric | Baseline | Target | Slug | Unit | Definition | How to measure |
|---|---|---|---|---|---|---|
| Rejection rate v2 vs v1 (incl. fleet windows) | — | measurably ↓, CI-gated | rejection_rate | % | Share of golden-set summaries ValidateAgainstEvidence rejects, fleet windows included | make eval over the frozen 30-window set (four domains); record per prompt × model |
| p95 latency (cached) | >1s | <0.8s | p95_latency | s | 95th-percentile request latency under the standard mixed profile | Grafana after a 10-min k6 run; never compare across different profiles |
| p99 latency | >2s | <2s | p99_latency | s | 99th-percentile request latency under the standard mixed profile | Same run as p95 — read both off one k6 pass |
| Cache hit rate | <20% | >60% | cache_hit_rate | % | hits ÷ (hits + misses) | Gateway cache metric over a 10-min replayed-traffic window |
| Cost per 1k queries | — | −30% | cost_per_1k | € | Euro cost per 1,000 requests | API pricing × tokens, or GPU-hour ÷ requests served — say which in the note |
| Fine-tuned vs base on golden set | — | ≥ baseline at lower cost | tuned_vs_base | ratio | Tuned ÷ base rejection rate on the golden set | Eval runner, both providers, same day |
| Throughput, concurrent writers | "750" (SQLite, misleading) | real measured number | throughput_concurrent | msg/s | Messages per second with N concurrent writers on Postgres | Rerun the W9 load profile; put N in the note |
| OSS PRs merged (foxglove/mcap) | 0 | 3 by W13 | oss_prs_merged | PRs | PRs you opened that a foxglove/mcap maintainer merged | Count from your GitHub PR list; merged only — open or closed-unmerged do not count |
| Applications sent | 0 | 1 by W3 · 11 by W16 · 21 by B7 | applications_sent | applications | Applications actually submitted | Count the 📮 entries in the Log for the period; the Log is the record, this is the roll-up |
| Medium posts | — | #1 W4 · #2 W8 · #3 W11 · #4 B2 · 5 by B7 | medium_posts | posts | Posts published, not drafted | Count live URLs; a draft is a Learned bullet, not a metric |
| Time to first token | — | tracked from W12 | ttft | s | Request to first streamed token through the gateway | TTFT metric during the standard k6 profile |
| Robots reporting | 0 | 3 simulated by W3 | fleet_robots_reporting | robots | Distinct robot_ids seen in the ingest window | fleet_robots_reporting gauge on the fleet dashboard, over the last 5 min |

## Skills

| code | name | description | associate when | target |
|---|---|---|---|---|
| ros2 | ROS 2 literacy | Nodes, topics, message types, services, actions, bags, TF — reading robot data, not writing robot code. | the task reads, explains, or maps ROS 2 concepts or bag contents. | Practitioner |
| mcap | MCAP | The container format: Schema/Channel/Message records, reader and writer paths, mcap info/cat/doctor, CDR decoding. | the task reads, writes, or validates MCAP. | Expert |
| mqtt | MQTT | QoS levels, persistent sessions, retained messages, Last Will and Testament as an offline detector, Paho's Store. | the task publishes or consumes over MQTT. | Practitioner |
| evals | Evaluation systems | Measuring LLM output quality: golden sets, rejection/coverage/stability metrics, regression gates. | the task creates, runs, or gates on an evaluation. | Expert |
| go-perf | Go performance | pprof, allocation discipline, benchmarks, hot-path fixes. | the task profiles, optimises, or benchmarks Go code. | Expert |
| chaos | Load & chaos testing | k6/vegeta, dependency-kill experiments, proving failure modes. | the task generates load, or breaks something on purpose to observe behaviour. | Expert |
| observability | Observability | Prometheus, Grafana, OTel traces, dashboards, alerting. | the task adds or reads metrics, traces, or dashboards. | Expert |
| serving | LLM serving | vLLM, batching, token streaming, TTFT, provider wiring, bake-offs. | the task runs, routes to, or measures a model server. | Expert |
| finetuning | Fine-tuning & quantization | Dataset curation, LoRA, AWQ/GGUF, tuned-vs-base verdicts. | the task prepares data for, trains, or compresses a model. | Practitioner |
| postgres | Postgres internals | pgxpool, isolation levels, EXPLAIN, indexes, migrations, vacuum. | the task touches the relational layer below the ORM line. | Expert |
| retrieval | Retrieval & pgvector | Embeddings, top-k search, HNSW, retrieval latency. | the task stores or queries vectors. | Practitioner |
| distsys | Distributed systems | DDIA theory, and Kafka in practice: DLQ, retries, backpressure, idempotency, consistency. | the task deals with partial failure between processes. | — |
| terraform | Infrastructure as code | Terraform modules, reproducible environments, destroy/apply proofs. | the task provisions anything by code instead of by clicking. | Practitioner |
| k8s | Kubernetes | kind, Deployments, probes, HPA, manifests. | the task runs workloads on a cluster. | Practitioner |
| aws | AWS | SAA preparation and exam, service architecture, IAM. | the task studies or applies AWS specifics. | Expert |
| security | Security engineering | gosec/govulncheck, secrets hygiene, OWASP, OAuth/OIDC. | the task finds, fixes, or prevents a vulnerability class. | Practitioner |
| oss | Open-source collaboration | Working in codebases you don't own: repros, PRs, review etiquette, maintainer time. | the task touches foxglove/mcap. | — |
| writing | Technical writing | Medium posts, RELIABILITY.md-grade docs, case studies. | the task produces prose a stranger will read. | — |
| career | Career ops | CV, LinkedIn, applications, interviews, networking, checkpoints. | the task advances the job search rather than the codebase. | — |
| tooling | Internal tooling & DX | Building tools that make the rest of the work faster — this dashboard included. | the task improves your own workflow with software. | — |

## Checkpoint questions

| question | helper |
|---|---|
| interview-grade eval number? | Good = the exact sentence you'd say out loud, with real X and Y. Not yet = name the one missing piece. |
| chaos falsified anything? | Name the belief that died. Nothing surprised you = you tested too gently — schedule the harder rerun before answering. |
| 3 PRs merged or stale? | Status + date of last maintainer contact + your next move, per PR. |
| fifth repo started? | Yes/no. If yes: which one gets archived this week. mcap-tools was the sanctioned exception and is frozen. |
| robotics-infra vs AI-infra vs PM/founder — which leads Q1 applications? | Re-decide, don't re-litigate. One paragraph, then it drives B1. |

## Skill progress

| Skill | Now → Target | Evidence |
|---|---|---|
| ROS 2 | 0 → literate | Reads a public bag cold; the six-point "why robot data is hard" answer |
| MCAP | 0 → H | mcap-tools · synapse MCAP source · 3 merged PRs in the format's own repo |
| MQTT | 0 → M | QoS-1 duplicate-convergence demo; LWT-as-offline-detector unprompted |
| Go perf / pprof | M → H | Bottleneck fix, before/after p99 |
| Postgres internals | L → H | Pool, isolation, index w/ EXPLAIN |
| Chaos / load (k6) | 0 → H | RELIABILITY.md |
| Prometheus/Grafana/OTel | L → H | Gateway dashboards + traces · fleet dashboard |
| vLLM / serving | 0 → H | Bake-off + streaming through gateway |
| Fine-tuning / quantization | 0 → M | LoRA model on the cost curve, evaled |
| Terraform | 0 → M | Stack reproduced from zero |
| Kubernetes | L → M | kind deploy (B1) |
| pgvector / retrieval | L → M | Top-k in prod path |
| Evals | 0 → H | Harness + CI gate, four domains |
| AWS | L → certified | SAA (B1) |
| Security | L → M | OWASP pass, OAuth |

## Budget — €2,500 (unchanged; robotics workstreams cost ≈€0 — public datasets, free Foxglove tier)

| Item | € |
|---|---|
| O'Reilly annual | 500 |
| ByteByteGo | 250 |
| AWS SAA exam + prep | 350 |
| LLM API credits + GPU rental | 250 |
| Cloud Postgres sandbox | 300 |
| Confluent/Kafka sandbox | 250 |
| Conference ticket | 350 |
| Newsletters / communities | 150 |
| Udemy LLM course | 0 (company license) |
| Buffer | 100 |

## Guardrails

1. No new **portfolio** repo — one sanctioned exception: `mcap-tools` (W1, ≤1 weekend, <500 lines, frozen after W1; superseded by the synapse MCAP source). The dashboard stays a utility; `infra/` and `ml/` live inside synapsePlatform.
2. Every project ends in something a stranger can verify: a number, a merged PR, or a linked recording.
3. Write up each project when done.
4. Slip order: Terraform full stack → K8s → bake-off extras → MQTT. Never slip OSS or the W3 Rerun application.
5. Sunday rest is load-bearing.
6. **Robotics traps** (from the transition plan): never build a visualiser — Foxglove and Rerun exist; lead with *correctness under adversity*, not throughput; one pipeline, N sources — the abstraction is the argument.

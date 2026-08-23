# Master Plan v5 — Final, Executable, Tagged

Supersedes v4. Every bullet now starts with a project tag = which repo/context you open to do it. Tags flow into the dashboard seed as each task's `project` field.

**Tags:** `[synapse]` synapsePlatform · `[gateway]` LLMGateway-Go · `[dash]` dashboard app · `[oss]` your chosen external repo · `[learn]` courses/reading/certs · `[write]` Medium · `[career]` CV/LinkedIn/applications/networking · `[all repos]` synapse + gateway + dash

## Projects

| id | label |
|---|---|
| synapse | Synapse |
| gateway | LLM Gateway |
| dash | Dashboard |
| oss | Open source |
| learn | Learning |
| write | Writing |
| career | Career |
| all | All repos |

## North star

Backend → Platform/Distributed → **AI Infrastructure** → Founder/CTO.
Comp staging: now → €70–120k (platform) → €120–200k (AI infra) → equity upside.

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
Cadence: daily log → Sunday review → biweekly metrics delta → checkpoints Nov 9 + Feb 21 → reality check every 6 months.

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

## Goals (Kanban seed)

| ID | Goal | Project | Done means | Target |
|---|---|---|---|---|
| G0 | Dashboard live | dash | v1 on tailnet, seeded | Aug 31 |
| G1 | Eval harness | synapse | Rejection-rate number, 30-window golden set, 2 models, CI gate | W3 |
| G2 | Reliability proven | gateway | RELIABILITY.md: 5 experiments w/ graphs + 1 pprof fix | W7 |
| G3 | Postgres migration | synapse | Pool/isolation/index justified + honest concurrent number | W10 |
| G4 | pgvector search | synapse | Top-k semantic query live | W10 |
| G5 | K8s deploy | synapse | On kind: probes, HPA | P2-B1 |
| G6 | AWS SAA | learn | Passed | P2-B1 |
| G7 | OSS presence | oss | 2 PRs merged | W9 / P2 |
| G8 | Writing | write | 3 Medium posts | 2 by W7, #3 in B1 |
| G9 | Career round 1 | career | CV+LinkedIn rebuilt, 10 applications | W12 |
| G10 | Model serving | gateway | vLLM behind gateway: batching + streaming, bake-off | W8 |
| G11 | Fine-tune | synapse + gateway | LoRA + quantize + eval vs base on golden set | W11 |
| G12 | Kafka depth | synapse | DLQ, retries, backpressure, idempotency live | P2-B2 |
| G13 | Experiment infra | synapse | W&B/MLflow + nightly eval regression alert | P2-B1 |
| G14 | IaC | synapse (infra/) | Terraform: cloud PG (W9) → full stack (W12) | W12 |
| G15 | Security pass | all repos | OWASP review, gosec/govulncheck clean, OAuth | W12 / P2-B6 |
| G16 | Network | career | 1 conference attended | P2-B6 |
| G17 | Role | career | Platform/AI-infra offer signed | Q2 2027 |

---

# Phase 1 — 12 weeks (Aug 24 → Nov 15)

## W1 · Aug 24–30 — Golden set

- [ ] **[synapse] Golden set committed (then frozen forever)**
  1. Create `eval/golden/`.
  2. Write a tiny exporter (`cmd/goldenexport`) that dumps event windows from your dev DB to JSON, one file per window.
  3. Pick 16–26 real windows across Energy / Finance / Monitoring.
  4. Hand-write 4 nasty ones: `empty.json`, `single_event.json`, `one_domain_floods.json`, `same_timestamps.json`.
  5. Add `eval/golden/README.md`: one line per file saying what it tests. Commit.
  → **Done =** set is in git and treated as read-only from now on.
  → **Skills =** evals

- [ ] **[synapse] summary.v2 prompt in a versioned prompts dir**
  1. Create `prompts/`; move the current prompt to `prompts/summary.v1.txt`.
  2. Write `summary.v2.txt` with one deliberate change (e.g. stricter citation rules, tighter schema instructions). Note the diff in the file header.
  3. Make `SummaryPromptVersion` load by version from that dir via config.
  → **Done =** the pipeline runs with either version by flipping one config value.
  → **Skills =** evals

- [ ] **[oss] OSS target picked**
  1. Shortlist 3 Go projects you already use (OTel Collector, sqlc, Bubble Tea, Kafka Go client, Loki).
  2. For each: skim CONTRIBUTING.md + the last 10 merged PRs (size, review tone, response time).
  3. Pick one. Watch → Issues on GitHub. Bookmark 3 open issues touching code you understand.
  → **Done =** one repo chosen, 3 candidate issues bookmarked.
  → **Skills =** oss

- [ ] **[learn] Subscriptions + DDIA ch.1**
  1. Buy O'Reilly annual + ByteByteGo. Subscribe to 2 newsletters (Pragmatic Engineer + one AI-infra).
  2. Read DDIA ch.1; write 3 bullets in the daily log.
  → **Done =** receipts exist, notes exist.
  → **Skills =** distsys

- [ ] **[dash] Dashboard v1 (G0)**
  1. Execute `dashboard-plan.md` M0→M6 in order. Hard stop at 14h — whatever's missing goes to the stretch list.
  → **Done =** the 6 acceptance checks pass on your phone over Tailscale.
  → **Skills =** tooling

## W2 · Aug 31–Sep 6 — Runner + metrics

- [ ] **[synapse] Eval runner**
  1. Create `cmd/evalrun`: loop {v1, v2} × {model A, model B} × every golden file.
  2. Each run calls the existing summary pipeline; capture raw output + `ValidateAgainstEvidence` result.
  3. Write one row per prompt×model×window to `eval/results/<date>.json`.
  4. Add `make eval`.
  → **Done =** `make eval` produces a committed results file.
  → **Skills =** evals

- [ ] **[synapse] Core metrics + baseline logged**
  1. In the runner, compute per prompt×model: rejection rate, schema-validity % (parses as `EventSummary`), evidence-coverage %.
  2. Print a summary table at the end of the run.
  3. Commit the baseline results; log the headline numbers in the dashboard (📊).
  → **Done =** baseline numbers exist in git + dashboard.
  → **Skills =** evals

- [ ] **[learn] AWS SAA started**
  1. Buy one prep course (Cantrill or Maarek — pick in 10 min, don't research for a week).
  2. Do module 1 on Thursday; log it (📚).
  → **Done =** module 1 logged; exam window = late Nov (book in B1).
  → **Skills =** aws

- [ ] **[oss] Reproduce a real bug**
  1. From your 3 bookmarks, pick the one you can reproduce locally.
  2. Clone; write a minimal repro (failing test or 20-line `main.go`).
  3. Comment on the issue with the repro + "happy to fix if maintainers agree with direction".
  → **Done =** your repro comment is posted.
  → **Skills =** oss

## W3 · Sep 7–13 — Ship the number (G1)

- [ ] **[synapse] Stability + cost + latency metrics**
  1. Add `-runs=N` to the runner; run each window 5× at your production temperature.
  2. Compute drift: % of runs whose output differs field-by-field from run 1.
  3. Record tokens + latency per call; compute € per summary from provider pricing.
  → **Done =** results file has stability, cost, latency columns.
  → **Skills =** evals

- [ ] **[synapse] Regression gate in CI**
  1. Add a `make verify` step: run eval, fail if v2 rejection rate > committed v1 baseline.
  2. Add it as a GitHub Actions job.
  3. Prove it: break the prompt on a branch → CI red; revert → green.
  → **Done =** you watched CI fail on a bad prompt.
  → **Skills =** evals, tooling

- [ ] **[synapse] The sentence**
  1. Fill in real numbers: "v1→v2 dropped rejection from X% to Y% on a 30-window golden set, across 2 models."
  2. Put it at the top of the README + CV draft; log it (📊).
  → **Done =** sentence written with real X and Y.
  → **Skills =** evals, career

## W4 · Sep 14–20 — Write-up + observability + vLLM up

- [ ] **[write] Medium #1**
  1. Outline 5 sections: problem → golden set → metrics → the number → CI gate.
  2. Draft ~1200 words Sat/Sun; include the results table + 1 diagram.
  3. Publish; log it (✍️).
  → **Done =** live URL.
  → **Skills =** writing

- [ ] **[gateway] Prometheus + Grafana**
  1. Add a `promhttp` `/metrics` endpoint.
  2. Instrument: request-duration histogram (labels: route, provider), slots-in-use gauge, Redis-up gauge, JWKS-cache-age gauge.
  3. docker-compose: Prometheus (scraping gateway) + Grafana.
  4. Build one dashboard: latency p50/95/99, slots, dependency health. Export its JSON into the repo.
  → **Done =** send test load, watch p99 move on the board.
  → **Skills =** observability

- [ ] **[gateway] OTel tracing on the hot path**
  1. Add otel-go; spans for auth → ratelimit → slot acquire → provider call.
  2. Add Jaeger to docker-compose; export traces to it.
  3. Open one trace showing all 4 spans with timings; screenshot to `docs/`.
  → **Done =** screenshot saved.
  → **Skills =** observability

- [ ] **[gateway] vLLM serving a small model**
  1. No local GPU? Rent one (RunPod/Lambda, 24GB card, ~€0.3–0.7/h — comes out of the API-credit budget).
  2. `pip install vllm`; `vllm serve` a small instruct model (Qwen2.5-1.5B or Llama-3.2-3B class).
  3. `curl` the OpenAI-compatible `/v1/chat/completions`; confirm streamed tokens.
  4. Save the exact launch command + model name in `docs/`.
  → **Done =** curl returns a streamed completion.
  → **Skills =** serving

- [ ] **[oss] PR #1 + CV line**
  1. Implement the fix on a branch; add a test; follow the project's checklist (lint, commit format).
  2. Open the PR referencing the issue. Respond to any review within 24h.
  3. Add the eval-harness line to the CV.
  → **Done =** PR open with green CI.
  → **Skills =** oss

## W5 · Sep 21–27 — Chaos: fail-open / fail-static

- [ ] **[gateway] k6 mixed-load baseline**
  1. Write a k6 script (`load/`) with 2 scenarios sharing the gateway: fast client (high rps, short prompts) + slow client (low rps, long streaming), different API keys.
  2. Run 10 min; save Grafana snapshot of p50/95/99 + error rate to `docs/reliability/baseline.png`.
  → **Done =** baseline graphs saved.
  → **Skills =** chaos

- [ ] **[gateway] Kill Redis mid-load (fail-open)**
  1. Start mixed load; at minute 5, `docker stop redis`.
  2. Verify requests keep succeeding; capture the degradation gauge flipping + any error blip.
  3. `docker start redis`; verify limits re-engage.
  4. Save annotated graph.
  → **Done =** graph shows flat traffic through the outage.
  → **Skills =** chaos

- [ ] **[gateway] Kill JWKS mid-load (fail-static)**
  1. Point the gateway at a JWKS URL you control (static file server in compose).
  2. Stop it mid-load; verify cached keys keep validating.
  3. Verify the staleness metric is visible and rising the whole time.
  4. Save annotated graph.
  → **Done =** graph + staleness evidence in `docs/reliability/`.
  → **Skills =** chaos, security

## W6 · Sep 28–Oct 4 — Chaos: slots + failover → vLLM

- [ ] **[gateway] Slot starvation**
  1. Tune the slow-client scenario to hold streaming requests until every slot is taken (slot gauge at max).
  2. Measure the fast client during starvation: queue time, 429s, latency.
  3. Write 5 lines: what slots showed that rate limiting alone never would.
  → **Done =** starvation graph + notes committed.
  → **Skills =** chaos

- [ ] **[gateway] vLLM wired as a provider**
  1. Add a provider config entry for the vLLM OpenAI-compatible endpoint; alias it `local-small`.
  2. Send one request through the gateway to it.
  3. Confirm metrics/traces carry `provider=vllm`.
  → **Done =** proxied completion visible in Grafana.
  → **Skills =** serving

- [ ] **[gateway] Failover lands on vLLM**
  1. Priority: primary = hosted API, fallback = `local-small`.
  2. Under load, break the primary (invalid key or null-route its host).
  3. From traces, measure failover latency (last primary failure → first fallback success).
  4. Assert the response names the model that actually answered.
  → **Done =** failover latency number + model-attribution proof saved.
  → **Skills =** chaos, serving

## W7 · Oct 5–11 — Profile + RELIABILITY.md (G2)

- [ ] **[gateway] pprof: find and fix one bottleneck**
  1. Under load, capture 30s CPU + heap profiles.
  2. Pick the top hot frame in *your* code; write a one-line hypothesis.
  3. Fix (buffer reuse, fewer allocs, avoid re-marshal — whatever the profile says).
  4. Prove: `go test -bench` before/after + k6 p50/95/99 before/after.
  → **Done =** before/after table committed.
  → **Skills =** go-perf

- [ ] **[gateway] RELIABILITY.md**
  1. One section per experiment (Redis, JWKS, slots, failover, pprof): claim → method → graph → result.
  2. Add a "what surprised me" line per section. Nothing surprised you? Re-run harder.
  → **Done =** merged with 5 graphs.
  → **Skills =** chaos, writing

- [ ] **[write] Medium #2 + honest CV numbers**
  1. Post: "I chaos-tested my own README" — compress RELIABILITY.md.
  2. Replace "~750 msg/s local" on the CV with 2–3 measured numbers (p99 under mixed load, failover latency).
  → **Done =** live URL + CV updated.
  → **Skills =** writing

## W8 · Oct 12–18 — Serving week (G10)

- [ ] **[gateway] Batching + token streaming**
  1. Fire 20 concurrent requests at vLLM; record throughput vs sequential to see continuous batching work.
  2. Verify SSE pass-through in the gateway: flush per chunk, zero buffering.
  3. Add a TTFT (time-to-first-token) metric.
  → **Done =** `curl -N` through the gateway streams; TTFT on the dashboard.
  → **Skills =** serving

- [ ] **[gateway] Bake-off table**
  1. Pick 3–4 configs: small model FP16 · same model AWQ-quantized · hosted API model · (optional) one size up.
  2. Run the identical k6 profile 10 min per config.
  3. Record per config: p50/95/99, TTFT, tokens/s, € per 1k requests (GPU-hour or API price).
  4. Commit `docs/serving-bakeoff.md`.
  → **Done =** table with ≥3 fully measured rows.
  → **Skills =** serving
  *(Overflow rule: table may finish in W9 Mon/Thu slots.)*

## W9 · Oct 19–25 — Postgres, provisioned by Terraform

- [ ] **[synapse] Terraform module #1: cloud Postgres**
  1. Create `infra/` **inside synapsePlatform** (no fourth repo) with pinned provider versions. Neon (community provider) or RDS (official AWS provider) — pick one, don't agonize.
  2. Write the module: instance/project, database, role; connection string as output. Secrets via variables, `.tfvars` gitignored.
  3. `terraform apply`; connect with `psql` using the output.
  4. Prove reproducibility: `terraform destroy && terraform apply`, connect again.
  → **Done =** DB rebuilt from zero by two commands.
  → **Skills =** terraform

- [ ] **[synapse] Postgres behind sqlc + pool + migrations**
  1. Add pgx/v5 + pgxpool. Set `MaxConns` explicitly; write the justification as a comment (start: 2×cores or provider conn cap, whichever is lower).
  2. Switch sqlc engine to postgres; regenerate; fix dialect breakage.
  3. Init goose; schema → `001_init.sql`; wire `make migrate`.
  4. Run the full test suite against the cloud DB.
  → **Done =** tests green on Postgres; pool reasoning written down.
  → **Skills =** postgres

- [ ] **[oss] Follow-up**
  1. PR #1 unmerged? Ping politely; offer to split or shrink it.
  2. Bookmark issue #2 in the same repo.
  → **Done =** PR moved or ping sent; issue #2 identified.
  → **Skills =** oss

## W10 · Oct 26–Nov 1 — Postgres depth + pgvector (G3, G4)

- [ ] **[synapse] EXPLAIN → index → EXPLAIN**
  1. Enable `pg_stat_statements` (available on Neon/RDS); find the slowest hot-path query.
  2. `EXPLAIN (ANALYZE, BUFFERS)`; save output.
  3. Add the index it asks for via a goose migration; re-run; save the after.
  → **Done =** before/after plans committed with the timing delta.
  → **Skills =** postgres

- [ ] **[synapse] Isolation level, chosen on purpose**
  1. Read Read Committed vs Repeatable Read vs Serializable against your ingestion path's actual anomalies.
  2. Set the chosen level explicitly in the ingestion transaction; write a 10-line why in docs.
  3. Add one test demonstrating the anomaly you're preventing (or knowingly accepting).
  → **Done =** explicit level in code + written justification + test.
  → **Skills =** postgres

- [ ] **[synapse] The honest number**
  1. Re-run the W5 load profile with N concurrent writers against Postgres.
  2. Publish it in the README next to the retired 750, with one sentence of context. Log it (📊).
  → **Done =** new number public, even if it's worse.
  → **Skills =** postgres, chaos

- [ ] **[synapse] pgvector top-k**
  1. `CREATE EXTENSION vector` via migration; add an embeddings column (dim = your embedding model's).
  2. Batch-embed existing summaries through the gateway; store.
  3. Top-k query (`ORDER BY embedding <=> $1 LIMIT k`) behind a Go function + one endpoint.
  4. Add an HNSW index; measure query latency before/after.
  → **Done =** endpoint returns sane neighbors; latency recorded.
  → **Skills =** retrieval

- [ ] **[learn] Postgres internals reading**
  1. Read: autovacuum docs, one BRIN/GIN article, partitioning overview.
  2. Write 5 bullets: which of these synapsePlatform will need, and when.
  → **Done =** notes in the log.
  → **Skills =** postgres

## W11 · Nov 2–8 — Fine-tune week (G11)

- [ ] **[synapse] Dataset curated**
  1. Export input→accepted-summary pairs from your domains into `ml/data/`; target 500–2,000 examples.
  2. Dedupe, strip anything sensitive, format to instruct JSONL; split 90/10 train/val.
  3. Freeze val. Never train on the golden set — that's the test.
  → **Done =** `train.jsonl` + `val.jsonl` with counts recorded.
  → **Skills =** finetuning

- [ ] **[synapse] LoRA fine-tune → quantize**
  1. Rent the same GPU class as W4; base model = the one you've been serving.
  2. LoRA via Axolotl or Unsloth: 1–3 epochs; watch the loss curve, stop when val flattens.
  3. Merge the adapter; quantize to AWQ (vLLM-friendly; GGUF only if you also want llama.cpp).
  4. Save artifacts + exact configs to `ml/models/` registry notes.
  → **Done =** quantized model + training config saved.
  → **Skills =** finetuning

- [ ] **[gateway + synapse] Serve tuned model, eval vs base**
  1. [gateway] `vllm serve` the tuned model; add provider `local-tuned`.
  2. [synapse] Run the W2 eval runner: base vs tuned on the golden set (rejection, coverage, stability) + cost/latency via the bake-off harness.
  3. Write the true verdict: "tuned = X% rejection at Y% of the cost" — whatever the numbers say.
  → **Done =** eval table committed; verdict logged (📊).
  → **Skills =** finetuning, serving, evals

- [ ] **[learn] SAA practice exam #1**
  1. One full timed practice exam Thursday.
  2. Log the score; list 3 weakest domains; queue those modules.
  → **Done =** score logged.
  → **Skills =** aws

## W12 · Nov 9–15 — Checkpoint + Terraform stack + career (G9, G14)

- [ ] **[dash] Checkpoint (Mon Nov 9, 45 min)**
  1. In the dashboard Review, answer in writing: interview-grade eval number? chaos falsified anything? PR merged or stale? fourth repo? still AI-infra path?
  2. Any bad answer → adjust B1 scope immediately.
  → **Done =** written answers exist.
  → **Skills =** career

- [ ] **[synapse] Terraform full stack (infra/)**
  1. Add a module for one small VM (Hetzner/EC2); cloud-init installs Docker + pulls your compose file (gateway, synapse services, Prometheus, Grafana).
  2. Wire the W9 PG outputs into the app config.
  3. `terraform destroy && apply` on a scratch env; run a smoke-test script against it.
  4. `infra/README.md`: the one-command bring-up.
  → **Done =** fresh environment reachable end-to-end from zero.
  → **Skills =** terraform

- [ ] **[all repos] Security pass**
  1. `gosec ./...` + `govulncheck ./...` on synapse, gateway, dash; fix or explicitly triage every finding.
  2. Trufflehog scan of git history; rotate anything it finds.
  3. 30-min OWASP top-10 walk of the gateway: authz on every route, input size limits, no error leakage.
  → **Done =** `docs/security.md` with clean-or-triaged reports.
  → **Skills =** security

- [ ] **[all repos] Polish + conference**
  1. Each repo README top: what it is → the numbers → one diagram → quickstart.
  2. Pin the 3 repos; profile README with the 3 headline numbers.
  3. [career] Book the conference ticket.
  → **Done =** a stranger gets the story in 2 minutes per repo; ticket booked.
  → **Skills =** career, writing

- [ ] **[career] CV + 10 applications**
  1. Rewrite CV to 1 page: 3 numbers, OSS line, serving + fine-tune line.
  2. Update LinkedIn headline; feature the 2 posts.
  3. List 15 target companies (AI infra / LLM infra / Go platform); tailor and send 10; log each (📮).
  → **Done =** 10 applications logged in the dashboard.
  → **Skills =** career

*Slip order if the week breaks: Terraform full stack → B1.*

---

# Phase 2 — 2-week blocks (Nov 16 → Feb 21, 2027)

*Re-scope at the Nov 9 checkpoint. Steps kept lighter on purpose.*

## B1 · Nov 16–29

- [ ] **[learn] Sit the SAA (G6):** 1. practice exam #2 → 2. book + sit → 3. log (🎓). **Done =** pass.
  → **Skills =** aws
- [ ] **[synapse] K8s on kind (G5):** 1. `kind create cluster` → 2. Deployments + Services + liveness/readiness probes in `deploy/` → 3. HPA on CPU + load until it scales. **Done =** you watched a scaling event.
  → **Skills =** k8s
- [ ] **[synapse] Eval regression alerts (G13):** 1. pick W&B (free tier) or MLflow → 2. log every eval run (params + metrics) → 3. nightly GitHub Action runs eval, alerts on regression → 4. test the alert once on purpose. **Done =** one real alert received.
  → **Skills =** evals, observability
- [ ] **[write] Medium #3:** the W11 table becomes "fine-tuning vs prompting, measured." **Done =** live URL (✍️).
  → **Skills =** writing
- [ ] **[learn]** Udemy LLM course starts in the Thursday slot.
  → **Skills =** serving

## B2 · Nov 30–Dec 13 — Kafka depth (G12)

- [ ] **[synapse] DLQ + retries:** 1. create `<topic>.dlq` → 2. consumer: N retries with backoff, then publish to DLQ with error headers → 3. small CLI to inspect + replay. **Done =** a poison message lands in DLQ and replays clean.
  → **Skills =** distsys
- [ ] **[synapse] Idempotency:** 1. event UUID + unique constraint / processed table → 2. test: duplicate delivery causes exactly one effect. **Done =** test proves it.
  → **Skills =** distsys
- [ ] **[synapse] Backpressure:** 1. bound in-flight work (worker pool; pause/resume consumer on depth) → 2. slow the DB on purpose: lag grows, memory stays flat. **Done =** the graph shows exactly that.
  → **Skills =** distsys

## B3 · Dec 14–27

- [ ] **[gateway] Result caching:** Redis cache keyed by the existing cache key, TTL configurable; hit-rate metric; tune >60% on replayed traffic. **Done =** panel shows >60%.
  → **Skills =** serving
- [ ] **[gateway + synapse] Hardening:** timeouts on every outbound call; retry+jitter where idempotent; cost budget alert. **Done =** grep finds no call without a deadline.
  → **Skills =** distsys
- [ ] **[gateway] Per-key quotas:** quota table + middleware + 429 with headers. **Done =** k6 proves the cap.
  → **Skills =** security

## B4 · Dec 28–Jan 10 (holiday-light)

- [ ] **[learn]** Udemy modules + finish DDIA + buffer. Rest counts as progress this block.
  → **Skills =** serving, distsys

## B5 · Jan 11–24

- [ ] **[gateway] Per-tenant cost dashboard:** tokens × price per key → Grafana panel. **Done =** top-3 spenders visible.
  → **Skills =** observability
- [ ] **[learn] GPU basics:** read one serving-GPU primer; profile vLLM GPU util during a bake-off rerun. **Done =** 5-bullet notes.
  → **Skills =** serving
- [ ] **[learn]** System-design drills ×2 (multi-region inference).
  → **Skills =** distsys

## B6 · Jan 25–Feb 7

- [ ] **[oss] PR #2 (G7):** implement → open → shepherd to merge. **Done =** merged, because one can be luck.
  → **Skills =** oss
- [ ] **[gateway] OAuth/OIDC (G15):** client-credentials flow via a Keycloak container; retire static-key path behind a flag; OWASP recheck. **Done =** token-based auth works end to end.
  → **Skills =** security
- [ ] **[career] Conference (G16):** goal = 5 real conversations, 3 follow-ups within 48h (🤝).
  → **Skills =** career

## B7 · Feb 8–21

- [ ] **[career]** 10 more applications (📮) · 2 mock interviews.
  → **Skills =** career
- [ ] **[synapse]** Tag v0.3 + **[write]** case-study post.
  → **Skills =** writing
- [ ] **[dash] Checkpoint Feb 21:** written review, same 5-question format.
  → **Skills =** career

---

# Phase 3 — Quarters (2027)

**Q2:** interview loop → land platform/AI-infra role (G17); keep 1 serving-depth block/week. **Q3:** on-the-job scale, GPU depth, pprof/eBPF habit. **Q4:** 1 post/quarter, mentoring, evaluate €120–200k move. **2028+:** AI Infra → staff scope → founder/CTO review. *(Expanded into blocks at the Feb 21 checkpoint.)*

## Metrics targets

| Metric | Baseline | Target | Slug | Unit | Definition | How to measure |
|---|---|---|---|---|---|---|
| Rejection rate v2 vs v1 | — | measurably ↓, CI-gated | rejection_rate | % | Share of golden-set summaries ValidateAgainstEvidence rejects | make eval over the frozen 30-window set; record per prompt × model |
| p95 latency (cached) | >1s | <0.8s | p95_latency | s | 95th-percentile request latency under the standard mixed profile | Grafana after a 10-min k6 run; never compare across different profiles |
| p99 latency | >2s | <2s | p99_latency | s | 99th-percentile request latency under the standard mixed profile | Same run as p95 — read both off one k6 pass |
| Cache hit rate | <20% | >60% | cache_hit_rate | % | hits ÷ (hits + misses) | Gateway cache metric over a 10-min replayed-traffic window |
| Cost per 1k queries | — | −30% | cost_per_1k | € | Euro cost per 1,000 requests | API pricing × tokens, or GPU-hour ÷ requests served — say which in the note |
| Fine-tuned vs base on golden set | — | ≥ baseline at lower cost | tuned_vs_base | ratio | Tuned ÷ base rejection rate on the golden set | Eval runner, both providers, same day |
| Throughput, concurrent writers | "750" (SQLite, misleading) | real measured number | throughput_concurrent | msg/s | Messages per second with N concurrent writers on Postgres | Rerun the W5 load profile; put N in the note |
| OSS PRs merged | 0 | 2 | oss_prs_merged | PRs | PRs you opened that a maintainer merged | Count from your GitHub PR list; merged only — open or closed-unmerged do not count |
| Applications sent | 0 | 20 | applications_sent | applications | Applications actually submitted | Count the 📮 entries in the Log for the period; the Log is the record, this is the roll-up |
| Medium posts | — | 2 by W7 · 3 by B1 · 5 by Feb | medium_posts | posts | Posts published, not drafted | Count live URLs; a draft is a Learned bullet, not a metric |
| Time to first token | — | tracked from W8 | ttft | s | Request to first streamed token through the gateway | TTFT metric during the standard k6 profile |

## Skills

| code | name | description | associate when | target |
|---|---|---|---|---|
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
| oss | Open-source collaboration | Working in codebases you don't own: repros, PRs, review etiquette, maintainer time. | the task touches the chosen external repo. | — |
| writing | Technical writing | Medium posts, RELIABILITY.md-grade docs, case studies. | the task produces prose a stranger will read. | — |
| career | Career ops | CV, LinkedIn, applications, interviews, networking, checkpoints. | the task advances the job search rather than the codebase. | — |
| tooling | Internal tooling & DX | Building tools that make the rest of the work faster — this dashboard included. | the task improves your own workflow with software. | — |

## Checkpoint questions

| question | helper |
|---|---|
| interview-grade eval number? | Good = the exact sentence you'd say out loud, with real X and Y. Not yet = name the one missing piece. |
| chaos falsified anything? | Name the belief that died. Nothing surprised you = you tested too gently — schedule the harder rerun before answering. |
| PR merged or stale? | Status + date of last maintainer contact + your next move. |
| fourth repo? | Yes/no. If yes: which one gets archived this week. |
| still AI-infra path? | Gut check, one paragraph max. Re-decide, don't re-litigate. |

## Budget — €2,500

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

1. No fourth **portfolio** repo. The dashboard is a utility; `infra/` and `ml/` live inside synapsePlatform.
2. Every project ends in something a stranger can verify: a number or a merged PR.
3. Write up each project when done.
4. If a week slips, slip in this order: Terraform full stack → K8s → bake-off extras. Never slip OSS.
5. Sunday rest is load-bearing.

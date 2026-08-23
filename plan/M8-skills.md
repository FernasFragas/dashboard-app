# M8 · Skills — every task builds a named skill

**Goal:** a skill profile derived from the work itself. Every task links to ≥1 skill; every skill has a description precise enough that choosing the right one is obvious; the Skills tab shows where you're actually growing, with the evidence trail an interviewer would ask for.
**Depends on:** M7 shipped (tracker first, profile second). M9 (gamification) builds on this — M8 defines associations, M9 layers XP/levels on them.
**Sequencing:** post-v1. Not started before M7 ships.

## Data model (change `../docs/DATABASE.md` first, then migration)

```
skills        id INTEGER PK · code TEXT UNIQUE · name · description
              · associate_when TEXT · target_tier TEXT NULL · sort_order · created_at
task_skills   task_id FK · skill_id FK · PRIMARY KEY (task_id, skill_id)
              · ON DELETE CASCADE both ways
log_skills    log_id FK · skill_id FK · PRIMARY KEY (log_id, skill_id)   -- optional links
```

- **No stored progress.** Per-skill stats (tasks done/total, evidence count, last activity) are derived at read time — nothing to drift. Record as **ADR-007**.
- **Invariant: every task has ≥1 skill.** SQLite can't enforce across tables, so: (a) POST/PATCH task rejects empty `skill_ids` with 422; (b) a store test walks the seed and fails if any seeded task lacks skills; (c) any pre-M8 task without links renders a "needs skill" badge with one-tap assign.
- Seeding follows v3's model: additive upsert by `code` (skills) and existing `seed_key` (task links); re-seed replaces links for seeded tasks only, never user-created ones.

## Seeded skills (17) — code · name · what it represents · associate when

| code | name | Represents / associate tasks that… |
|---|---|---|
| evals | Evaluation systems | Measuring LLM output quality: golden sets, rejection/coverage/stability metrics, regression gates. …create, run, or gate on an evaluation. |
| go-perf | Go performance | pprof, allocation discipline, benchmarks, hot-path fixes. …profile, optimize, or benchmark Go code. |
| chaos | Load & chaos testing | k6/vegeta, dependency-kill experiments, proving failure modes. …generate load or break something on purpose to observe behavior. |
| observability | Observability | Prometheus, Grafana, OTel traces, dashboards, alerting. …add or read metrics, traces, or dashboards. |
| serving | LLM serving | vLLM, batching, token streaming, TTFT, provider wiring, bake-offs. …run, route to, or measure a model server. |
| finetuning | Fine-tuning & quantization | Dataset curation, LoRA, AWQ/GGUF, tuned-vs-base verdicts. …prepare data for, train, or compress a model. |
| postgres | Postgres internals | pgxpool, isolation levels, EXPLAIN, indexes, migrations, vacuum. …touch the relational layer below the ORM line. |
| retrieval | Retrieval & pgvector | Embeddings, top-k search, HNSW, retrieval latency. …store or query vectors. |
| distsys | Distributed systems | DDIA theory, Kafka: DLQ, retries, backpressure, idempotency, consistency. …deal with partial failure between processes. |
| terraform | Infrastructure as code | Terraform modules, reproducible environments, destroy/apply proofs. …provision anything by code instead of clicks. |
| k8s | Kubernetes | kind, Deployments, probes, HPA, manifests. …run workloads on a cluster. |
| aws | AWS | SAA prep and exam, service architecture, IAM. …study or apply AWS specifics. |
| security | Security engineering | gosec/govulncheck, secrets hygiene, OWASP, OAuth/OIDC. …find, fix, or prevent a vulnerability class. |
| oss | Open-source collaboration | Working in codebases you don't own: repros, PRs, review etiquette, maintainer time. …touch the chosen external repo. |
| writing | Technical writing | Medium posts, RELIABILITY.md-grade docs, case studies. …produce prose a stranger will read. |
| career | Career ops | CV, LinkedIn, applications, interviews, networking, checkpoints. …advance the job search rather than the codebase. |
| tooling | Internal tooling & DX | Building tools that make the rest of the work faster — this dashboard included. …improve your own workflow with software. |

`target_tier` is seeded from master-plan-v5's Skills tracker targets (H → Expert, M → Practitioner, certified → Expert) so each skill card can show *current vs target*.

## Task → skill seed mapping (transcribe into `seed.json`; validator enforces completeness)

| Week | task → skills |
|---|---|
| W1 | golden set → evals · summary.v2 → evals · OSS pick → oss · subs/DDIA → distsys · dashboard v1 → tooling |
| W2 | runner → evals · metrics+baseline → evals · SAA start → aws · bug repro → oss |
| W3 | stability/cost → evals · CI gate → evals, tooling · the sentence → evals, career |
| W4 | Medium #1 → writing · Prom+Grafana → observability · OTel → observability · vLLM up → serving · PR #1 → oss |
| W5 | k6 baseline → chaos · kill Redis → chaos · kill JWKS → chaos, security |
| W6 | slot starvation → chaos · wire vLLM → serving · failover → chaos, serving |
| W7 | pprof fix → go-perf · RELIABILITY.md → chaos, writing · Medium #2 → writing |
| W8 | batching+streaming → serving · bake-off → serving |
| W9 | TF module → terraform · PG+sqlc+pool → postgres · OSS follow-up → oss |
| W10 | EXPLAIN/index → postgres · isolation → postgres · honest number → postgres, chaos · pgvector → retrieval · internals reading → postgres |
| W11 | dataset → finetuning · LoRA+quantize → finetuning · serve+eval → finetuning, serving, evals · SAA practice → aws |
| W12 | checkpoint → career · TF full stack → terraform · security pass → security · polish → career, writing · CV+apps → career |
| B1 | SAA exam → aws · kind deploy → k8s · eval alerts → evals, observability · Medium #3 → writing · Udemy → serving |
| B2 | DLQ/idempotency/backpressure → distsys |
| B3 | caching → serving · hardening → distsys · quotas → security |
| B4 | Udemy/DDIA → serving, distsys |
| B5 | cost dashboard → observability · GPU basics → serving · drills → distsys |
| B6 | PR #2 → oss · OAuth → security · conference → career |
| B7 | apps/mocks → career · v0.3+post → writing · checkpoint → career |

## API (update `../docs/API.md`)

| Method + path | Purpose |
|---|---|
| GET /api/skills | list + derived stats: tasks done/total, evidence count, last activity |
| GET /api/skills/{id} | description, associate_when, target tier, linked tasks by week, linked logs |
| POST /api/tasks · PATCH /api/tasks/{id} | now take `skill_ids: []` — 422 if empty; PATCH respects If-Match |
| POST /api/logs | optional `skill_ids: []` |

## UX

1. **Skills tab** (5th bottom tab): card grid — name, done/total bar, target-tier chip, last-activity date. Tap → detail: description + "associate when" line at top (the affordance for correct tagging), linked tasks grouped by week, evidence list (linked logs, numbers).
2. **Task rows** (Today): small colored skill dots; tap task → detail sheet shows chips, editable via multi-select where **each option renders its associate_when line** — the description does the deciding.
3. **Add-task sheet**: skill multi-select is required; save disabled at zero selections with helper `Every task builds at least one skill — pick what this work trains.`
4. **Quick-log sheet**: optional skill chips row (defaults empty; one tap to attach evidence to a skill).
5. Copy lives in `web/src/copy/skills.ts`, same rule as M6+.

## Doc updates

`DATABASE.md` (three tables + derived-stats note) → migration; `API.md` endpoints; **ADR-007 · Skill stats are derived, never stored** — progress computed from associations at read time; the only stored facts are the links themselves.

## Acceptance

1. Seed boots with 17 skills, every seeded task linked per the mapping; validator test proves zero unlinked tasks.
2. Creating a task without a skill fails with 422 and an explanatory message; UI blocks it first.
3. Skills tab shows derived done/total per skill; completing a task updates its skills' cards on next fetch.
4. Skill detail shows description, associate_when, tasks by week, and linked-log evidence.
5. A log entry can be attached to a skill and appears in that skill's evidence list.
6. `DATABASE.md`, `API.md`, ADR-007 updated in the same change as the migration.

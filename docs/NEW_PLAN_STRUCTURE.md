# New Plan Structure

Use this as the starting shape for a new plan markdown file. Save it as something like
`my-plan.md`, then validate it before loading it.

```sh
go run ./cmd/server plan validate /path/to/my-plan.md
```

## Required Shape

The app can load a minimal plan with only projects, skills, and weeks. Add goals, categories,
metrics, and checkpoints when you need those screens populated.

```markdown
---
plan:
  id: my-plan
  name: "My Plan"
  start_year: 2026
  active_goal_limit: 3
---

# My Plan

## Projects

| id | label |
|---|---|
| ops | Operations |
| learn | Learning |

## Skills

| code | name | description | associate when | target |
|---|---|---|---|---|
| observability | Observability | Metrics, traces, dashboards, alerts. | the task adds or reads telemetry. | Practitioner |
| writing | Writing | Clear technical writing and publishing. | the task produces public or private writing. | Practitioner |

## Goals

| ID | Goal | Project | Done means | Target |
|---|---|---|---|---|
| G1 | Alerting live | ops | alert fires in staging | W1 |

## Log categories

| id | label | icon |
|---|---|---|
| shipped | Shipped | ok |
| learned | Learned | book |

# Phase 1

## W1 · Mar 1-7 — First week

- [ ] **[ops] Wire alert**
  1. Add the metric.
  2. Create the alert rule.
  -> **Done =** alert fires in staging.
  -> **Skills =** observability

- [ ] **[learn] Write the rollout note**
  -> **Done =** note is published.
  -> **Skills =** writing

## Metrics targets

| Metric | Baseline | Target | Slug | Unit | Definition | How to measure |
|---|---|---|---|---|---|---|
| p95 latency | >1s | <800ms | p95_latency | ms | 95th-percentile request latency. | Read the standard load-test dashboard. |

## Checkpoint questions

| question | helper |
|---|---|
| Are we still on track? | One paragraph; decide the next adjustment. |
```

## Rules

- `plan.id` should be stable. Changing it means the app sees a different plan.
- Project tags in tasks, like `[ops]`, must match `## Projects` ids.
- Skill codes in `Skills =` must match `## Skills` codes.
- If you include a `## Skills` section, every task must have a `Skills =` line.
- Week headings use `W1`, `W2`, etc. Blocks can use `B1`, `B2`, etc.
- A task title containing `Checkpoint` creates a checkpoint for that week.
- Metric rows must include both `Definition` and `How to measure`.

## Load It

For local testing with a new database:

```sh
tmpdir="$(mktemp -d)"
go run ./cmd/server -data "$tmpdir" -plan /path/to/my-plan.md
```

For the browser flow, open `/plan`, paste or choose the markdown file, preview it, then apply it.

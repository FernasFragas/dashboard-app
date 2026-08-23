# Plan Format

The dashboard can load any one local markdown plan:

```sh
dashboard -plan ~/plans/my-plan.md
dashboard plan validate ~/plans/my-plan.md
```

`plan.yaml` is optional. When present beside the markdown file, it sets the plan identity and a
small parser profile.

For browser upload, put the same profile at the top of the markdown as YAML front matter. Front
matter wins over `plan.yaml` when both are present.

## Markdown Sections

All data sections are optional. A plan with no weeks, skills, metrics, or checkpoints still
loads; the app will show empty lists where there is no plan data.

### Projects

```markdown
## Projects

| id | label |
|---|---|
| ops | Operations |
| learn | Learning |
```

Task tags and goal projects must match these ids. If the section is missing, projects are
derived from task tags.

### Goals

```markdown
## Goals

| ID | Goal | Project | Done means | Target |
|---|---|---|---|---|
| G1 | Alerting live | ops | alert fires in staging | S1 |
```

### Weeks And Tasks

Default week heading:

```markdown
## W1 · Mar 1-7 — First week
```

Default task shape:

```markdown
- [ ] **[ops] Wire alert**
  1. Add the metric.
  → **Done =** alert fires in staging.
  → **Skills =** observability
```

If the plan defines a `## Skills` catalogue, every task must have a `Skills` marker and every
listed code must exist. If the plan has no skills catalogue, task skill markers are omitted.

### Skills

```markdown
## Skills

| code | name | description | associate when | target |
|---|---|---|---|---|
| observability | Observability | Metrics, traces, dashboards, alerts. | the task adds or reads telemetry. | Practitioner |
```

`target` may be blank or `—`. Non-empty target values become the `skill_tiers` vocabulary.

### Log Categories

```markdown
## Log categories

| id | label | icon |
|---|---|---|
| shipped | Shipped | ✅ |
| learned | Learned | 📚 |
```

### Metrics

```markdown
## Metrics targets

| Metric | Baseline | Target | Slug | Unit | Definition | How to measure |
|---|---|---|---|---|---|---|
| p95 latency | >1s | <800ms | p95_latency | ms | 95th-percentile request latency. | Read the standard load-test dashboard. |
```

Metric rows must include `Definition` and `How to measure`.

### Checkpoints

Any task whose title contains `Checkpoint` creates a checkpoint for that week. The question
table supplies the prompts:

```markdown
## Checkpoint questions

| question | helper |
|---|---|
| still on track? | One paragraph; decide, do not re-litigate. |
```

## plan.yaml

Defaults match `master-plan-v5.md`, so no config is needed for that shape.

```yaml
plan:
  id: my-plan
  name: "My Plan"
  start_year: 2026
  active_goal_limit: 3

sections:
  projects: "Projects"
  goals: "Goals"
  skills: "Skills"
  rhythm: "Operating system"
  categories: "Log categories"
  metrics: "Metrics targets"
  checkpoints: "Checkpoint questions"

markers:
  done: "Done"
  skills: "Skills"

week_heading:
  pattern: '^##\s+(?P<code>[WB]\d+)\s*·\s*(?P<dates>.+?)(?:\s+—\s+(?P<focus>.*))?$'
  date_format: "Mon D-D"

task_bullet:
  pattern: '^-\s+\[[ x]\]\s+\*\*(?P<bold>.+?)\*\*(?P<trailing>.*)$'
  tag_pattern: '^\[([^\]]+)\]\s*(.*)$'
```

Only `week_heading.pattern` and `task_bullet.pattern` are regex escape hatches. The parser stays
strict inside week sections: an unparsed line is an error.

## Front Matter

Use front matter when the plan must travel as one file through the browser:

```markdown
---
plan:
  id: my-plan
  name: "My Plan"
  start_year: 2027
  active_goal_limit: 3
markers:
  done: "Done"
  skills: "Skills"
---

# My Plan
```

The keys are the same as `plan.yaml`. Front matter is optional; `master-plan-v5.md` uploads
unchanged without it.

## Minimal Complete Example

```markdown
# My Plan

## Projects

| id | label |
|---|---|
| ops | Operations |

## Skills

| code | name | description | associate when | target |
|---|---|---|---|---|
| observability | Observability | Metrics and alerts. | the task adds telemetry. | Practitioner |

# Phase 1

## W1 · Mar 1-7 — Launch

- [ ] **[ops] Wire alert**
  → **Done =** alert fires in staging.
  → **Skills =** observability
```

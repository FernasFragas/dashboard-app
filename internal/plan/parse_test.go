package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A miniature plan exercising every shape the real document uses.
const samplePlan = `# Master Plan

## Projects

| id | label |
|---|---|
| synapse | Synapse |
| gateway | LLM Gateway |
| dash | Dashboard |
| oss | Open source |
| learn | Learning |
| career | Career |
| all | All repos |

## Operating system

| Day | Slot |
|---|---|
| Mon | Theory: DDIA ~1 ch (1-2h) |
| Tue-Wed | Project work (2-3h) |
| Sun | Review, 3-bullet log. Rest. |

## Log categories

| id | label | icon |
|---|---|---|
| application | Application | app |
| module | Course module | mod |
| portfolio | Portfolio | port |
| post | Medium post | post |
| oss | OSS event | oss |
| number | Benchmark/number | num |
| network | Networking | net |
| exam | Exam/cert | exam |

## Goals (Kanban seed)

| ID | Goal | Project | Done means | Target |
|---|---|---|---|---|
| G0 | Dashboard live | dash | v1 on tailnet | Aug 31 |
| G6 | AWS SAA | learn | Passed | P2-B1 |
| G11 | Fine-tune | synapse + gateway | LoRA evaled | W11 |
| G14 | IaC | synapse (infra/) | Terraform | W12 |
| G15 | Security pass | all repos | OWASP review | W12 / P2-B6 |
| G17 | Role | career | Offer signed | Q2 2027 |

## Skills

| code | name | description | associate when | target |
|---|---|---|---|---|
| evals | Evaluation systems | Golden sets and gates. | the task creates or runs an evaluation. | Expert |
| oss | Open-source collaboration | Repros and PRs. | the task touches the external repo. | - |
| tooling | Internal tooling | Tools for your own workflow. | the task improves the dashboard or local tooling. | - |
| aws | AWS | SAA prep and services. | the task studies AWS. | Expert |
| career | Career ops | CV, applications and checkpoints. | the task advances the job search. | - |

---

# Phase 1 - 12 weeks

## W1 · Aug 24-30 — Golden set

- [ ] **[synapse] Golden set committed (then frozen forever)**
  1. Create eval/golden/.
  2. Pick 16-26 real windows.
  → **Done =** set is in git.
  → **Skills =** evals

- [ ] **[oss] OSS target picked**
  1. Shortlist 3 Go projects.
  → **Done =** one repo chosen.
  → **Skills =** oss

## W2 · Aug 31-Sep 6 — Runner

- [ ] **[synapse] Eval runner**
  1. Create cmd/evalrun.
  → **Done =** make eval produces a results file.
  → **Skills =** evals, tooling

*Slip order if the week breaks: nothing.*

## W12 · Nov 9-15 — Checkpoint

- [ ] **[dash] Checkpoint (Mon Nov 9, 45 min)**
  1. In the dashboard Review, answer in writing.
  → **Done =** written answers exist.
  → **Skills =** career

---

# Phase 2 - 2-week blocks

## B1 · Nov 16-29

- [ ] **[career]** 10 applications (app) · 2 mock interviews. **Done =** applications logged.
  → **Skills =** career

## B4 · Dec 28-Jan 10 (holiday-light)

- [ ] **[learn] Udemy LLM:** finish two modules.
  → **Skills =** aws

## B7 · Feb 8-21

- [ ] **[dash] Checkpoint Feb 21:** written review, same 5-question format.
  → **Skills =** career

---

## Metrics targets

| Metric | Baseline | Target | Slug | Unit | Definition | How to measure |
|---|---|---|---|---|---|---|
| Rejection rate v2 vs v1 | - | measurably lower | rejection_rate | % | Share of golden-set summaries rejected. | Run make eval on the frozen set. |
| p95 latency (cached) | >1s | <0.8s | p95_latency | s | 95th-percentile request latency. | Read Grafana after the standard k6 run. |
| Time to first token | - | tracked | ttft | s | Request to first streamed token. | Read the TTFT metric during the k6 profile. |

## Checkpoint questions

| question | helper |
|---|---|
| interview-grade eval number? | Write the exact sentence with real numbers. |
| chaos falsified anything? | Name the belief that changed. |
| PR merged or stale? | Status, date, and next move. |
| fourth repo? | Yes/no and what gets archived. |
| still AI-infra path? | One paragraph max. |
`

func TestParseWeeks(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Weeks) != 6 {
		t.Fatalf("weeks = %d, want 6", len(doc.Weeks))
	}

	tests := []struct {
		code, phase, start, end, focus string
	}{
		{"W1", "P1", "2026-08-24", "2026-08-30", "Golden set"},
		{"W2", "P1", "2026-08-31", "2026-09-06", "Runner"},
		{"W12", "P1", "2026-11-09", "2026-11-15", "Checkpoint"},
		{"B1", "P2", "2026-11-16", "2026-11-29", ""},
		{"B4", "P2", "2026-12-28", "2027-01-10", "holiday-light"},
		{"B7", "P2", "2027-02-08", "2027-02-21", ""},
	}

	for i, want := range tests {
		got := doc.Weeks[i]
		if got.Code != want.code || got.Phase != want.phase ||
			got.StartDate != want.start || got.EndDate != want.end {
			t.Errorf("week[%d] = %+v, want %+v", i, got, want)
		}

		focus := ""
		if got.Focus != nil {
			focus = *got.Focus
		}
		if focus != want.focus {
			t.Errorf("%s focus = %q, want %q", want.code, focus, want.focus)
		}
	}
}

func TestParseTaskShapes(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	byKey := map[string]int{}
	for i, task := range doc.Tasks {
		byKey[task.SeedKey] = i
	}

	t.Run("bold title with numbered steps and a done line", func(t *testing.T) {
		task := doc.Tasks[byKey["W1:golden-set-committed-then-frozen-forever"]]

		if task.Title != "Golden set committed (then frozen forever)" {
			t.Errorf("title = %q", task.Title)
		}
		if task.Project != "synapse" {
			t.Errorf("project = %q, want synapse", task.Project)
		}
		if len(task.Steps) != 2 {
			t.Errorf("steps = %v, want 2", task.Steps)
		}
		if task.DoneMeans == nil || *task.DoneMeans != "set is in git." {
			t.Errorf("done_means = %v", task.DoneMeans)
		}
		if got := strings.Join(task.Skills, ","); got != "evals" {
			t.Errorf("skills = %q, want evals", got)
		}
	})

	t.Run("tag-only bold, title outside", func(t *testing.T) {
		task := doc.Tasks[byKey["B1:10-applications-app-2-mock-interviews"]]

		if task.Title != "10 applications (app) · 2 mock interviews" {
			t.Errorf("title = %q", task.Title)
		}
		if task.Project != "career" {
			t.Errorf("project = %q, want career", task.Project)
		}
		if len(task.Steps) != 0 {
			t.Errorf("steps = %v, want none", task.Steps)
		}
		if task.DoneMeans == nil || *task.DoneMeans != "applications logged." {
			t.Errorf("done_means = %v, want inline done marker", task.DoneMeans)
		}
	})

	t.Run("bold title ending in a colon, detail trailing", func(t *testing.T) {
		task := doc.Tasks[byKey["B4:udemy-llm"]]

		if task.Title != "Udemy LLM" {
			t.Errorf("title = %q, want the colon stripped", task.Title)
		}
		if len(task.Steps) != 1 || task.Steps[0] != "finish two modules." {
			t.Errorf("steps = %v, want the trailing text", task.Steps)
		}
	})
}

func TestParseReferenceData(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Projects) != 7 {
		t.Fatalf("projects = %d, want 7", len(doc.Projects))
	}
	if len(doc.Categories) != 8 {
		t.Fatalf("categories = %d, want 8", len(doc.Categories))
	}
	if len(doc.Skills) != 5 {
		t.Fatalf("skills = %d, want 5", len(doc.Skills))
	}
	if len(doc.SkillTiers) != 1 || doc.SkillTiers[0].ID != "Expert" {
		t.Fatalf("skill tiers = %+v, want Expert only", doc.SkillTiers)
	}
}

func TestParseGoals(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Goals) != 6 {
		t.Fatalf("goals = %d, want 6", len(doc.Goals))
	}

	type goalFacts struct{ project, phase string }
	byCode := map[string]goalFacts{}
	for _, g := range doc.Goals {
		byCode[g.Code] = goalFacts{project: g.Project, phase: *g.Phase}
	}

	tests := map[string]goalFacts{
		"G0":  {project: "dash", phase: "P1"},
		"G6":  {project: "learn", phase: "P2"},
		"G11": {project: "synapse", phase: "P1"},
		"G14": {project: "synapse", phase: "P1"},
		"G15": {project: "all", phase: "P1"},
		"G17": {project: "career", phase: "P3"},
	}

	for code, want := range tests {
		if got := byCode[code]; got != want {
			t.Errorf("%s = %+v, want %+v", code, got, want)
		}
	}
}

func TestParseRhythmMetricDefsAndCheckpoints(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	wantDays := map[string]string{"Mon": "1", "Tue-Wed": "2,3", "Sun": "7"}
	for _, r := range doc.Rhythm {
		if wantDays[r.Label] != r.Weekdays {
			t.Errorf("%s weekdays = %q, want %q", r.Label, r.Weekdays, wantDays[r.Label])
		}
	}

	if len(doc.MetricDefs) != 3 {
		t.Fatalf("metric defs = %d, want 3", len(doc.MetricDefs))
	}
	if doc.MetricDefs[1].Slug == nil || *doc.MetricDefs[1].Slug != "p95_latency" {
		t.Errorf("slug = %v, want p95_latency", doc.MetricDefs[1].Slug)
	}
	if doc.MetricDefs[0].Baseline != nil {
		t.Errorf("dash baseline = %q, want nil", *doc.MetricDefs[0].Baseline)
	}

	if len(doc.Checkpoints) != 2 {
		t.Fatalf("checkpoints = %d, want 2", len(doc.Checkpoints))
	}
	for _, c := range doc.Checkpoints {
		if len(c.Questions) != 5 || len(c.Helpers) != 5 {
			t.Fatalf("%s checkpoint = %+v, want 5 questions and helpers", c.Week, c)
		}
	}
}

func TestParseAllowsMinimalDocumentWithoutOptionalSections(t *testing.T) {
	plan := `# Minimal

# Phase 1

## W1 · Mar 1-7

- [ ] **[ops] First task**
  → **Done =** shipped.
`

	doc, err := Parse(plan, 2027)
	if err != nil {
		t.Fatalf("parse minimal plan: %v", err)
	}

	if len(doc.Projects) != 1 || doc.Projects[0].ID != "ops" {
		t.Fatalf("projects = %+v, want derived ops project", doc.Projects)
	}
	if len(doc.Skills) != 0 || len(doc.Tasks[0].Skills) != 0 {
		t.Fatalf("skills = %+v, task skills = %+v; want no skills", doc.Skills, doc.Tasks[0].Skills)
	}
}

func TestParseFrontMatterOverridesProfile(t *testing.T) {
	source := `---
plan:
  id: uploaded-plan
  name: "Uploaded Plan"
  start_year: 2030
  active_goal_limit: 2
---
# Phase 1

## W1 · Jan 1-7 — Open

- [ ] **[ops] Ship the first slice**
`

	doc, err := ParseWithProfile(source, DefaultProfile())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if doc.Plan.ID != "uploaded-plan" || doc.Plan.Name != "Uploaded Plan" {
		t.Fatalf("plan = %+v, want uploaded front-matter identity", doc.Plan)
	}
	if doc.Plan.ActiveGoalLimit != 2 {
		t.Fatalf("active goal limit = %d, want 2", doc.Plan.ActiveGoalLimit)
	}
	if len(doc.Weeks) != 1 || doc.Weeks[0].StartDate != "2030-01-01" {
		t.Fatalf("weeks = %+v, want front-matter start year", doc.Weeks)
	}
}

func TestFrontMatterDoesNotShiftParserErrorLines(t *testing.T) {
	source := `---
plan:
  id: uploaded-plan
  name: "Uploaded Plan"
  start_year: 2030
---
# Phase 1

## W1 · Jan 1-7 — Open
this line is still line ten
`

	_, err := ParseWithProfile(source, DefaultProfile())
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}

	if !strings.Contains(err.Error(), "line 10:") {
		t.Fatalf("error = %q, want original line 10", err)
	}
}

func TestParseRejectsUnknownLineInsideAWeek(t *testing.T) {
	plan := samplePlan + "\n## W3 · Sep 7-13 — Ship\n\nthis line is not a task\n"

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for an unrecognised line, got nil")
	} else if !strings.Contains(err.Error(), "unrecognised line") {
		t.Errorf("error = %v, want it to name the unrecognised line", err)
	}
}

func TestParseRejectsUnknownProjectTag(t *testing.T) {
	plan := samplePlan + "\n## W3 · Sep 7-13 — Ship\n\n- [ ] **[nonsense] A task**\n"

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for an unknown project tag, got nil")
	} else if !strings.Contains(err.Error(), "unknown project tag") ||
		!strings.Contains(err.Error(), "line ") {
		t.Errorf("error = %v, want line-numbered unknown project", err)
	}
}

func TestParseRejectsTaskWithoutSkillsWhenCatalogueExists(t *testing.T) {
	plan := strings.Replace(samplePlan, "  → **Skills =** oss\n", "", 1)

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for a task with no skills, got nil")
	} else if !strings.Contains(err.Error(), "has no skills") ||
		!strings.Contains(err.Error(), "line ") {
		t.Errorf("error = %v, want line-numbered no-skills error", err)
	}
}

func TestParseRejectsUndefinedSkill(t *testing.T) {
	plan := strings.Replace(samplePlan, "  → **Skills =** evals\n", "  → **Skills =** nope\n", 1)

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for an undefined skill, got nil")
	} else if !strings.Contains(err.Error(), "undefined skill") ||
		!strings.Contains(err.Error(), "line ") {
		t.Errorf("error = %v, want line-numbered undefined skill", err)
	}
}

func TestParseRejectsMissingMetricHelp(t *testing.T) {
	plan := strings.Replace(
		samplePlan,
		"| p95 latency (cached) | >1s | <0.8s | p95_latency | s | 95th-percentile request latency. | Read Grafana after the standard k6 run. |",
		"| p95 latency (cached) | >1s | <0.8s | p95_latency | s | | |",
		1,
	)

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for missing metric help, got nil")
	} else if !strings.Contains(err.Error(), "needs definition and how to measure") ||
		!strings.Contains(err.Error(), "line ") {
		t.Errorf("error = %v, want line-numbered metric-help error", err)
	}
}

func TestParseRejectsOverlappingWeekWindows(t *testing.T) {
	plan := strings.Replace(samplePlan, "## W2 · Aug 31-Sep 6", "## W2 · Aug 29-Sep 6", 1)

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for overlapping weeks, got nil")
	} else if !strings.Contains(err.Error(), "overlaps") || !strings.Contains(err.Error(), "line ") {
		t.Errorf("error = %v, want line-numbered overlap error", err)
	}
}

func TestParseRejectsDuplicateSeedKeys(t *testing.T) {
	plan := samplePlan + "\n## W3 · Sep 7-13 — Ship\n\n" +
		"- [ ] **[dash] Same title**\n  → **Skills =** tooling\n\n" +
		"- [ ] **[dash] Same title**\n  → **Skills =** tooling\n"

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for duplicate seed keys, got nil")
	} else if !strings.Contains(err.Error(), "duplicate seed_key") {
		t.Errorf("error = %v, want it to name the duplicate", err)
	}
}

func TestParseFileUsesPlanYAMLProfile(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "side.md")
	configPath := filepath.Join(dir, "plan.yaml")

	if err := os.WriteFile(configPath, []byte(`plan:
  id: side-plan
  name: "Side Plan"
  start_year: 2027
  active_goal_limit: 2
sections:
  projects: "Project list"
  goals: "Objective list"
  skills: "Capability list"
  rhythm: "Cadence"
  categories: "Event types"
  metrics: "Numbers"
  checkpoints: "Review prompts"
week_heading:
  pattern: '^###\s+(?P<code>S\d+)\s+::\s+(?P<dates>.+?)\s+::\s+(?P<focus>.*)$'
task_bullet:
  pattern: '^\*\s+\[[ x]\]\s+(?P<bold>.+)(?P<trailing>)$'
`), 0o644); err != nil {
		t.Fatalf("write plan.yaml: %v", err)
	}

	if err := os.WriteFile(planPath, []byte(`# Side Plan

## Project list

| id | label |
|---|---|
| ops | Operations |

## Capability list

| code | name | description | associate when | target |
|---|---|---|---|---|
| ops | Operations | Run the operating loop. | the task improves operations. | Practitioner |

# Phase 1

### S1 :: Mar 1-7 :: First week

* [ ] [ops] Wire alert
  → **Done =** alert fires.
  → **Skills =** ops
`), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	doc, err := ParseFile(planPath)
	if err != nil {
		t.Fatalf("parse configured plan: %v", err)
	}

	if doc.Plan.ID != "side-plan" || doc.Plan.ActiveGoalLimit != 2 {
		t.Fatalf("plan = %+v, want side-plan with active limit 2", doc.Plan)
	}
	if len(doc.Weeks) != 1 || doc.Weeks[0].Code != "S1" || doc.Weeks[0].StartDate != "2027-03-01" {
		t.Fatalf("weeks = %+v", doc.Weeks)
	}
	if len(doc.Tasks) != 1 || doc.Tasks[0].Project != "ops" || doc.Tasks[0].Skills[0] != "ops" {
		t.Fatalf("tasks = %+v", doc.Tasks)
	}
}

func TestParseWeekdays(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"Mon", "1", false},
		{"Tue-Wed", "2,3", false},
		{"Mon-Fri", "1,2,3,4,5", false},
		{"Sun", "7", false},
		{"Fri-Mon", "", true},
		{"Xxx", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseWeekdays(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseWeekdays(%q) = %q, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWeekdays(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("parseWeekdays(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	tests := map[string]string{
		"Golden set committed (then frozen forever)": "golden-set-committed-then-frozen-forever",
		"CV + 10 applications":                       "cv-10-applications",
		"  Leading and trailing  ":                   "leading-and-trailing",
	}

	for in, want := range tests {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

// The generator must still parse the real plan: this is the regression guard for the committed
// seed.json.
func TestParseRealMasterPlan(t *testing.T) {
	doc, err := ParseFile("../../master-plan-v5.md")
	if err != nil {
		t.Fatalf("parse master-plan-v5.md: %v", err)
	}

	if len(doc.Goals) != 18 {
		t.Errorf("goals = %d, want 18 (G0..G17)", len(doc.Goals))
	}
	if len(doc.Weeks) != 19 {
		t.Errorf("weeks = %d, want 19 (W1..W12 + B1..B7)", len(doc.Weeks))
	}
	if len(doc.Projects) != 8 {
		t.Errorf("projects = %d, want 8", len(doc.Projects))
	}
	if len(doc.Categories) != 8 {
		t.Errorf("categories = %d, want 8", len(doc.Categories))
	}
	if len(doc.Skills) != 17 {
		t.Errorf("skills = %d, want 17", len(doc.Skills))
	}
	if len(doc.Checkpoints) != 2 {
		t.Errorf("checkpoints = %d, want 2", len(doc.Checkpoints))
	}

	for _, c := range doc.Checkpoints {
		if len(c.Questions) != 5 || len(c.Helpers) != 5 {
			t.Errorf("%s checkpoint = %+v, want 5 questions and helpers", c.Week, c)
		}
	}

	for _, m := range doc.MetricDefs {
		if m.Slug == nil || m.Unit == nil || m.Definition == nil || m.HowToMeasure == nil {
			t.Errorf("metric %q missing field-guide help: %+v", m.Name, m)
		}
	}

	if doc.Weeks[0].StartDate != "2026-08-24" {
		t.Errorf("W1 starts %q, want 2026-08-24", doc.Weeks[0].StartDate)
	}
}

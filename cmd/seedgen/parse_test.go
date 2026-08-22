package main

import (
	"os"
	"strings"
	"testing"
)

// A miniature plan exercising every shape the real document uses.
const samplePlan = `# Master Plan

## Operating system

| Day | Slot |
|---|---|
| Mon | Theory: DDIA ~1 ch (1–2h) |
| Tue–Wed | Project work (2–3h) |
| Sun | Review, 3-bullet log. Rest. |

## Log categories (the + buttons)

Application 📮 · Course module 📚 · Portfolio 🔧 · Medium post ✍️ · OSS event 🔀 · Benchmark/number 📊 · Networking 🤝 · Exam/cert 🎓

## Goals (Kanban seed)

| ID | Goal | Project | Done means | Target |
|---|---|---|---|---|
| G0 | Dashboard live | dash | v1 on tailnet | Aug 31 |
| G6 | AWS SAA | learn | Passed | P2-B1 |
| G11 | Fine-tune | synapse + gateway | LoRA evaled | W11 |
| G14 | IaC | synapse (infra/) | Terraform | W12 |
| G15 | Security pass | all repos | OWASP review | W12 / P2-B6 |
| G17 | Role | career | Offer signed | Q2 2027 |

---

# Phase 1 — 12 weeks

## W1 · Aug 24–30 — Golden set

- [ ] **[synapse] Golden set committed (then frozen forever)**
  1. Create ` + "`eval/golden/`" + `.
  2. Pick 16–26 real windows.
  → **Done =** set is in git.

- [ ] **[oss] OSS target picked**
  1. Shortlist 3 Go projects.
  → **Done =** one repo chosen.

## W2 · Aug 31–Sep 6 — Runner

- [ ] **[synapse] Eval runner**
  1. Create ` + "`cmd/evalrun`" + `.
  → **Done =** make eval produces a results file.

*Slip order if the week breaks: nothing.*

## W12 · Nov 9–15 — Checkpoint

- [ ] **[dash] Checkpoint (Mon Nov 9, 45 min)**
  1. In the dashboard Review, answer in writing: eval number? chaos falsified anything? PR merged or stale?
  2. Any bad answer → adjust scope.
  → **Done =** written answers exist.

---

# Phase 2 — 2-week blocks

## B1 · Nov 16–29

- [ ] **[career]** 10 applications (📮) · 2 mock interviews.

## B4 · Dec 28–Jan 10 (holiday-light)

- [ ] **[learn] Udemy LLM:** finish two modules.

## B7 · Feb 8–21

- [ ] **[dash] Checkpoint Feb 21:** written review, same 5-question format.

---

## Metrics targets

| Metric | Baseline | Target |
|---|---|---|
| p95 latency (cached) | >1s | <0.8s |
| Cache hit rate | <20% | >60% |
| OSS PRs merged | — | 2 |
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
		// The plan crosses into the next year here, and everything after it follows.
		{"B4", "P2", "2026-12-28", "2027-01-10", "holiday-light"},
		{"B7", "P2", "2027-02-08", "2027-02-21", ""},
	}

	for i, want := range tests {
		got := doc.Weeks[i]

		if got.Code != want.code {
			t.Errorf("week[%d].code = %q, want %q", i, got.Code, want.code)
		}
		if got.Phase != want.phase {
			t.Errorf("%s phase = %q, want %q", want.code, got.Phase, want.phase)
		}
		if got.StartDate != want.start {
			t.Errorf("%s start = %q, want %q", want.code, got.StartDate, want.start)
		}
		if got.EndDate != want.end {
			t.Errorf("%s end = %q, want %q", want.code, got.EndDate, want.end)
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
		if task.SortOrder != 100 {
			t.Errorf("sort_order = %d, want 100", task.SortOrder)
		}
	})

	t.Run("tag-only bold, title outside", func(t *testing.T) {
		task := doc.Tasks[byKey["B1:10-applications-2-mock-interviews"]]

		if task.Title != "10 applications (📮) · 2 mock interviews" {
			t.Errorf("title = %q", task.Title)
		}
		if task.Project != "career" {
			t.Errorf("project = %q, want career", task.Project)
		}
		if len(task.Steps) != 0 {
			t.Errorf("steps = %v, want none", task.Steps)
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

	tests := []struct {
		code, project, phase string
	}{
		{"G0", "dash", "P1"},
		{"G6", "learn", "P2"},
		// Known lossy case: "synapse + gateway" collapses to synapse.
		{"G11", "synapse", "P1"},
		{"G14", "synapse", "P1"},
		{"G15", "all", "P1"},
		{"G17", "career", "P3"},
	}

	for _, want := range tests {
		got, ok := byCode[want.code]
		if !ok {
			t.Errorf("goal %s missing", want.code)
			continue
		}
		if got.project != want.project {
			t.Errorf("%s project = %q, want %q", want.code, got.project, want.project)
		}
		if got.phase != want.phase {
			t.Errorf("%s phase = %q, want %q", want.code, got.phase, want.phase)
		}
	}
}

func TestParseCategories(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Categories) != 8 {
		t.Fatalf("categories = %d, want 8", len(doc.Categories))
	}

	if doc.Categories[0].ID != "application" || doc.Categories[0].Label != "Application" ||
		doc.Categories[0].Icon != "📮" {
		t.Errorf("first category = %+v", doc.Categories[0])
	}

	// A multi-word label must keep its words and shed only the emoji.
	if doc.Categories[3].Label != "Medium post" || doc.Categories[3].Icon != "✍️" {
		t.Errorf("post category = %+v", doc.Categories[3])
	}
}

func TestParseRhythm(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := map[string]string{"Mon": "1", "Tue–Wed": "2,3", "Sun": "7"}

	if len(doc.Rhythm) != len(want) {
		t.Fatalf("rhythm rows = %d, want %d", len(doc.Rhythm), len(want))
	}

	for _, r := range doc.Rhythm {
		if want[r.Label] != r.Weekdays {
			t.Errorf("%s weekdays = %q, want %q", r.Label, r.Weekdays, want[r.Label])
		}
	}
}

func TestParseMetricDefs(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.MetricDefs) != 3 {
		t.Fatalf("metric defs = %d, want 3", len(doc.MetricDefs))
	}

	if doc.MetricDefs[0].Name != "p95 latency (cached)" {
		t.Errorf("name = %q", doc.MetricDefs[0].Name)
	}
	if doc.MetricDefs[0].Target == nil || *doc.MetricDefs[0].Target != "<0.8s" {
		t.Errorf("target = %v", doc.MetricDefs[0].Target)
	}

	// The plan's em-dash placeholder means "no value", not the literal character.
	if doc.MetricDefs[2].Baseline != nil {
		t.Errorf("em-dash baseline = %q, want nil", *doc.MetricDefs[2].Baseline)
	}
}

// The questions live inside a sentence; B7 reuses W12's set.
func TestParseCheckpoints(t *testing.T) {
	doc, err := Parse(samplePlan, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Checkpoints) != 2 {
		t.Fatalf("checkpoints = %d, want 2", len(doc.Checkpoints))
	}

	want := []string{"eval number?", "chaos falsified anything?", "PR merged or stale?"}

	for _, c := range doc.Checkpoints {
		if len(c.Questions) != len(want) {
			t.Fatalf("%s questions = %v, want %v", c.Week, c.Questions, want)
		}
		for i, q := range want {
			if c.Questions[i] != q {
				t.Errorf("%s question[%d] = %q, want %q", c.Week, i, c.Questions[i], q)
			}
		}
	}
}

// A silent skip is how a week of tasks disappears unnoticed, so unknown lines are fatal.
func TestParseRejectsUnknownLineInsideAWeek(t *testing.T) {
	plan := samplePlan + "\n## W3 · Sep 7–13 — Ship\n\nthis line is not a task\n"

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for an unrecognised line, got nil")
	} else if !strings.Contains(err.Error(), "unrecognised line") {
		t.Errorf("error = %v, want it to name the unrecognised line", err)
	}
}

func TestParseRejectsUnknownProjectTag(t *testing.T) {
	plan := samplePlan + "\n## W3 · Sep 7–13 — Ship\n\n- [ ] **[nonsense] A task**\n"

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for an unknown project tag, got nil")
	} else if !strings.Contains(err.Error(), "unknown project tag") {
		t.Errorf("error = %v, want it to name the bad tag", err)
	}
}

func TestParseRejectsIncompleteDocument(t *testing.T) {
	if _, err := Parse("# Master Plan\n\nnothing here\n", 2026); err == nil {
		t.Fatal("expected an error for a document with no categories, got nil")
	}
}

func TestParseRejectsDuplicateSeedKeys(t *testing.T) {
	plan := samplePlan + "\n## W3 · Sep 7–13 — Ship\n\n" +
		"- [ ] **[dash] Same title**\n\n- [ ] **[dash] Same title**\n"

	if _, err := Parse(plan, 2026); err == nil {
		t.Fatal("expected an error for duplicate seed keys, got nil")
	} else if !strings.Contains(err.Error(), "duplicate seed_key") {
		t.Errorf("error = %v, want it to name the duplicate", err)
	}
}

func TestNormaliseProject(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"synapse", "synapse", false},
		{"Synapse", "synapse", false},
		{"synapse (infra/)", "synapse", false},
		{"synapse + gateway", "synapse", false},
		{"all repos", "all", false},
		{"nonsense", "", true},
		{"", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := normaliseProject(tc.in)

			if tc.wantErr {
				if err == nil {
					t.Errorf("normaliseProject(%q) = %q, want an error", tc.in, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("normaliseProject(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("normaliseProject(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseWeekdays(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"Mon", "1", false},
		{"Tue–Wed", "2,3", false},
		{"Mon-Fri", "1,2,3,4,5", false},
		{"Sun", "7", false},
		{"Fri–Mon", "", true},
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
	source, err := os.ReadFile("../../master-plan-v5.md")
	if err != nil {
		t.Skipf("master-plan-v5.md not readable: %v", err)
	}

	doc, err := Parse(string(source), 2026)
	if err != nil {
		t.Fatalf("parse master-plan-v5.md: %v", err)
	}

	if len(doc.Goals) != 18 {
		t.Errorf("goals = %d, want 18 (G0..G17)", len(doc.Goals))
	}
	if len(doc.Weeks) != 19 {
		t.Errorf("weeks = %d, want 19 (W1..W12 + B1..B7)", len(doc.Weeks))
	}
	if len(doc.Categories) != 8 {
		t.Errorf("categories = %d, want 8", len(doc.Categories))
	}
	if len(doc.Checkpoints) != 2 {
		t.Errorf("checkpoints = %d, want 2", len(doc.Checkpoints))
	}

	for _, c := range doc.Checkpoints {
		if len(c.Questions) != 5 {
			t.Errorf("%s has %d questions, want the plan's 5", c.Week, len(c.Questions))
		}
	}

	if doc.Weeks[0].StartDate != "2026-08-24" {
		t.Errorf("W1 starts %q, want 2026-08-24", doc.Weeks[0].StartDate)
	}
}

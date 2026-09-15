package seed

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/FernasFragas/dashboard-app/internal/store"
	"github.com/FernasFragas/dashboard-app/migrations"
)

// newDB opens a migrated database against a real temp SQLite file.
func newDB(t *testing.T) *sql.DB {
	t.Helper()

	s, err := store.Open(filepath.Join(t.TempDir(), "seed.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	if err := s.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return s.DB()
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()

	var n int
	if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}

	return n
}

func TestLoadParsesEmbeddedDocument(t *testing.T) {
	doc, err := Load()
	if err != nil {
		t.Fatalf("load seed.json: %v", err)
	}

	if doc.GeneratedFrom != "master-plan-v5.md" {
		t.Errorf("generated_from = %q, want master-plan-v5.md", doc.GeneratedFrom)
	}

	// The counts the plan actually contains: G0..G21, sixteen weeks plus seven blocks, the
	// eight quick-log buttons, and the two checkpoints.
	if len(doc.Goals) != 22 {
		t.Errorf("goals = %d, want 22 (G0..G21)", len(doc.Goals))
	}
	if len(doc.Weeks) != 23 {
		t.Errorf("weeks = %d, want 23 (W1..W16 + B1..B7)", len(doc.Weeks))
	}
	if len(doc.Categories) != 8 {
		t.Errorf("categories = %d, want 8", len(doc.Categories))
	}
	if len(doc.Checkpoints) != 2 {
		t.Errorf("checkpoints = %d, want 2 (W16, B7)", len(doc.Checkpoints))
	}
	if len(doc.Tasks) == 0 {
		t.Error("tasks = 0, want the plan's checklist")
	}
	if len(doc.Rhythm) == 0 {
		t.Error("rhythm = 0, want the operating-system table")
	}
	if len(doc.MetricDefs) == 0 {
		t.Error("metric_defs = 0, want the metrics targets table")
	}
	if len(doc.MetricDefs) != 12 {
		t.Errorf("metric_defs = %d, want 12 including TTFT", len(doc.MetricDefs))
	}
	for _, m := range doc.MetricDefs {
		if m.Slug == nil || m.Unit == nil || m.Definition == nil || m.HowToMeasure == nil {
			t.Errorf("metric %q missing field-guide help: %+v", m.Name, m)
		}
	}
	for _, c := range doc.Checkpoints {
		if len(c.Helpers) != len(c.Questions) {
			t.Errorf("%s helpers = %d, questions = %d", c.Week, len(c.Helpers), len(c.Questions))
		}
	}
}

// Deleting the database and rebooting recreates it with the full plan.
func TestApplySeedsEverything(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	doc, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	res, err := Apply(ctx, db, doc)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if res.Goals != len(doc.Goals) {
		t.Errorf("inserted goals = %d, want %d", res.Goals, len(doc.Goals))
	}
	if got := count(t, db, "goals"); got != 22 {
		t.Errorf("goals rows = %d, want 22", got)
	}
	if got := count(t, db, "tasks"); got != len(doc.Tasks) {
		t.Errorf("tasks rows = %d, want %d", got, len(doc.Tasks))
	}
	if got := count(t, db, "categories"); got != 8 {
		t.Errorf("categories rows = %d, want 8", got)
	}
	if got := count(t, db, "projects"); got != len(doc.Projects) {
		t.Errorf("projects rows = %d, want %d", got, len(doc.Projects))
	}
	if got := count(t, db, "phases"); got != len(doc.Phases) {
		t.Errorf("phases rows = %d, want %d", got, len(doc.Phases))
	}
	if got := count(t, db, "skill_tiers"); got != len(doc.SkillTiers) {
		t.Errorf("skill_tiers rows = %d, want %d", got, len(doc.SkillTiers))
	}

	// Every seeded goal carries its G-number and starts in Backlog: choosing what is active is
	// the accountability mechanism the board exists to force.
	var backlog int
	if err := db.QueryRow(
		`SELECT count(*) FROM goals WHERE status = 'backlog' AND code IS NOT NULL`,
	).Scan(&backlog); err != nil {
		t.Fatalf("count backlog goals: %v", err)
	}
	if backlog != 22 {
		t.Errorf("seeded backlog goals = %d, want 22", backlog)
	}

	// Every seeded task carries its steps and a stable seed_key.
	var withSteps int
	if err := db.QueryRow(
		`SELECT count(*) FROM tasks WHERE seed_key IS NOT NULL AND steps <> '[]'`,
	).Scan(&withSteps); err != nil {
		t.Fatalf("count tasks with steps: %v", err)
	}
	if withSteps == 0 {
		t.Error("no seeded task has steps: the substeps did not survive the load")
	}
}

// Rule 2: the loader run twice leaves row counts identical.
func TestApplyIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	doc, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	before := map[string]int{}
	tables := []string{
		"projects", "phases", "skill_tiers", "weeks", "rhythm", "categories", "metric_defs",
		"goals", "tasks", "checkpoints",
	}

	for _, table := range tables {
		before[table] = count(t, db, table)
	}

	second, err := Apply(ctx, db, doc)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if second.Total() != 0 {
		t.Errorf("second apply inserted %d rows, want 0", second.Total())
	}

	for _, table := range tables {
		if got := count(t, db, table); got != before[table] {
			t.Errorf("%s rows = %d after re-run, want %d", table, got, before[table])
		}
	}
}

// Rule 3: re-seeding after progress must never touch it. This is the whole point of
// additive-upsert over v2's delete-and-reseed rule.
func TestApplyPreservesProgress(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	doc, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Do some work: finish a task, activate a goal, and add a task of your own.
	if _, err := db.ExecContext(ctx, `
		UPDATE tasks SET status = 'done', done_at = '2026-08-25T18:00:00Z', version = 2
		WHERE seed_key = (SELECT seed_key FROM tasks WHERE week = 'W1' ORDER BY sort_order LIMIT 1)`,
	); err != nil {
		t.Fatalf("mark task done: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE goals SET status = 'active', sort_order = 100, version = 2 WHERE code = 'G1'`,
	); err != nil {
		t.Fatalf("activate goal: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO tasks (week, title, project, steps, status, seed_key, sort_order, version, created_at)
		VALUES ('W1', 'A task I added myself', 'dash', '[]', 'todo', NULL, 9999, 1, '2026-08-25T10:00:00Z')`,
	); err != nil {
		t.Fatalf("insert hand-created task: %v", err)
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("re-apply: %v", err)
	}

	var (
		status string
		doneAt *string
	)
	if err := db.QueryRow(`
		SELECT status, done_at FROM tasks WHERE week = 'W1' AND seed_key IS NOT NULL
		ORDER BY sort_order LIMIT 1`,
	).Scan(&status, &doneAt); err != nil {
		t.Fatalf("read task: %v", err)
	}

	if status != "done" || doneAt == nil {
		t.Errorf("task status = %q, done_at = %v after re-seed: progress was overwritten", status, doneAt)
	}

	var goalStatus string
	if err := db.QueryRow(`SELECT status FROM goals WHERE code = 'G1'`).Scan(&goalStatus); err != nil {
		t.Fatalf("read goal: %v", err)
	}
	if goalStatus != "active" {
		t.Errorf("goal G1 status = %q after re-seed, want active", goalStatus)
	}

	// A hand-created row has no seed_key, so the loader cannot see it at all.
	var mine int
	if err := db.QueryRow(
		`SELECT count(*) FROM tasks WHERE seed_key IS NULL AND title = 'A task I added myself'`,
	).Scan(&mine); err != nil {
		t.Fatalf("count hand-created tasks: %v", err)
	}
	if mine != 1 {
		t.Errorf("hand-created tasks = %d, want 1 untouched", mine)
	}
}

func TestApplyUpsertsReferenceHelpAndPreservesCheckpointProgress(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	doc, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	metric := doc.MetricDefs[0]
	if metric.Definition == nil {
		t.Fatalf("metric %q has no definition in seed", metric.Name)
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE metric_defs SET definition = 'stale help' WHERE name = ?`,
		metric.Name,
	); err != nil {
		t.Fatalf("mutate metric definition: %v", err)
	}

	checkpoint := doc.Checkpoints[0]
	answers := make([]string, len(checkpoint.Questions))
	for i := range answers {
		answers[i] = "kept answer"
	}
	encodedAnswers, err := json.Marshal(answers)
	if err != nil {
		t.Fatalf("encode answers: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE checkpoints
		SET questions = '["stale?","stale?","stale?","stale?","stale?"]',
			helpers = '["stale","stale","stale","stale","stale"]',
			answers = ?,
			completed_at = '2026-11-15T20:10:00Z'
		WHERE week = ?`,
		string(encodedAnswers), checkpoint.Week,
	); err != nil {
		t.Fatalf("mutate checkpoint: %v", err)
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("re-apply: %v", err)
	}

	var definition string
	if err := db.QueryRowContext(ctx,
		`SELECT definition FROM metric_defs WHERE name = ?`,
		metric.Name,
	).Scan(&definition); err != nil {
		t.Fatalf("read metric definition: %v", err)
	}
	if definition != *metric.Definition {
		t.Errorf("definition = %q, want %q", definition, *metric.Definition)
	}

	var (
		questionsRaw string
		helpersRaw   string
		answersRaw   string
		completedAt  string
	)
	if err := db.QueryRowContext(ctx, `
		SELECT questions, helpers, answers, completed_at FROM checkpoints WHERE week = ?`,
		checkpoint.Week,
	).Scan(&questionsRaw, &helpersRaw, &answersRaw, &completedAt); err != nil {
		t.Fatalf("read checkpoint: %v", err)
	}

	var gotQuestions, gotHelpers, gotAnswers []string
	for label, target := range map[string]struct {
		raw  string
		into *[]string
	}{
		"questions": {raw: questionsRaw, into: &gotQuestions},
		"helpers":   {raw: helpersRaw, into: &gotHelpers},
		"answers":   {raw: answersRaw, into: &gotAnswers},
	} {
		if err := json.Unmarshal([]byte(target.raw), target.into); err != nil {
			t.Fatalf("decode %s: %v", label, err)
		}
	}

	if gotQuestions[0] != checkpoint.Questions[0] {
		t.Errorf("question[0] = %q, want %q", gotQuestions[0], checkpoint.Questions[0])
	}
	if gotHelpers[0] != checkpoint.Helpers[0] {
		t.Errorf("helper[0] = %q, want %q", gotHelpers[0], checkpoint.Helpers[0])
	}
	if gotAnswers[0] != "kept answer" {
		t.Errorf("answer[0] = %q, want preserved answer", gotAnswers[0])
	}
	if completedAt != "2026-11-15T20:10:00Z" {
		t.Errorf("completed_at = %q, want preserved timestamp", completedAt)
	}
}

// Adding a task to the master plan and re-seeding makes it appear, without disturbing the rest.
func TestApplyIsAdditive(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	doc, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	beforeTasks := count(t, db, "tasks")
	beforeGoals := count(t, db, "goals")

	doc.Tasks = append(doc.Tasks, Task{
		SeedKey: "W1:a-task-added-to-the-plan-later",
		Week:    "W1",
		Title:   "A task added to the plan later",
		Project: "dash",
		Steps:   []string{"do the thing"},
		Skills:  []string{"tooling"},
	})

	res, err := Apply(ctx, db, doc)
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}

	if res.Tasks != 1 {
		t.Errorf("inserted tasks = %d, want exactly the new one", res.Tasks)
	}
	if got := count(t, db, "tasks"); got != beforeTasks+1 {
		t.Errorf("tasks rows = %d, want %d", got, beforeTasks+1)
	}
	if got := count(t, db, "goals"); got != beforeGoals {
		t.Errorf("goals rows = %d, want %d unchanged", got, beforeGoals)
	}
}

// Renaming a task in the plan changes its seed_key, so it arrives as a new row and the old one
// stays. This is the accepted cost of additive-upsert (docs/DATABASE.md section 8).
func TestApplyTreatsRenamedTaskAsNew(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	doc := minimalSeedDoc("rename-test")

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	doc.Tasks[0] = Task{
		SeedKey: "W1:renamed-title", Week: "W1", Title: "Renamed title",
		Project: "dash", Steps: []string{}, Skills: []string{"tooling"},
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("re-apply: %v", err)
	}

	if got := count(t, db, "tasks"); got != 2 {
		t.Errorf("tasks rows = %d, want 2: the rename adds a row and leaves the original", got)
	}
}

// A partial seed is worse than none, so the whole load is one transaction.
func TestApplyRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	doc := Document{
		Plan:     Plan{ID: "rollback-test", Name: "Rollback test", ActiveGoalLimit: 3},
		Projects: []Project{{ID: "dash", Label: "Dashboard", SortOrder: 10}},
		Phases:   []Phase{{ID: "P1", Label: "P1", SortOrder: 10}},
		Weeks: []Week{{
			Code: "W1", Phase: "P1", StartDate: "2026-08-24", EndDate: "2026-08-30", SortOrder: 1,
		}},
		Goals: []Goal{
			{Code: "G0", Title: "Fine", Project: "dash", SortOrder: 100},
			{Code: "G1", Title: "Broken", Project: "not-a-project", SortOrder: 200},
		},
	}

	if _, err := Apply(ctx, db, doc); err == nil {
		t.Fatal("expected the bad project to fail the load, got nil")
	}

	if got := count(t, db, "goals"); got != 0 {
		t.Errorf("goals rows = %d after a failed load, want 0", got)
	}
	if got := count(t, db, "weeks"); got != 0 {
		t.Errorf("weeks rows = %d after a failed load, want 0", got)
	}
}

func TestApplyRefusesDifferentPlan(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	if _, err := Apply(ctx, db, minimalSeedDoc("plan-a")); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	if _, err := Apply(ctx, db, minimalSeedDoc("plan-b")); !errors.Is(err, ErrPlanMismatch) {
		t.Fatalf("second apply error = %v, want ErrPlanMismatch", err)
	}

	if got := count(t, db, "tasks"); got != 1 {
		t.Errorf("tasks rows = %d after refused plan, want original 1", got)
	}

	var planID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM plan_meta`).Scan(&planID); err != nil {
		t.Fatalf("read plan_meta: %v", err)
	}
	if planID != "plan-a" {
		t.Errorf("plan_meta id = %q, want plan-a", planID)
	}
}

func TestReplacePreservesHistoryAndRetiresReferencedCategories(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)

	if _, err := Apply(ctx, db, replaceSeedDoc("plan-a", "application")); err != nil {
		t.Fatalf("apply plan a: %v", err)
	}

	var goalID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM goals WHERE code = 'G0'`).Scan(&goalID); err != nil {
		t.Fatalf("read goal id: %v", err)
	}
	var skillID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM skills WHERE code = 'tooling'`).Scan(&skillID); err != nil {
		t.Fatalf("read skill id: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE tasks SET status = 'done', done_at = '2026-08-25T18:00:00Z'`); err != nil {
		t.Fatalf("mark task done: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE goals SET status = 'active', sort_order = 10`); err != nil {
		t.Fatalf("activate goal: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO log_entries (category_id, title, goal_id, occurred_at, created_at)
		VALUES ('application', 'kept log', ?, '2026-08-25T18:00:00Z', '2026-08-25T18:00:00Z')`,
		goalID,
	); err != nil {
		t.Fatalf("insert log: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO log_skills (log_id, skill_id)
		VALUES ((SELECT id FROM log_entries WHERE title = 'kept log'), ?)`, skillID,
	); err != nil {
		t.Fatalf("insert log skill: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO daily_reviews (date, learned, created_at, updated_at)
		VALUES ('2026-08-25', 'history survives', '2026-08-25T18:00:00Z', '2026-08-25T18:00:00Z')`); err != nil {
		t.Fatalf("insert review: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO metrics (name, value, recorded_at)
		VALUES ('p95 latency', 0.7, '2026-08-25T18:00:00Z')`); err != nil {
		t.Fatalf("insert metric: %v", err)
	}

	if _, err := Replace(ctx, db, replaceSeedDoc("plan-b", "number")); err != nil {
		t.Fatalf("replace: %v", err)
	}

	for _, tc := range []struct {
		table string
		want  int
	}{
		{"log_entries", 1},
		{"daily_reviews", 1},
		{"metrics", 1},
		{"log_skills", 0},
		{"goals", 1},
		{"tasks", 1},
	} {
		if got := count(t, db, tc.table); got != tc.want {
			t.Errorf("%s rows = %d, want %d", tc.table, got, tc.want)
		}
	}

	var (
		goalLink  *int64
		retiredAt *string
	)
	if err := db.QueryRowContext(ctx,
		`SELECT goal_id FROM log_entries WHERE title = 'kept log'`).Scan(&goalLink); err != nil {
		t.Fatalf("read log goal link: %v", err)
	}
	if goalLink != nil {
		t.Errorf("log goal_id = %v, want nil after goal replacement", *goalLink)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT retired_at FROM categories WHERE id = 'application'`).Scan(&retiredAt); err != nil {
		t.Fatalf("read retired category: %v", err)
	}
	if retiredAt == nil {
		t.Fatal("old referenced category was not retired")
	}

	var activeOld int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM categories WHERE id = 'application' AND retired_at IS NULL`,
	).Scan(&activeOld); err != nil {
		t.Fatalf("count active old category: %v", err)
	}
	if activeOld != 0 {
		t.Errorf("old category is still active")
	}
}

func minimalSeedDoc(id string) Document {
	return Document{
		Plan:     Plan{ID: id, Name: id, ActiveGoalLimit: 2},
		Projects: []Project{{ID: "dash", Label: "Dashboard", SortOrder: 10}},
		Phases:   []Phase{{ID: "P1", Label: "P1", SortOrder: 10}},
		Weeks: []Week{{
			Code: "W1", Phase: "P1", StartDate: "2026-08-24", EndDate: "2026-08-30", SortOrder: 1,
		}},
		Skills: []Skill{{
			Code: "tooling", Name: "Internal tooling", Description: "d",
			AssociateWhen: "w", SortOrder: 10,
		}},
		Tasks: []Task{{
			SeedKey: "W1:original-title", Week: "W1", Title: "Original title",
			Project: "dash", Steps: []string{}, Skills: []string{"tooling"},
		}},
	}
}

func replaceSeedDoc(id, category string) Document {
	doc := minimalSeedDoc(id)
	doc.Categories = []Category{{
		ID: category, Label: category, Icon: "x", SortOrder: 10,
	}}
	doc.Goals = []Goal{{
		Code: "G0", Title: "Goal " + id, Project: "dash", SortOrder: 100,
	}}
	return doc
}

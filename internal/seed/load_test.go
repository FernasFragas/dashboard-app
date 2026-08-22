package seed

import (
	"context"
	"database/sql"
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

	// The counts the plan actually contains: G0..G17, twelve weeks plus seven blocks, the
	// eight quick-log buttons, and the two checkpoints.
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
		t.Errorf("checkpoints = %d, want 2 (W12, B7)", len(doc.Checkpoints))
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
	if got := count(t, db, "goals"); got != 18 {
		t.Errorf("goals rows = %d, want 18", got)
	}
	if got := count(t, db, "tasks"); got != len(doc.Tasks) {
		t.Errorf("tasks rows = %d, want %d", got, len(doc.Tasks))
	}
	if got := count(t, db, "categories"); got != 8 {
		t.Errorf("categories rows = %d, want 8", got)
	}

	// Every seeded goal carries its G-number and starts in Backlog: choosing what is active is
	// the accountability mechanism the board exists to force.
	var backlog int
	if err := db.QueryRow(
		`SELECT count(*) FROM goals WHERE status = 'backlog' AND code IS NOT NULL`,
	).Scan(&backlog); err != nil {
		t.Fatalf("count backlog goals: %v", err)
	}
	if backlog != 18 {
		t.Errorf("seeded backlog goals = %d, want 18", backlog)
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
	tables := []string{"weeks", "rhythm", "categories", "metric_defs", "goals", "tasks", "checkpoints"}

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

	doc := Document{
		Weeks: []Week{{Code: "W1", Phase: "P1", StartDate: "2026-08-24", EndDate: "2026-08-30", SortOrder: 1}},
		Tasks: []Task{{SeedKey: "W1:original-title", Week: "W1", Title: "Original title", Project: "dash", Steps: []string{}}},
	}

	if _, err := Apply(ctx, db, doc); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	doc.Tasks[0] = Task{
		SeedKey: "W1:renamed-title", Week: "W1", Title: "Renamed title", Project: "dash", Steps: []string{},
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
		Weeks: []Week{{Code: "W1", Phase: "P1", StartDate: "2026-08-24", EndDate: "2026-08-30", SortOrder: 1}},
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

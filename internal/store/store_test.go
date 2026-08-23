package store

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/FernasFragas/dashboard-app/migrations"
)

// Tests run against a real temp SQLite file, never mocks (ADR-004). Fixtures are inserted with
// raw SQL rather than the real seed so that store tests stay independent of the plan's content.

var fixedNow = time.Date(2026, 9, 24, 17, 40, 0, 0, time.UTC)

// newTestStore opens a migrated, fixture-loaded database in a temp directory.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	s := newEmptyStore(t)

	if err := s.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	insertFixtures(t, s)

	return s
}

// newEmptyStore opens an unmigrated database.
func newEmptyStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	s.SetClock(func() time.Time { return fixedNow })

	return s
}

// insertFixtures adds the reference rows the domain tables have foreign keys onto.
func insertFixtures(t *testing.T, s *Store) {
	t.Helper()

	ctx := context.Background()

	projects := [][]any{
		{"synapse", "Synapse", 10},
		{"gateway", "LLM Gateway", 20},
		{"dash", "Dashboard", 30},
		{"oss", "Open source", 40},
		{"learn", "Learning", 50},
		{"write", "Writing", 60},
		{"career", "Career", 70},
		{"all", "All repos", 80},
	}
	for _, p := range projects {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO projects (id, label, sort_order) VALUES (?, ?, ?)`, p...); err != nil {
			t.Fatalf("insert project fixture: %v", err)
		}
	}

	phases := [][]any{
		{"P1", "Phase 1", 10},
		{"P2", "Phase 2", 20},
		{"P3", "Phase 3", 30},
	}
	for _, p := range phases {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO phases (id, label, sort_order) VALUES (?, ?, ?)`, p...); err != nil {
			t.Fatalf("insert phase fixture: %v", err)
		}
	}

	tiers := [][]any{
		{"Practitioner", "Practitioner", 10},
		{"Expert", "Expert", 20},
	}
	for _, tier := range tiers {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO skill_tiers (id, label, sort_order) VALUES (?, ?, ?)`, tier...); err != nil {
			t.Fatalf("insert skill tier fixture: %v", err)
		}
	}

	weeks := [][]any{
		{"W1", "P1", "2026-08-24", "2026-08-30", "Golden set", 1},
		{"W5", "P1", "2026-09-21", "2026-09-27", "Chaos", 5},
		{"W12", "P1", "2026-11-09", "2026-11-15", "Checkpoint", 12},
	}

	for _, w := range weeks {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO weeks (code, phase, start_date, end_date, focus, sort_order)
			VALUES (?, ?, ?, ?, ?, ?)`, w...); err != nil {
			t.Fatalf("insert week fixture: %v", err)
		}
	}

	categories := [][]any{
		{"application", "Application", "📮", 1},
		{"module", "Course module", "📚", 2},
		{"number", "Benchmark/number", "📊", 3},
	}

	for _, c := range categories {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO categories (id, label, icon, sort_order) VALUES (?, ?, ?, ?)`,
			c...); err != nil {
			t.Fatalf("insert category fixture: %v", err)
		}
	}

	rhythm := [][]any{
		{"Mon", "1", "Theory", 1},
		{"Tue–Wed", "2,3", "Project work", 2},
	}

	for _, r := range rhythm {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO rhythm (label, weekdays, slot, sort_order) VALUES (?, ?, ?, ?)`,
			r...); err != nil {
			t.Fatalf("insert rhythm fixture: %v", err)
		}
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO metric_defs (
			name, slug, unit, baseline, target, definition, how_to_measure, sort_order
		)
		VALUES (
			'p95 latency (cached)', 'p95_latency', 's', '>1s', '<0.8s',
			'95th-percentile request latency under the standard mixed profile',
			'Grafana after a 10-min k6 run', 1
		)`); err != nil {
		t.Fatalf("insert metric def fixture: %v", err)
	}

	// Every task builds at least one skill, so the fixtures carry one to link to.
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO skills (id, code, name, description, associate_when, target_tier,
			sort_order, created_at)
		VALUES (1, 'tooling', 'Internal tooling', 'Tools that speed the rest of the work.',
			'the task improves your own workflow with software.', NULL, 10,
			'2026-08-24T09:00:00Z')`,
	); err != nil {
		t.Fatalf("insert skill fixture: %v", err)
	}
}

// fixtureSkillID is the skill inserted by insertFixtures, for tests that must satisfy the
// every-task-has-a-skill invariant without caring which skill it is.
const fixtureSkillID int64 = 1

func ptr[T any](v T) *T { return &v }

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()

	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}

	return n
}

// schemaFingerprint returns the full DDL of the database, so a migration re-run can be shown
// to have changed nothing.
func schemaFingerprint(t *testing.T, s *Store) string {
	t.Helper()

	rows, err := s.db.Query(
		`SELECT type, name, coalesce(sql, '') FROM sqlite_master ORDER BY type, name`)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	defer rows.Close()

	var out string
	for rows.Next() {
		var kind, name, sql string
		if err := rows.Scan(&kind, &name, &sql); err != nil {
			t.Fatalf("scan schema: %v", err)
		}
		out += kind + "|" + name + "|" + sql + "\n"
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("read schema: %v", err)
	}

	return out
}

// Rule 1: the migration runner applied twice is a no-op and leaves the schema unchanged.
func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newEmptyStore(t)

	if err := s.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	firstSchema := schemaFingerprint(t, s)

	version, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("schema version: %v", err)
	}

	// Derived from what is on disk, not hardcoded: every new migration would otherwise have to
	// remember to bump a number in this test.
	want := len(migrationFiles(t))
	if version != want {
		t.Fatalf("schema version = %d, want %d", version, want)
	}

	if err := s.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if got := schemaFingerprint(t, s); got != firstSchema {
		t.Errorf("schema changed on re-run:\nfirst:\n%s\nsecond:\n%s", firstSchema, got)
	}

	if got := countRows(t, s, "schema_migrations"); got != want {
		t.Errorf("schema_migrations rows = %d, want %d", got, want)
	}
}

func TestMigration004BackfillsDistinctSkillTiers(t *testing.T) {
	ctx := context.Background()
	s := newEmptyStore(t)

	if err := s.Migrate(ctx, embeddedMigrationSubset(t,
		"001_init.sql",
		"002_field_guide.sql",
		"003_skills.sql",
	)); err != nil {
		t.Fatalf("migrate through 003: %v", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO skills (id, code, name, description, associate_when, target_tier,
			sort_order, created_at)
		VALUES
			(1, 'shipping', 'Shipping', 'Ship production changes.',
				'the work ships a production change.', 'Expert', 10, '2026-08-24T09:00:00Z'),
			(2, 'debugging', 'Debugging', 'Diagnose defects.',
				'the work fixes a defect.', 'Expert', 20, '2026-08-24T09:00:00Z'),
			(3, 'testing', 'Testing', 'Verify behavior.',
				'the work adds coverage.', 'Practitioner', 30, '2026-08-24T09:00:00Z')
	`); err != nil {
		t.Fatalf("insert duplicate target tiers: %v", err)
	}

	if err := s.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("migrate through 004: %v", err)
	}

	for _, tier := range []string{"Expert", "Practitioner"} {
		var n int
		if err := s.db.QueryRowContext(ctx,
			`SELECT count(*) FROM skill_tiers WHERE id = ?`, tier).Scan(&n); err != nil {
			t.Fatalf("count %s tier: %v", tier, err)
		}
		if n != 1 {
			t.Errorf("skill_tiers[%s] rows = %d, want 1", tier, n)
		}
	}

	if got := countRows(t, s, "skills"); got != 3 {
		t.Errorf("skills rows after migration = %d, want 3", got)
	}
}

// migrationFiles lists the embedded migrations, so schema-version assertions track reality
// instead of a number someone has to remember to update.
func migrationFiles(t *testing.T) []string {
	t.Helper()

	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}

	return names
}

func embeddedMigrationSubset(t *testing.T, names ...string) fs.FS {
	t.Helper()

	files := make(map[string]string, len(names))
	for _, name := range names {
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatalf("read embedded migration %s: %v", name, err)
		}
		files[name] = string(body)
	}

	return mapFS(files)
}

func TestMigrateAppliesEveryTable(t *testing.T) {
	s := newTestStore(t)

	want := []string{
		"weeks", "rhythm", "categories", "metric_defs", "goals", "tasks",
		"log_entries", "daily_reviews", "metrics", "checkpoints", "schema_migrations",
		"xp_rules", "xp_events", "xp_event_skills", "achievements", "achievement_unlocks",
		"pairing_codes",
	}

	for _, table := range want {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
}

// The ON DELETE SET NULL rules that protect history are inert if this pragma is off, so it is
// verified rather than assumed (ADR-001).
func TestForeignKeysPragmaIsOn(t *testing.T) {
	s := newTestStore(t)

	var enabled int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("read pragma: %v", err)
	}

	if enabled != 1 {
		t.Errorf("foreign_keys = %d, want 1", enabled)
	}

	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}

	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}

func TestLoadMigrationsRejectsBadNames(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{"no version prefix", map[string]string{"init.sql": "SELECT 1;"}},
		{"non-numeric prefix", map[string]string{"abc_init.sql": "SELECT 1;"}},
		{"duplicate version", map[string]string{
			"001_a.sql": "SELECT 1;",
			"001_b.sql": "SELECT 1;",
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := loadMigrations(mapFS(tc.files)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestMigrateFailsOnBadSQL(t *testing.T) {
	s := newEmptyStore(t)

	err := s.Migrate(context.Background(), mapFS(map[string]string{
		"001_broken.sql": "CREATE TABLE (;",
	}))
	if err == nil {
		t.Fatal("expected a migration error, got nil")
	}

	// The failed migration must not be recorded: startup aborts and no partial schema remains.
	version, err := s.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("schema version: %v", err)
	}

	if version != 0 {
		t.Errorf("schema version = %d after a failed migration, want 0", version)
	}
}

func TestClassifyMapsNoRowsToNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetGoal(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGoal(999) error = %v, want ErrNotFound", err)
	}
}

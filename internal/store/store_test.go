package store

import (
	"context"
	"errors"
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
		INSERT INTO metric_defs (name, unit, baseline, target, sort_order)
		VALUES ('p95 latency (cached)', 's', '>1s', '<0.8s', 1)`); err != nil {
		t.Fatalf("insert metric def fixture: %v", err)
	}
}

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
	if version != 1 {
		t.Fatalf("schema version = %d, want 1", version)
	}

	if err := s.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if got := schemaFingerprint(t, s); got != firstSchema {
		t.Errorf("schema changed on re-run:\nfirst:\n%s\nsecond:\n%s", firstSchema, got)
	}

	if got := countRows(t, s, "schema_migrations"); got != 1 {
		t.Errorf("schema_migrations rows = %d, want 1", got)
	}
}

func TestMigrateAppliesEveryTable(t *testing.T) {
	s := newTestStore(t)

	want := []string{
		"weeks", "rhythm", "categories", "metric_defs", "goals", "tasks",
		"log_entries", "daily_reviews", "metrics", "checkpoints", "schema_migrations",
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

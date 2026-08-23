package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestRunCreatesConsistentSnapshotAndPrunes(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(root, "dashboard.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	}()

	if _, err := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, title TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO items (title) VALUES ('kept')"); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	dir := filepath.Join(root, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create backups dir: %v", err)
	}
	for day := 1; day <= 15; day++ {
		name := filepath.Join(dir, fmt.Sprintf("dashboard-202607%02d.db", day))
		if err := os.WriteFile(name, []byte("old"), 0o644); err != nil {
			t.Fatalf("write old backup: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "pre-plan-side-plan.db"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write pre-plan backup: %v", err)
	}

	loc := mustLocation(t, "Europe/Lisbon")
	now := time.Date(2026, 8, 23, 4, 0, 0, 0, loc)
	result, err := Run(context.Background(), Config{
		DB:        db,
		Dir:       dir,
		Retention: 14,
		Location:  loc,
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("run backup: %v", err)
	}
	if !result.Created {
		t.Fatalf("Created = false, want true")
	}
	if want := filepath.Join(dir, "dashboard-20260822.db"); result.Path != want {
		t.Fatalf("path = %q, want %q", result.Path, want)
	}

	backupDB, err := sql.Open("sqlite", "file:"+result.Path)
	if err != nil {
		t.Fatalf("open backup sqlite: %v", err)
	}
	defer func() {
		if err := backupDB.Close(); err != nil {
			t.Fatalf("close backup sqlite: %v", err)
		}
	}()

	var count int
	if err := backupDB.QueryRow("SELECT count(*) FROM items WHERE title = 'kept'").Scan(&count); err != nil {
		t.Fatalf("query backup: %v", err)
	}
	if count != 1 {
		t.Fatalf("backup count = %d, want 1", count)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read backups dir: %v", err)
	}
	dashboardBackups := 0
	for _, entry := range entries {
		if isDashboardBackup(entry.Name()) {
			dashboardBackups++
		}
	}
	if dashboardBackups != 14 {
		t.Fatalf("dashboard backup count = %d, want 14", dashboardBackups)
	}
	for _, removed := range []string{"dashboard-20260701.db", "dashboard-20260702.db"} {
		if _, err := os.Stat(filepath.Join(dir, removed)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists or stat failed with %v", removed, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "pre-plan-side-plan.db")); err != nil {
		t.Fatalf("pre-plan backup was pruned: %v", err)
	}
}

func TestNextRunUsesThreeAMLocalTime(t *testing.T) {
	loc := mustLocation(t, "Europe/Lisbon")

	before := time.Date(2026, 8, 22, 1, 30, 0, 0, loc)
	wantBefore := time.Date(2026, 8, 22, 3, 0, 0, 0, loc)
	if got := nextRun(before, loc); !got.Equal(wantBefore) {
		t.Fatalf("nextRun before = %s, want %s", got, wantBefore)
	}

	after := time.Date(2026, 8, 22, 4, 0, 0, 0, loc)
	wantAfter := time.Date(2026, 8, 23, 3, 0, 0, 0, loc)
	if got := nextRun(after, loc); !got.Equal(wantAfter) {
		t.Fatalf("nextRun after = %s, want %s", got, wantAfter)
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()

	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return loc
}

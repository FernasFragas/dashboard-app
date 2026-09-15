package store

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/FernasFragas/dashboard-app/migrations"
)

// Schema contract tests: every table the migrations create is documented in docs/DATABASE.md §4
// and included in GET /api/export. Both are easy to forget when adding a migration, and both
// failures are silent — an undocumented table, or data missing from the insurance export.

// notExported lists the tables deliberately left out of ExportData, with the reason.
var notExported = map[string]string{
	"schema_migrations": "represented by ExportData.SchemaVersion",
	"pairing_codes":     "ephemeral one-time secrets with a 90 s TTL (ADR-010)",
}

var documentedTableRe = regexp.MustCompile("^### 4\\.[0-9.]+ `([a-z_]+)`")

func TestEveryTableIsDocumented(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "DATABASE.md"))
	if err != nil {
		t.Fatalf("read docs/DATABASE.md: %v", err)
	}

	documented := map[string]bool{}
	for _, line := range strings.Split(string(body), "\n") {
		if m := documentedTableRe.FindStringSubmatch(line); m != nil {
			documented[m[1]] = true
		}
	}

	created := map[string]bool{}
	for _, table := range migratedTables(t) {
		created[table] = true
		if !documented[table] {
			t.Errorf("table %s is created by migrations/ but has no \"### 4.x `%s`\" section in "+
				"docs/DATABASE.md; document it there first — the doc is the schema source of truth",
				table, table)
		}
	}

	for table := range documented {
		if !created[table] {
			t.Errorf("docs/DATABASE.md documents table %s but no migration creates it; remove the "+
				"section or add the migration", table)
		}
	}
}

func TestEveryTableIsExported(t *testing.T) {
	exported := map[string]bool{}
	typ := reflect.TypeOf(ExportData{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		exported[name] = true
	}

	for _, table := range migratedTables(t) {
		if _, skip := notExported[table]; skip {
			continue
		}
		if !exported[table] {
			t.Errorf("table %s is missing from ExportData in internal/store/export.go; GET /api/export "+
				"must dump every table (docs/DATABASE.md §9). Export it, or add it to notExported "+
				"with the reason", table)
		}
	}
}

func migratedTables(t *testing.T) []string {
	t.Helper()

	s := newEmptyStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list tables: %v", err)
	}

	return tables
}

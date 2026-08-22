package store

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// migration is one numbered SQL file.
type migration struct {
	version int
	name    string
	sql     string
}

// Migrate applies every migration in fsys whose version is greater than the highest already
// recorded, each inside its own transaction. Forward-only; there are no down migrations, and
// the recovery path is a backup file (docs/DATABASE.md section 7).
//
// Running it twice is a no-op.
func (s *Store) Migrate(ctx context.Context, fsys fs.FS) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	current, err := s.currentVersion(ctx)
	if err != nil {
		return err
	}

	migrations, err := loadMigrations(fsys)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		if err := s.applyMigration(ctx, m); err != nil {
			return err
		}
	}

	return nil
}

// SchemaVersion reports the highest applied migration version, or 0 on a fresh database.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	return s.currentVersion(ctx)
}

func (s *Store) currentVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(
		ctx, `SELECT coalesce(max(version), 0) FROM schema_migrations`,
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}

	return version, nil
}

func (s *Store) applyMigration(ctx context.Context, m migration) error {
	err := s.tx(ctx, func(tx execer) error {
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			m.version, s.utcNow(),
		); err != nil {
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// loadMigrations reads and orders every NNN_*.sql file in fsys.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	seen := make(map[int]string, len(entries))

	for _, name := range entries {
		base := path.Base(name)

		prefix, _, found := strings.Cut(base, "_")
		if !found {
			return nil, fmt.Errorf("migration %q: name must be NNN_description.sql", base)
		}

		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q: bad version prefix: %w", base, err)
		}

		if other, dup := seen[version]; dup {
			return nil, fmt.Errorf("migration %q: duplicate version %d, also in %q", base, version, other)
		}
		seen[version] = base

		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", base, err)
		}

		migrations = append(migrations, migration{version: version, name: base, sql: string(body)})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

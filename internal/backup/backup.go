// Package backup owns scheduled SQLite snapshots.
package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultRetention = 14
	nightlyHour      = 3
)

// Config wires the backup runner.
type Config struct {
	DB        *sql.DB
	Dir       string
	Retention int
	Location  *time.Location
	Logger    *slog.Logger
	Now       func() time.Time
}

// Result describes a single backup attempt.
type Result struct {
	Path    string
	Created bool
	Pruned  []string
}

// Start runs the nightly backup loop until ctx is cancelled. Failures are logged and do not
// stop the process.
func Start(ctx context.Context, cfg Config) {
	cfg = withDefaults(cfg)
	if err := validate(cfg); err != nil {
		cfg.Logger.Error("backup scheduler disabled", "error", err)
		return
	}

	cfg.Logger.Info("backup scheduler started",
		"dir", cfg.Dir, "retention", cfg.Retention, "next_run", nextRun(cfg.Now(), cfg.Location))

	go func() {
		for {
			now := cfg.Now()
			timer := time.NewTimer(nextRun(now, cfg.Location).Sub(now))

			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			result, err := Run(ctx, cfg)
			if err != nil {
				cfg.Logger.Error("backup failed", "error", err)
				continue
			}

			if result.Created {
				cfg.Logger.Info("backup completed",
					"path", result.Path, "pruned", len(result.Pruned))
			} else {
				cfg.Logger.Info("backup already exists",
					"path", result.Path, "pruned", len(result.Pruned))
			}
		}
	}()
}

// Run writes one daily snapshot and prunes older dashboard backups.
func Run(ctx context.Context, cfg Config) (Result, error) {
	cfg = withDefaults(cfg)
	if err := validate(cfg); err != nil {
		return Result{}, err
	}

	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create backup dir: %w", err)
	}

	path := filepath.Join(cfg.Dir, "dashboard-"+backupDate(cfg.Now(), cfg.Location)+".db")
	result := Result{Path: path}

	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return result, fmt.Errorf("stat backup %s: %w", path, err)
		}

		if _, err := cfg.DB.ExecContext(ctx, "VACUUM INTO "+sqlQuote(path)); err != nil {
			_ = os.Remove(path)
			return result, fmt.Errorf("vacuum into %s: %w", path, err)
		}
		result.Created = true
	}

	pruned, err := prune(cfg.Dir, cfg.Retention)
	if err != nil {
		return result, err
	}
	result.Pruned = pruned
	return result, nil
}

func withDefaults(cfg Config) Config {
	if cfg.Retention == 0 {
		cfg.Retention = defaultRetention
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return cfg
}

func validate(cfg Config) error {
	if cfg.DB == nil {
		return errors.New("backup db is nil")
	}
	if strings.TrimSpace(cfg.Dir) == "" {
		return errors.New("backup dir is empty")
	}
	if cfg.Retention < 1 {
		return errors.New("backup retention must be at least 1")
	}
	return nil
}

func nextRun(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), nightlyHour, 0, 0, 0, loc)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func backupDate(now time.Time, loc *time.Location) string {
	return now.In(loc).AddDate(0, 0, -1).Format("20060102")
}

func prune(dir string, keep int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() && isDashboardBackup(entry.Name()) {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)
	if len(files) <= keep {
		return nil, nil
	}

	remove := files[:len(files)-keep]
	removed := make([]string, 0, len(remove))
	for _, name := range remove {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			return removed, fmt.Errorf("prune backup %s: %w", path, err)
		}
		removed = append(removed, path)
	}
	return removed, nil
}

func isDashboardBackup(name string) bool {
	return strings.HasPrefix(name, "dashboard-") && strings.HasSuffix(name, ".db")
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// Package store owns SQLite: opening the database, applying migrations, and typed CRUD.
//
// Per ADR-004 this package is persistence only. It never logs and never formats anything for
// HTTP; errors are wrapped with %w and returned, and the API layer logs each failure exactly
// once at the boundary.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, no CGO (ADR-001)
)

// Sentinel errors. The API layer maps these to status codes; see docs/API.md section 1.
var (
	// ErrNotFound is returned when a row addressed by id or key does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned on a unique-constraint violation or a stale optimistic version.
	ErrConflict = errors.New("conflict")
)

// Store is the single handle onto the database. Methods are grouped by table across the files
// in this package; handlers depend on this one concrete type (ADR-004).
type Store struct {
	db *sql.DB

	// now supplies timestamps for created_at, done_at and friends. Tests replace it to get
	// deterministic values; production leaves it as time.Now.
	now func() time.Time
}

// Open opens (creating if needed) the SQLite database at path and applies the PRAGMAs required
// by docs/DATABASE.md section 1.
//
// foreign_keys must be ON for every connection: SQLite defaults it off, and the ON DELETE SET
// NULL rules that protect history are silently inert without it. It is set through the DSN so
// that every pooled connection gets it, not just the first.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"+
			"&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		path,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}

	// One writer, one user (ADR-001). Serialising through a single connection removes any
	// chance of SQLITE_BUSY between our own pooled connections.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", path, err)
	}

	s := &Store{db: db, now: time.Now}

	if err := s.verifyPragmas(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close sqlite: %w", err)
	}
	return nil
}

// DB exposes the handle for the backup ticker's VACUUM INTO. Nothing else should use it.
func (s *Store) DB() *sql.DB { return s.db }

// SetClock replaces the time source. Intended for tests.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// verifyPragmas fails loudly if the DSN did not take effect. A silently-off foreign_keys
// pragma disables the ON DELETE SET NULL rules, so it is checked rather than assumed.
func (s *Store) verifyPragmas() error {
	var foreignKeys int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("read foreign_keys pragma: %w", err)
	}
	if foreignKeys != 1 {
		return errors.New("store: foreign_keys pragma is off")
	}

	var journalMode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("read journal_mode pragma: %w", err)
	}
	if journalMode != "wal" {
		return fmt.Errorf("store: journal_mode is %q, want wal", journalMode)
	}

	return nil
}

// utcNow returns the current time as an RFC3339 UTC string, the storage format for every
// timestamp column (docs/DATABASE.md section 1).
func (s *Store) utcNow() string {
	return s.now().UTC().Format(time.RFC3339)
}

// execer is satisfied by both *sql.DB and *sql.Tx, so query helpers can be shared between
// transactional and single-statement paths.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// tx runs fn inside a transaction, rolling back on error or panic.
func (s *Store) tx(ctx context.Context, fn func(execer) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

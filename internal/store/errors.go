package store

import (
	"database/sql"
	"errors"
	"fmt"

	sqlitelib "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrConstraint is returned when a CHECK, NOT NULL or foreign-key constraint rejects a write.
// Handlers validate before calling the store, so reaching this means the request got past
// validation: the API layer maps it to 400 rather than 500 so the cause is visible.
var ErrConstraint = errors.New("constraint violation")

// classify converts a driver error into one of the package sentinels, preserving the original
// via %w so the API layer can log the full chain (ADR-004).
func classify(op string, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	}

	var serr *sqlitelib.Error
	if errors.As(err, &serr) {
		switch serr.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return fmt.Errorf("%s: %w: %v", op, ErrConflict, err)
		case sqlite3.SQLITE_CONSTRAINT_CHECK,
			sqlite3.SQLITE_CONSTRAINT_NOTNULL,
			sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY,
			sqlite3.SQLITE_CONSTRAINT_TRIGGER,
			sqlite3.SQLITE_CONSTRAINT:
			return fmt.Errorf("%s: %w: %v", op, ErrConstraint, err)
		}
	}

	return fmt.Errorf("%s: %w", op, err)
}

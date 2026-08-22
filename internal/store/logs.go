package store

import (
	"context"
	"fmt"
	"strings"
)

// defaultLogLimit and maxLogLimit bound the feed page size (docs/API.md, GET /api/logs).
const (
	defaultLogLimit = 200
	maxLogLimit     = 500
)

// ListLogEntries returns the accomplishment feed, newest first.
//
// Day grouping is the client's job: it needs the Europe/Lisbon date of occurred_at, and the
// store deals only in UTC (docs/DATABASE.md section 1).
func (s *Store) ListLogEntries(ctx context.Context, f LogFilter) ([]LogEntry, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultLogLimit
	}
	if limit > maxLogLimit {
		limit = maxLogLimit
	}

	query := `
		SELECT le.id, le.category_id, le.title, le.note, le.url, le.goal_id,
			le.occurred_at, le.created_at, c.label, c.icon, g.code
		FROM log_entries le
		JOIN categories c ON c.id = le.category_id
		LEFT JOIN goals g ON g.id = le.goal_id`

	var (
		where []string
		args  []any
	)

	if len(f.Categories) > 0 {
		where = append(where, "le.category_id IN ("+placeholders(len(f.Categories))+")")
		for _, c := range f.Categories {
			args = append(args, c)
		}
	}

	if f.From != "" {
		where = append(where, "le.occurred_at >= ?")
		args = append(args, f.From)
	}

	if f.To != "" {
		where = append(where, "le.occurred_at <= ?")
		args = append(args, f.To)
	}

	if f.Cursor != "" {
		where = append(where, "le.occurred_at < ?")
		args = append(args, f.Cursor)
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	query += " ORDER BY le.occurred_at DESC, le.id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, classify("list log entries", err)
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.CategoryID, &e.Title, &e.Note, &e.URL, &e.GoalID,
			&e.OccurredAt, &e.CreatedAt, &e.CategoryLabel, &e.Icon, &e.GoalCode); err != nil {
			return nil, classify("scan log entry", err)
		}
		out = append(out, e)
	}

	return out, classify("list log entries", rows.Err())
}

// CreateLogEntry appends one accomplishment. This is the path the quick-log sheet optimises.
func (s *Store) CreateLogEntry(ctx context.Context, in NewLogEntry) (LogEntry, error) {
	occurredAt := in.OccurredAt
	if occurredAt == "" {
		occurredAt = s.utcNow()
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO log_entries (category_id, title, note, url, goal_id, occurred_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.CategoryID, in.Title, in.Note, in.URL, in.GoalID, occurredAt, s.utcNow(),
	)
	if err != nil {
		return LogEntry{}, classify("insert log entry", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return LogEntry{}, classify("insert log entry id", err)
	}

	return s.GetLogEntry(ctx, id)
}

// GetLogEntry returns one entry with its category and goal joined for display.
func (s *Store) GetLogEntry(ctx context.Context, id int64) (LogEntry, error) {
	var e LogEntry
	err := s.db.QueryRowContext(ctx, `
		SELECT le.id, le.category_id, le.title, le.note, le.url, le.goal_id,
			le.occurred_at, le.created_at, c.label, c.icon, g.code
		FROM log_entries le
		JOIN categories c ON c.id = le.category_id
		LEFT JOIN goals g ON g.id = le.goal_id
		WHERE le.id = ?`, id,
	).Scan(&e.ID, &e.CategoryID, &e.Title, &e.Note, &e.URL, &e.GoalID,
		&e.OccurredAt, &e.CreatedAt, &e.CategoryLabel, &e.Icon, &e.GoalCode)
	if err != nil {
		return LogEntry{}, classify(fmt.Sprintf("get log entry %d", id), err)
	}

	return e, nil
}

// DeleteLogEntry removes an entry. This is the only mutation the log supports: entries are
// never editable, because the log is a record of what happened.
func (s *Store) DeleteLogEntry(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM log_entries WHERE id = ?`, id)
	if err != nil {
		return classify(fmt.Sprintf("delete log entry %d", id), err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return classify("delete log entry rows", err)
	}

	if affected == 0 {
		return fmt.Errorf("delete log entry %d: %w", id, ErrNotFound)
	}

	return nil
}

// SummarizeLogs counts entries per category between two inclusive RFC3339 UTC bounds. Empty
// bounds mean unbounded.
//
// Categories with a zero count are included so the recap card has a stable shape.
func (s *Store) SummarizeLogs(ctx context.Context, from, to string) ([]CategoryCount, error) {
	query := `
		SELECT c.id, c.label, c.icon, count(le.id)
		FROM categories c
		LEFT JOIN log_entries le ON le.category_id = c.id`

	var (
		conds []string
		args  []any
	)

	if from != "" {
		conds = append(conds, "le.occurred_at >= ?")
		args = append(args, from)
	}

	if to != "" {
		conds = append(conds, "le.occurred_at <= ?")
		args = append(args, to)
	}

	// The bounds belong in the JOIN, not a WHERE: a WHERE would drop the zero-count
	// categories the LEFT JOIN exists to keep.
	if len(conds) > 0 {
		query += " AND " + strings.Join(conds, " AND ")
	}

	query += " GROUP BY c.id, c.label, c.icon, c.sort_order ORDER BY c.sort_order"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, classify("summarize logs", err)
	}
	defer rows.Close()

	var out []CategoryCount
	for rows.Next() {
		var c CategoryCount
		if err := rows.Scan(&c.CategoryID, &c.Label, &c.Icon, &c.Count); err != nil {
			return nil, classify("scan log summary", err)
		}
		out = append(out, c)
	}

	return out, classify("summarize logs", rows.Err())
}

// LogDays returns the distinct UTC timestamps of every log entry, newest first. The streak
// calculation converts them to Europe/Lisbon days; that conversion is a pure function and does
// not belong here (ADR-004).
func (s *Store) LogDays(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT occurred_at FROM log_entries ORDER BY occurred_at DESC`)
	if err != nil {
		return nil, classify("list log days", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var ts string
		if err := rows.Scan(&ts); err != nil {
			return nil, classify("scan log day", err)
		}
		out = append(out, ts)
	}

	return out, classify("list log days", rows.Err())
}

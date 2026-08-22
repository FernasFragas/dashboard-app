package store

import (
	"context"
	"strings"
)

const defaultMetricLimit = 100

// ListMetrics returns readings newest first, optionally narrowed by name and time bounds.
func (s *Store) ListMetrics(ctx context.Context, f MetricFilter) ([]Metric, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultMetricLimit
	}

	query := `SELECT id, name, value, unit, note, recorded_at FROM metrics`

	var (
		where []string
		args  []any
	)

	if f.Name != "" {
		where = append(where, "name = ?")
		args = append(args, f.Name)
	}

	if f.From != "" {
		where = append(where, "recorded_at >= ?")
		args = append(args, f.From)
	}

	if f.To != "" {
		where = append(where, "recorded_at <= ?")
		args = append(args, f.To)
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	query += " ORDER BY recorded_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, classify("list metrics", err)
	}
	defer rows.Close()

	var out []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ID, &m.Name, &m.Value, &m.Unit, &m.Note, &m.RecordedAt); err != nil {
			return nil, classify("scan metric", err)
		}
		out = append(out, m)
	}

	return out, classify("list metrics", rows.Err())
}

// CreateMetric records one reading.
//
// Name is deliberately unvalidated against metric_defs: the Review screen's "other..." field
// must accept a new metric without a migration (docs/DATABASE.md section 4.10). Spelling
// consistency is the picker's job.
func (s *Store) CreateMetric(ctx context.Context, in NewMetric) (Metric, error) {
	recordedAt := in.RecordedAt
	if recordedAt == "" {
		recordedAt = s.utcNow()
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO metrics (name, value, unit, note, recorded_at) VALUES (?, ?, ?, ?, ?)`,
		in.Name, in.Value, in.Unit, in.Note, recordedAt,
	)
	if err != nil {
		return Metric{}, classify("insert metric", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Metric{}, classify("insert metric id", err)
	}

	var m Metric
	err = s.db.QueryRowContext(ctx,
		`SELECT id, name, value, unit, note, recorded_at FROM metrics WHERE id = ?`, id,
	).Scan(&m.ID, &m.Name, &m.Value, &m.Unit, &m.Note, &m.RecordedAt)
	if err != nil {
		return Metric{}, classify("read created metric", err)
	}

	return m, nil
}

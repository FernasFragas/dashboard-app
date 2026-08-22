package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Reference data: weeks, rhythm, categories and metric_defs. All seeded, read-only at runtime.

// ListWeeks returns every plan window in chronological order.
func (s *Store) ListWeeks(ctx context.Context) ([]Week, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT code, phase, start_date, end_date, focus, sort_order
		FROM weeks ORDER BY sort_order`)
	if err != nil {
		return nil, classify("list weeks", err)
	}
	defer rows.Close()

	var weeks []Week
	for rows.Next() {
		var w Week
		if err := rows.Scan(&w.Code, &w.Phase, &w.StartDate, &w.EndDate, &w.Focus, &w.SortOrder); err != nil {
			return nil, classify("scan week", err)
		}
		weeks = append(weeks, w)
	}

	return weeks, classify("list weeks", rows.Err())
}

// GetWeek returns one plan window by code.
func (s *Store) GetWeek(ctx context.Context, code string) (Week, error) {
	var w Week
	err := s.db.QueryRowContext(ctx, `
		SELECT code, phase, start_date, end_date, focus, sort_order
		FROM weeks WHERE code = ?`, code,
	).Scan(&w.Code, &w.Phase, &w.StartDate, &w.EndDate, &w.Focus, &w.SortOrder)
	if err != nil {
		return Week{}, classify(fmt.Sprintf("get week %q", code), err)
	}

	return w, nil
}

// ListRhythm returns the operating-system table in Monday-first order.
func (s *Store) ListRhythm(ctx context.Context) ([]Rhythm, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, label, weekdays, slot, sort_order FROM rhythm ORDER BY sort_order`)
	if err != nil {
		return nil, classify("list rhythm", err)
	}
	defer rows.Close()

	var out []Rhythm
	for rows.Next() {
		var r Rhythm
		if err := rows.Scan(&r.ID, &r.Label, &r.Weekdays, &r.Slot, &r.SortOrder); err != nil {
			return nil, classify("scan rhythm", err)
		}
		out = append(out, r)
	}

	return out, classify("list rhythm", rows.Err())
}

// RhythmForWeekday returns the slot covering the given ISO weekday (1=Mon .. 7=Sun).
// It returns ErrNotFound when no row covers that day.
//
// Matching happens in Go rather than SQL: there are six rows, and a LIKE over a CSV column is
// harder to read than a split.
func (s *Store) RhythmForWeekday(ctx context.Context, weekday int) (Rhythm, error) {
	if weekday < 1 || weekday > 7 {
		return Rhythm{}, fmt.Errorf("rhythm for weekday %d: %w", weekday, ErrNotFound)
	}

	all, err := s.ListRhythm(ctx)
	if err != nil {
		return Rhythm{}, err
	}

	want := strconv.Itoa(weekday)
	for _, r := range all {
		for _, day := range strings.Split(r.Weekdays, ",") {
			if strings.TrimSpace(day) == want {
				return r, nil
			}
		}
	}

	return Rhythm{}, fmt.Errorf("rhythm for weekday %d: %w", weekday, ErrNotFound)
}

// ListCategories returns the eight quick-log buttons in grid order.
func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, label, icon, sort_order FROM categories ORDER BY sort_order`)
	if err != nil {
		return nil, classify("list categories", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Label, &c.Icon, &c.SortOrder); err != nil {
			return nil, classify("scan category", err)
		}
		out = append(out, c)
	}

	return out, classify("list categories", rows.Err())
}

// ListMetricDefs returns the metric-name catalogue in plan order.
func (s *Store) ListMetricDefs(ctx context.Context) ([]MetricDef, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, unit, baseline, target, sort_order FROM metric_defs ORDER BY sort_order`)
	if err != nil {
		return nil, classify("list metric defs", err)
	}
	defer rows.Close()

	var out []MetricDef
	for rows.Next() {
		var m MetricDef
		if err := rows.Scan(&m.Name, &m.Unit, &m.Baseline, &m.Target, &m.SortOrder); err != nil {
			return nil, classify("scan metric def", err)
		}
		out = append(out, m)
	}

	return out, classify("list metric defs", rows.Err())
}

package store

import (
	"context"
	"fmt"
)

// GetReview returns the daily review for one Europe/Lisbon calendar date.
func (s *Store) GetReview(ctx context.Context, date string) (DailyReview, error) {
	var r DailyReview
	err := s.db.QueryRowContext(ctx, `
		SELECT date, learned, issue, "next", minutes, created_at, updated_at
		FROM daily_reviews WHERE date = ?`, date,
	).Scan(&r.Date, &r.Learned, &r.Issue, &r.Next, &r.Minutes, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return DailyReview{}, classify(fmt.Sprintf("get review %s", date), err)
	}

	return r, nil
}

// ListReviews returns reviews between two inclusive YYYY-MM-DD bounds, newest first. Empty
// bounds mean unbounded.
func (s *Store) ListReviews(ctx context.Context, from, to string) ([]DailyReview, error) {
	query := `SELECT date, learned, issue, "next", minutes, created_at, updated_at
		FROM daily_reviews`

	var args []any

	switch {
	case from != "" && to != "":
		query += " WHERE date BETWEEN ? AND ?"
		args = append(args, from, to)
	case from != "":
		query += " WHERE date >= ?"
		args = append(args, from)
	case to != "":
		query += " WHERE date <= ?"
		args = append(args, to)
	}

	query += " ORDER BY date DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, classify("list reviews", err)
	}
	defer rows.Close()

	var out []DailyReview
	for rows.Next() {
		var r DailyReview
		if err := rows.Scan(&r.Date, &r.Learned, &r.Issue, &r.Next,
			&r.Minutes, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, classify("scan review", err)
		}
		out = append(out, r)
	}

	return out, classify("list reviews", rows.Err())
}

// UpsertReview writes the 3-bullet review for a date, creating or replacing it.
//
// The date is the key, so this is never a create-or-conflict: a Sunday review written on
// Monday simply replaces whatever was there. created_at is preserved across updates.
func (s *Store) UpsertReview(ctx context.Context, r DailyReview) (DailyReview, error) {
	now := s.utcNow()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_reviews (date, learned, issue, "next", minutes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (date) DO UPDATE SET
			learned    = excluded.learned,
			issue      = excluded.issue,
			"next"     = excluded."next",
			minutes    = excluded.minutes,
			updated_at = excluded.updated_at`,
		r.Date, r.Learned, r.Issue, r.Next, r.Minutes, now, now,
	)
	if err != nil {
		return DailyReview{}, classify(fmt.Sprintf("upsert review %s", r.Date), err)
	}

	return s.GetReview(ctx, r.Date)
}

// ReviewDates returns every date that has a review, newest first. Used with LogDays by the
// pure streak function.
func (s *Store) ReviewDates(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date FROM daily_reviews ORDER BY date DESC`)
	if err != nil {
		return nil, classify("list review dates", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, classify("scan review date", err)
		}
		out = append(out, d)
	}

	return out, classify("list review dates", rows.Err())
}

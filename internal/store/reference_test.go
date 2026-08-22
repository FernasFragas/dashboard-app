package store

import (
	"context"
	"errors"
	"testing"
)

func TestListWeeksAndGetWeek(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	weeks, err := s.ListWeeks(ctx)
	if err != nil {
		t.Fatalf("list weeks: %v", err)
	}

	if len(weeks) != 3 {
		t.Fatalf("weeks = %d, want 3 fixtures", len(weeks))
	}
	if weeks[0].Code != "W1" {
		t.Errorf("first week = %q, want W1 (chronological order)", weeks[0].Code)
	}

	w, err := s.GetWeek(ctx, "W5")
	if err != nil {
		t.Fatalf("get week: %v", err)
	}
	if w.StartDate != "2026-09-21" {
		t.Errorf("start_date = %q, want 2026-09-21", w.StartDate)
	}

	if _, err := s.GetWeek(ctx, "W99"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown week error = %v, want ErrNotFound", err)
	}
}

// A window whose end precedes its start is rejected by the table CHECK.
func TestWeekDatesMustBeOrdered(t *testing.T) {
	s := newTestStore(t)

	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO weeks (code, phase, start_date, end_date, focus, sort_order)
		VALUES ('W9', 'P1', '2026-10-25', '2026-10-19', NULL, 9)`)
	if !errors.Is(classify("insert week", err), ErrConstraint) {
		t.Errorf("error = %v, want ErrConstraint", err)
	}
}

func TestListCategories(t *testing.T) {
	categories, err := newTestStore(t).ListCategories(context.Background())
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}

	if len(categories) != 3 {
		t.Fatalf("categories = %d, want 3 fixtures", len(categories))
	}
	if categories[0].ID != "application" {
		t.Errorf("first category = %q, want application (grid order)", categories[0].ID)
	}
}

// The CHECK on the seeded slug turns a typo into a boot failure rather than an orphan category.
func TestCategoryIDIsConstrained(t *testing.T) {
	s := newTestStore(t)

	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO categories (id, label, icon, sort_order) VALUES ('typo', 'Typo', '?', 9)`)
	if !errors.Is(classify("insert category", err), ErrConstraint) {
		t.Errorf("error = %v, want ErrConstraint", err)
	}
}

// The rhythm lookup must resolve a grouped label like "Tue-Wed" to both of its weekdays.
func TestRhythmForWeekday(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	tests := []struct {
		name      string
		weekday   int
		wantLabel string
		wantErr   bool
	}{
		{"monday", 1, "Mon", false},
		{"tuesday in a group", 2, "Tue–Wed", false},
		{"wednesday in a group", 3, "Tue–Wed", false},
		{"uncovered day", 5, "", true},
		{"below range", 0, "", true},
		{"above range", 8, "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.RhythmForWeekday(ctx, tc.weekday)

			if tc.wantErr {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("error = %v, want ErrNotFound", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("rhythm for weekday %d: %v", tc.weekday, err)
			}
			if got.Label != tc.wantLabel {
				t.Errorf("label = %q, want %q", got.Label, tc.wantLabel)
			}
		})
	}
}

func TestListRhythm(t *testing.T) {
	rows, err := newTestStore(t).ListRhythm(context.Background())
	if err != nil {
		t.Fatalf("list rhythm: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("rhythm rows = %d, want 2 fixtures", len(rows))
	}
	if rows[1].Weekdays != "2,3" {
		t.Errorf("weekdays = %q, want the CSV 2,3", rows[1].Weekdays)
	}
}

// The loader matches rhythm rows on label, so the column must be unique.
func TestRhythmLabelIsUnique(t *testing.T) {
	s := newTestStore(t)

	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO rhythm (label, weekdays, slot, sort_order) VALUES ('Mon', '1', 'again', 9)`)
	if !errors.Is(classify("insert rhythm", err), ErrConflict) {
		t.Errorf("error = %v, want ErrConflict", err)
	}
}

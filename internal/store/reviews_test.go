package store

import (
	"context"
	"errors"
	"testing"
)

func TestUpsertReviewCreatesAndUpdates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	created, err := s.UpsertReview(ctx, DailyReview{
		Date:    "2026-09-24",
		Learned: ptr("WAL checkpointing blocks on a long read txn"),
		Issue:   ptr("k6 run was CPU-bound"),
		Next:    ptr("move load gen"),
		Minutes: ptr(45),
	})
	if err != nil {
		t.Fatalf("create review: %v", err)
	}

	if created.Date != "2026-09-24" {
		t.Errorf("date = %q, want 2026-09-24", created.Date)
	}
	if created.Minutes == nil || *created.Minutes != 45 {
		t.Errorf("minutes = %v, want 45", created.Minutes)
	}

	// The date is the key: writing it again replaces rather than duplicating.
	updated, err := s.UpsertReview(ctx, DailyReview{
		Date:    "2026-09-24",
		Learned: ptr("revised"),
	})
	if err != nil {
		t.Fatalf("update review: %v", err)
	}

	if *updated.Learned != "revised" {
		t.Errorf("learned = %q, want the replaced value", *updated.Learned)
	}
	if updated.Issue != nil {
		t.Errorf("issue = %v, want nil after replacement", *updated.Issue)
	}

	if got := countRows(t, s, "daily_reviews"); got != 1 {
		t.Errorf("daily_reviews rows = %d, want 1", got)
	}
}

// An entirely empty review must not count toward the streak, so the schema refuses to store it.
func TestUpsertReviewRejectsEmpty(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	_, err := s.UpsertReview(ctx, DailyReview{Date: "2026-09-24"})
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("error = %v, want ErrConstraint", err)
	}

	_, err = s.UpsertReview(ctx, DailyReview{
		Date: "2026-09-24", Learned: ptr(""), Issue: ptr(""), Next: ptr(""),
	})
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("all-blank error = %v, want ErrConstraint", err)
	}
}

func TestUpsertReviewRejectsNegativeMinutes(t *testing.T) {
	_, err := newTestStore(t).UpsertReview(context.Background(), DailyReview{
		Date: "2026-09-24", Learned: ptr("x"), Minutes: ptr(-5),
	})
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("error = %v, want ErrConstraint", err)
	}
}

// A Sunday review written on Monday must land on Sunday - the backfill the streak depends on.
func TestUpsertReviewBackdated(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	backfilled, err := s.UpsertReview(ctx, DailyReview{
		Date: "2026-09-20", Learned: ptr("Sunday's bullets, written Monday"),
	})
	if err != nil {
		t.Fatalf("backfill review: %v", err)
	}

	if backfilled.Date != "2026-09-20" {
		t.Errorf("date = %q, want the backdated day", backfilled.Date)
	}
}

func TestGetReviewMissing(t *testing.T) {
	_, err := newTestStore(t).GetReview(context.Background(), "2026-01-01")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestListReviewsRanges(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for _, date := range []string{"2026-09-20", "2026-09-21", "2026-09-22"} {
		if _, err := s.UpsertReview(ctx, DailyReview{Date: date, Learned: ptr("x")}); err != nil {
			t.Fatalf("upsert %s: %v", date, err)
		}
	}

	tests := []struct {
		name      string
		from, to  string
		wantCount int
	}{
		{"unbounded", "", "", 3},
		{"from only", "2026-09-21", "", 2},
		{"to only", "", "2026-09-21", 2},
		{"both", "2026-09-21", "2026-09-21", 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ListReviews(ctx, tc.from, tc.to)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("reviews = %d, want %d", len(got), tc.wantCount)
			}
		})
	}

	all, err := s.ListReviews(ctx, "", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if all[0].Date != "2026-09-22" {
		t.Errorf("first = %q, want the newest", all[0].Date)
	}
}

func TestReviewDates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for _, date := range []string{"2026-09-20", "2026-09-22"} {
		if _, err := s.UpsertReview(ctx, DailyReview{Date: date, Learned: ptr("x")}); err != nil {
			t.Fatalf("upsert %s: %v", date, err)
		}
	}

	dates, err := s.ReviewDates(ctx)
	if err != nil {
		t.Fatalf("review dates: %v", err)
	}

	if len(dates) != 2 || dates[0] != "2026-09-22" {
		t.Errorf("dates = %v, want newest first", dates)
	}
}

package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func createEntry(t *testing.T, s *Store, category, title, occurredAt string) LogEntry {
	t.Helper()

	e, err := s.CreateLogEntry(context.Background(), NewLogEntry{
		CategoryID: category, Title: title, OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatalf("create log entry %q: %v", title, err)
	}

	return e
}

func TestCreateLogEntryDefaultsOccurredAtToNow(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	e, err := s.CreateLogEntry(ctx, NewLogEntry{
		CategoryID: "application",
		Title:      "Applied — Platform Engineer @ Acme",
		URL:        ptr("https://acme.example/careers/1234"),
	})
	if err != nil {
		t.Fatalf("create log entry: %v", err)
	}

	want := fixedNow.Format("2006-01-02T15:04:05Z")
	if e.OccurredAt != want {
		t.Errorf("occurred_at = %q, want %q", e.OccurredAt, want)
	}
	if e.CategoryLabel != "Application" || e.Icon != "📮" {
		t.Errorf("category join = %q/%q, want Application/📮", e.CategoryLabel, e.Icon)
	}
	if e.URL == nil || *e.URL != "https://acme.example/careers/1234" {
		t.Errorf("url = %v, want the submitted link", e.URL)
	}
}

// A backdated entry keeps created_at at "now" while occurred_at moves - that difference is what
// makes the Log feed honest about when something happened.
func TestCreateLogEntryBackdated(t *testing.T) {
	s := newTestStore(t)

	e := createEntry(t, s, "module", "DDIA ch.3", "2026-09-20T10:00:00Z")

	if e.OccurredAt != "2026-09-20T10:00:00Z" {
		t.Errorf("occurred_at = %q, want the backdated value", e.OccurredAt)
	}
	if e.CreatedAt != fixedNow.Format("2006-01-02T15:04:05Z") {
		t.Errorf("created_at = %q, want now", e.CreatedAt)
	}
}

func TestCreateLogEntryRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	tests := []struct {
		name string
		in   NewLogEntry
	}{
		{"unknown category", NewLogEntry{CategoryID: "nope", Title: "x"}},
		{"blank title", NewLogEntry{CategoryID: "application", Title: "   "}},
		{"unknown goal", NewLogEntry{CategoryID: "application", Title: "x", GoalID: ptr(int64(999))}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.CreateLogEntry(ctx, tc.in); !errors.Is(err, ErrConstraint) {
				t.Errorf("error = %v, want ErrConstraint", err)
			}
		})
	}
}

// Categories are seeded and must never be deleted out from under history: the RESTRICT rule
// is what stops it.
func TestCategoryDeleteIsRestricted(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createEntry(t, s, "application", "Applied", "")

	_, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id = 'application'`)
	if !errors.Is(classify("delete category", err), ErrConstraint) {
		t.Errorf("error = %v, want a foreign-key constraint violation", err)
	}
}

func TestListLogEntriesOrderAndFilters(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createEntry(t, s, "application", "oldest", "2026-09-20T09:00:00Z")
	createEntry(t, s, "module", "middle", "2026-09-21T09:00:00Z")
	createEntry(t, s, "application", "newest", "2026-09-22T09:00:00Z")

	all, err := s.ListLogEntries(ctx, LogFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("entries = %d, want 3", len(all))
	}
	if all[0].Title != "newest" || all[2].Title != "oldest" {
		t.Errorf("order = %q..%q, want newest first", all[0].Title, all[2].Title)
	}

	byCategory, err := s.ListLogEntries(ctx, LogFilter{Categories: []string{"application"}})
	if err != nil {
		t.Fatalf("list by category: %v", err)
	}
	if len(byCategory) != 2 {
		t.Errorf("application entries = %d, want 2", len(byCategory))
	}

	bounded, err := s.ListLogEntries(ctx, LogFilter{
		From: "2026-09-21T00:00:00Z", To: "2026-09-21T23:59:59Z",
	})
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(bounded) != 1 || bounded[0].Title != "middle" {
		t.Errorf("bounded entries = %v, want just the middle one", bounded)
	}
}

func TestListLogEntriesPaginates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for i := 1; i <= 5; i++ {
		createEntry(t, s, "application", "entry", time.Date(2026, 9, 20+i, 9, 0, 0, 0, time.UTC).
			Format(time.RFC3339))
	}

	first, err := s.ListLogEntries(ctx, LogFilter{Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page size = %d, want 2", len(first))
	}

	second, err := s.ListLogEntries(ctx, LogFilter{Limit: 2, Cursor: first[len(first)-1].OccurredAt})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("second page size = %d, want 2", len(second))
	}

	if second[0].OccurredAt >= first[len(first)-1].OccurredAt {
		t.Errorf("cursor did not advance: %q then %q", first[len(first)-1].OccurredAt, second[0].OccurredAt)
	}
}

func TestListLogEntriesLimitIsClamped(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createEntry(t, s, "application", "one", "")

	// A limit beyond the maximum must not error; it is clamped.
	entries, err := s.ListLogEntries(ctx, LogFilter{Limit: maxLogLimit + 1000})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %d, want 1", len(entries))
	}
}

func TestDeleteLogEntry(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	e := createEntry(t, s, "application", "typo", "")

	if err := s.DeleteLogEntry(ctx, e.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if err := s.DeleteLogEntry(ctx, e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete error = %v, want ErrNotFound", err)
	}
}

// Zero-count categories are included so the recap card has a stable shape.
func TestSummarizeLogsIncludesEmptyCategories(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createEntry(t, s, "application", "a", "2026-09-22T09:00:00Z")
	createEntry(t, s, "application", "b", "2026-09-22T10:00:00Z")
	createEntry(t, s, "module", "c", "2026-09-22T11:00:00Z")

	counts, err := s.SummarizeLogs(ctx, "", "")
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}

	if len(counts) != 3 {
		t.Fatalf("categories = %d, want all 3 fixtures", len(counts))
	}

	got := map[string]int{}
	for _, c := range counts {
		got[c.CategoryID] = c.Count
	}

	if got["application"] != 2 || got["module"] != 1 || got["number"] != 0 {
		t.Errorf("counts = %v, want application:2 module:1 number:0", got)
	}
}

// The bounds live in the JOIN, not a WHERE: a WHERE would drop the zero-count rows the LEFT
// JOIN exists to keep.
func TestSummarizeLogsBoundedKeepsZeroRows(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createEntry(t, s, "application", "in range", "2026-09-22T09:00:00Z")
	createEntry(t, s, "module", "out of range", "2026-08-01T09:00:00Z")

	counts, err := s.SummarizeLogs(ctx, "2026-09-21T00:00:00Z", "2026-09-27T23:59:59Z")
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}

	if len(counts) != 3 {
		t.Fatalf("categories = %d, want all 3 even when empty in range", len(counts))
	}

	got := map[string]int{}
	for _, c := range counts {
		got[c.CategoryID] = c.Count
	}

	if got["application"] != 1 || got["module"] != 0 {
		t.Errorf("counts = %v, want application:1 module:0", got)
	}
}

// Rule 7: a log at 23:30Z and one at 00:30Z must land on the correct Europe/Lisbon days.
//
// The store deals only in UTC; the caller supplies bounds derived from the Lisbon day. This
// test pins the conversion that the Log feed and the streak both depend on - in September
// Lisbon is UTC+1, so 23:30Z on the 22nd is already the 23rd locally.
func TestLisbonDayBoundary(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	lisbon, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		t.Fatalf("load Europe/Lisbon: %v", err)
	}

	lateEvening := createEntry(t, s, "application", "23:30Z", "2026-09-22T23:30:00Z")
	afterMidnight := createEntry(t, s, "module", "00:30Z", "2026-09-23T00:30:00Z")

	// Both timestamps fall on 2026-09-23 in Lisbon: UTC+1 pushes the first over midnight.
	for _, e := range []LogEntry{lateEvening, afterMidnight} {
		ts, err := time.Parse(time.RFC3339, e.OccurredAt)
		if err != nil {
			t.Fatalf("parse %q: %v", e.OccurredAt, err)
		}

		if got := ts.In(lisbon).Format("2006-01-02"); got != "2026-09-23" {
			t.Errorf("entry %q is on Lisbon day %s, want 2026-09-23", e.Title, got)
		}
	}

	// Querying the Lisbon day 2026-09-23 as UTC bounds must return both.
	dayStart := time.Date(2026, 9, 23, 0, 0, 0, 0, lisbon).UTC().Format(time.RFC3339)
	dayEnd := time.Date(2026, 9, 23, 23, 59, 59, 0, lisbon).UTC().Format(time.RFC3339)

	entries, err := s.ListLogEntries(ctx, LogFilter{From: dayStart, To: dayEnd})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("entries on Lisbon 2026-09-23 = %d, want 2", len(entries))
	}

	// The same instants queried as a naive UTC day would split across two days - the bug this
	// rule exists to prevent.
	utcDay, err := s.ListLogEntries(ctx, LogFilter{
		From: "2026-09-23T00:00:00Z", To: "2026-09-23T23:59:59Z",
	})
	if err != nil {
		t.Fatalf("list utc day: %v", err)
	}

	if len(utcDay) != 1 {
		t.Errorf("entries on the naive UTC day = %d, want 1 - the split this rule prevents", len(utcDay))
	}
}

func TestLogDays(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createEntry(t, s, "application", "a", "2026-09-20T09:00:00Z")
	createEntry(t, s, "module", "b", "2026-09-22T09:00:00Z")

	days, err := s.LogDays(ctx)
	if err != nil {
		t.Fatalf("log days: %v", err)
	}

	if len(days) != 2 {
		t.Fatalf("days = %d, want 2", len(days))
	}
	if days[0] != "2026-09-22T09:00:00Z" {
		t.Errorf("days[0] = %q, want the newest first", days[0])
	}
}

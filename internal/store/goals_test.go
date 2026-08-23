package store

import (
	"context"
	"errors"
	"testing"
)

func createGoal(t *testing.T, s *Store, title, project, status string) Goal {
	t.Helper()

	g, err := s.CreateGoal(context.Background(), NewGoal{
		Title:   title,
		Project: project,
		Status:  status,
	})
	if err != nil {
		t.Fatalf("create goal %q: %v", title, err)
	}

	return g
}

func TestCreateGoalDefaults(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	g, err := s.CreateGoal(ctx, NewGoal{
		Title:     "Ship the eval CI gate",
		Project:   "synapse",
		DoneMeans: ptr("CI fails on regression"),
		Target:    ptr("W6"),
	})
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}

	if g.Status != "backlog" {
		t.Errorf("status = %q, want backlog", g.Status)
	}
	if g.Version != 1 {
		t.Errorf("version = %d, want 1", g.Version)
	}
	if g.Code != nil {
		t.Errorf("code = %v, want nil: G-numbers belong to the seed", *g.Code)
	}
	if g.CompletedAt != nil {
		t.Errorf("completed_at = %v, want nil", *g.CompletedAt)
	}
	if g.SortOrder != sortStep {
		t.Errorf("sort_order = %d, want %d", g.SortOrder, sortStep)
	}
	if g.CreatedAt != fixedNow.Format("2006-01-02T15:04:05Z") {
		t.Errorf("created_at = %q, want the clock's value", g.CreatedAt)
	}
}

// Rule 5: the project and status CHECK constraints reject unknown enum values.
func TestCreateGoalRejectsBadEnums(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	tests := []struct {
		name string
		in   NewGoal
	}{
		{"unknown project", NewGoal{Title: "x", Project: "not-a-project"}},
		{"unknown status", NewGoal{Title: "x", Project: "dash", Status: "wip"}},
		{"unknown phase", NewGoal{Title: "x", Project: "dash", Phase: ptr("P9")}},
		{"blank title", NewGoal{Title: "   ", Project: "dash"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateGoal(ctx, tc.in)
			if !errors.Is(err, ErrConstraint) {
				t.Errorf("error = %v, want ErrConstraint", err)
			}
		})
	}
}

// Rule 6, first half: a duplicate goal code is a unique violation, surfaced as ErrConflict.
func TestGoalCodeIsUnique(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	insert := func() error {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO goals (code, title, project, status, sort_order, version, created_at)
			VALUES ('G7', 'OSS presence', 'oss', 'backlog', 100, 1, ?)`, s.utcNow())

		return classify("insert goal", err)
	}

	if err := insert(); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	if err := insert(); !errors.Is(err, ErrConflict) {
		t.Errorf("second insert error = %v, want ErrConflict", err)
	}
}

// Rule 6, second half: a stale version conflicts, and a successful update bumps it.
func TestUpdateGoalVersionGuard(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	g := createGoal(t, s, "OSS presence", "oss", "backlog")

	updated, _, err := s.UpdateGoal(ctx, g.ID, g.Version, GoalPatch{Title: ptr("OSS presence v2")})
	if err != nil {
		t.Fatalf("update goal: %v", err)
	}

	if updated.Version != g.Version+1 {
		t.Errorf("version = %d, want %d", updated.Version, g.Version+1)
	}
	if updated.Title != "OSS presence v2" {
		t.Errorf("title = %q, want the patched value", updated.Title)
	}

	// Replaying the original version must now fail.
	_, _, err = s.UpdateGoal(ctx, g.ID, g.Version, GoalPatch{Title: ptr("stale write")})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("stale update error = %v, want ErrConflict", err)
	}

	after, err := s.GetGoal(ctx, g.ID)
	if err != nil {
		t.Fatalf("get goal: %v", err)
	}
	if after.Title != "OSS presence v2" {
		t.Errorf("title = %q, want the stale write to have been rejected", after.Title)
	}
}

func TestUpdateGoalMissing(t *testing.T) {
	s := newTestStore(t)

	_, _, err := s.UpdateGoal(context.Background(), 4242, 1, GoalPatch{Title: ptr("x")})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// Moving into done stamps completed_at; moving out clears it.
func TestUpdateGoalStampsCompletion(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	g := createGoal(t, s, "AWS SAA", "learn", "active")

	done, _, err := s.UpdateGoal(ctx, g.ID, g.Version, GoalPatch{
		Status:   ptr("done"),
		Position: ptr(0),
	})
	if err != nil {
		t.Fatalf("move to done: %v", err)
	}

	if done.CompletedAt == nil {
		t.Fatal("completed_at is nil after moving to done")
	}

	back, _, err := s.UpdateGoal(ctx, done.ID, done.Version, GoalPatch{
		Status:   ptr("active"),
		Position: ptr(0),
	})
	if err != nil {
		t.Fatalf("move back to active: %v", err)
	}

	if back.CompletedAt != nil {
		t.Errorf("completed_at = %v after leaving done, want nil", *back.CompletedAt)
	}
}

// A move sends a 0-based position; the server renumbers the whole column in sparse steps and
// returns every affected card (docs/API.md, PATCH /api/goals/{id}).
func TestUpdateGoalRenumbersColumn(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	first := createGoal(t, s, "first", "dash", "active")
	second := createGoal(t, s, "second", "dash", "active")
	third := createGoal(t, s, "third", "dash", "active")

	// Move the third card to the front of the column.
	moved, reordered, err := s.UpdateGoal(ctx, third.ID, third.Version, GoalPatch{
		Status:   ptr("active"),
		Position: ptr(0),
	})
	if err != nil {
		t.Fatalf("move goal: %v", err)
	}

	if moved.SortOrder != sortStep {
		t.Errorf("moved sort_order = %d, want %d", moved.SortOrder, sortStep)
	}

	wantOrder := []int64{third.ID, first.ID, second.ID}
	if len(reordered) != len(wantOrder) {
		t.Fatalf("reordered length = %d, want %d", len(reordered), len(wantOrder))
	}

	for i, want := range wantOrder {
		if reordered[i].ID != want {
			t.Errorf("reordered[%d].ID = %d, want %d", i, reordered[i].ID, want)
		}
		if got := reordered[i].SortOrder; got != (i+1)*sortStep {
			t.Errorf("reordered[%d].SortOrder = %d, want %d", i, got, (i+1)*sortStep)
		}
	}

	listed, err := s.ListGoals(ctx, GoalFilter{Status: "active"})
	if err != nil {
		t.Fatalf("list goals: %v", err)
	}

	for i, want := range wantOrder {
		if listed[i].ID != want {
			t.Errorf("listed[%d].ID = %d, want %d", i, listed[i].ID, want)
		}
	}
}

// version guards user edits - title, status, project. Renumbering sort_order is a server-side
// consequence of someone else's move, not a competing edit, so it must not bump anything.
//
// When it did, the moved card bumped twice (the main UPDATE plus the renumber), which made the
// frontend's optimistic +1 wrong and turned a quick second move into a spurious 412.
func TestMoveBumpsMovedGoalVersionExactlyOnce(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	first := createGoal(t, s, "first", "dash", "backlog")
	createGoal(t, s, "second", "dash", "backlog")

	moved, _, err := s.UpdateGoal(ctx, first.ID, first.Version, GoalPatch{
		Status:   ptr("active"),
		Position: ptr(0),
	})
	if err != nil {
		t.Fatalf("move goal: %v", err)
	}

	if moved.Version != first.Version+1 {
		t.Errorf("version = %d after one move, want %d", moved.Version, first.Version+1)
	}

	// And the version it reports must be the one a follow-up If-Match can actually use.
	if _, _, err := s.UpdateGoal(ctx, moved.ID, moved.Version, GoalPatch{
		Status:   ptr("done"),
		Position: ptr(0),
	}); err != nil {
		t.Fatalf("second move replaying the returned version: %v", err)
	}
}

// A neighbour being renumbered must not invalidate its ETag: with 18 seeded goals in Backlog,
// moving one card would otherwise conflict every other card on the board.
func TestMoveLeavesOtherGoalVersionsAlone(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	source := []Goal{
		createGoal(t, s, "backlog one", "dash", "backlog"),
		createGoal(t, s, "backlog two", "dash", "backlog"),
		createGoal(t, s, "backlog three", "dash", "backlog"),
	}
	target := []Goal{
		createGoal(t, s, "active one", "dash", "active"),
		createGoal(t, s, "active two", "dash", "active"),
	}

	// Move the middle backlog card into the middle of the active column, so both columns are
	// renumbered and neither is trivially untouched.
	if _, _, err := s.UpdateGoal(ctx, source[1].ID, source[1].Version, GoalPatch{
		Status:   ptr("active"),
		Position: ptr(1),
	}); err != nil {
		t.Fatalf("move goal: %v", err)
	}

	after, err := s.ListGoals(ctx, GoalFilter{})
	if err != nil {
		t.Fatalf("list goals: %v", err)
	}

	versions := make(map[int64]int, len(after))
	for _, goal := range after {
		versions[goal.ID] = goal.Version
	}

	untouched := append(append([]Goal{}, source[0], source[2]), target...)
	for _, goal := range untouched {
		if versions[goal.ID] != goal.Version {
			t.Errorf("goal %q version = %d after a neighbour moved, want %d unchanged",
				goal.Title, versions[goal.ID], goal.Version)
		}
	}

	// The renumbering itself still has to have happened.
	order := columnOrder(t, s, "active")
	if len(order) != 3 {
		t.Fatalf("active column = %v, want 3 cards", order)
	}
	if order[1] != source[1].ID {
		t.Errorf("moved card is at index %d, want 1", indexOf(order, source[1].ID))
	}
}

// A cross-column move must return both columns, so the client can apply one payload and skip
// the follow-up board read.
func TestUpdateGoalReturnsBothColumns(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	backlog := []Goal{
		createGoal(t, s, "backlog one", "dash", "backlog"),
		createGoal(t, s, "backlog two", "dash", "backlog"),
		createGoal(t, s, "backlog three", "dash", "backlog"),
	}
	active := []Goal{
		createGoal(t, s, "active one", "dash", "active"),
		createGoal(t, s, "active two", "dash", "active"),
	}

	moved, reordered, err := s.UpdateGoal(ctx, backlog[0].ID, backlog[0].Version, GoalPatch{
		Status:   ptr("active"),
		Position: ptr(1),
	})
	if err != nil {
		t.Fatalf("move goal: %v", err)
	}

	byID := make(map[int64]Goal, len(reordered))
	for _, goal := range reordered {
		if _, dup := byID[goal.ID]; dup {
			t.Errorf("goal %d appears twice in reordered", goal.ID)
		}
		byID[goal.ID] = goal
	}

	// Target column: the two that were there plus the arrival.
	for _, goal := range append([]Goal{moved}, active...) {
		if _, ok := byID[goal.ID]; !ok {
			t.Errorf("target-column goal %q missing from reordered", goal.Title)
		}
	}

	// Source column: the two left behind, which closed the gap.
	for _, goal := range backlog[1:] {
		got, ok := byID[goal.ID]
		if !ok {
			t.Errorf("source-column goal %q missing from reordered", goal.Title)
			continue
		}
		if got.Status != "backlog" {
			t.Errorf("goal %q status = %q in reordered, want backlog", goal.Title, got.Status)
		}
	}

	// Applying reordered alone must reproduce what a fresh read would show.
	for _, status := range []string{"active", "backlog"} {
		fresh, err := s.ListGoals(ctx, GoalFilter{Status: status})
		if err != nil {
			t.Fatalf("list %s: %v", status, err)
		}

		for _, goal := range fresh {
			payload, ok := byID[goal.ID]
			if !ok {
				t.Errorf("goal %q is in %s but absent from reordered", goal.Title, status)
				continue
			}
			if payload.SortOrder != goal.SortOrder {
				t.Errorf("goal %q sort_order = %d in reordered, want %d",
					goal.Title, payload.SortOrder, goal.SortOrder)
			}
		}
	}
}

// A move within one column touches only that column.
func TestUpdateGoalWithinColumnReturnsOneColumn(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	first := createGoal(t, s, "first", "dash", "active")
	createGoal(t, s, "second", "dash", "active")
	createGoal(t, s, "spectator", "dash", "backlog")

	_, reordered, err := s.UpdateGoal(ctx, first.ID, first.Version, GoalPatch{
		Status:   ptr("active"),
		Position: ptr(1),
	})
	if err != nil {
		t.Fatalf("move goal: %v", err)
	}

	if len(reordered) != 2 {
		t.Fatalf("reordered = %d cards, want just the active column's 2", len(reordered))
	}

	for _, goal := range reordered {
		if goal.Status != "active" {
			t.Errorf("reordered contains a %s card; a same-column move must not touch others",
				goal.Status)
		}
	}
}

func columnOrder(t *testing.T, s *Store, status string) []int64 {
	t.Helper()

	goals, err := s.ListGoals(context.Background(), GoalFilter{Status: status})
	if err != nil {
		t.Fatalf("list %s goals: %v", status, err)
	}

	ids := make([]int64, 0, len(goals))
	for _, goal := range goals {
		ids = append(ids, goal.ID)
	}

	return ids
}

func indexOf(ids []int64, id int64) int {
	for i, candidate := range ids {
		if candidate == id {
			return i
		}
	}
	return -1
}

func TestListGoalsFilters(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createGoal(t, s, "backlog dash", "dash", "backlog")
	createGoal(t, s, "active oss", "oss", "active")
	createGoal(t, s, "active dash", "dash", "active")

	active, err := s.ListGoals(ctx, GoalFilter{Status: "active"})
	if err != nil {
		t.Fatalf("list by status: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("active goals = %d, want 2", len(active))
	}

	dash, err := s.ListGoals(ctx, GoalFilter{Projects: []string{"dash"}})
	if err != nil {
		t.Fatalf("list by project: %v", err)
	}
	if len(dash) != 2 {
		t.Errorf("dash goals = %d, want 2", len(dash))
	}

	both, err := s.ListGoals(ctx, GoalFilter{Status: "active", Projects: []string{"dash"}})
	if err != nil {
		t.Fatalf("list by both: %v", err)
	}
	if len(both) != 1 {
		t.Errorf("active dash goals = %d, want 1", len(both))
	}
}

func TestCountGoalsByStatus(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for i := 0; i < 4; i++ {
		createGoal(t, s, "active goal", "dash", "active")
	}

	n, err := s.CountGoalsByStatus(ctx, "active")
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	// Four active goals exceed the limit of three. The store reports it; nothing blocks it.
	if n != 4 {
		t.Errorf("active count = %d, want 4", n)
	}
}

// Rule 4: deleting a goal must not delete history. Linked tasks and log entries survive with
// goal_id nulled by the ON DELETE SET NULL rules.
func TestDeleteGoalPreservesHistory(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	g := createGoal(t, s, "OSS presence", "oss", "active")

	task, err := s.CreateTask(ctx, NewTask{
		Week: "W1", Title: "Pick an OSS target", Project: "oss", GoalID: &g.ID,
		SkillIDs: []int64{fixtureSkillID},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	entry, err := s.CreateLogEntry(ctx, NewLogEntry{
		CategoryID: "application", Title: "Applied somewhere", GoalID: &g.ID,
	})
	if err != nil {
		t.Fatalf("create log entry: %v", err)
	}

	if err := s.DeleteGoal(ctx, g.ID); err != nil {
		t.Fatalf("delete goal: %v", err)
	}

	survivingTask, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("task did not survive the goal deletion: %v", err)
	}
	if survivingTask.GoalID != nil {
		t.Errorf("task goal_id = %v, want nil", *survivingTask.GoalID)
	}

	survivingEntry, err := s.GetLogEntry(ctx, entry.ID)
	if err != nil {
		t.Fatalf("log entry did not survive the goal deletion: %v", err)
	}
	if survivingEntry.GoalID != nil {
		t.Errorf("log entry goal_id = %v, want nil", *survivingEntry.GoalID)
	}
}

func TestDeleteGoalMissing(t *testing.T) {
	if err := newTestStore(t).DeleteGoal(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// ListGoals reports the linked-log count that drives the "proof of motion" chip.
func TestListGoalsCountsLinkedLogs(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	g := createGoal(t, s, "Career round 1", "career", "active")

	for i := 0; i < 3; i++ {
		if _, err := s.CreateLogEntry(ctx, NewLogEntry{
			CategoryID: "application", Title: "Applied", GoalID: &g.ID,
		}); err != nil {
			t.Fatalf("create log entry: %v", err)
		}
	}

	goals, err := s.ListGoals(ctx, GoalFilter{})
	if err != nil {
		t.Fatalf("list goals: %v", err)
	}

	if goals[0].LogCount != 3 {
		t.Errorf("log_count = %d, want 3", goals[0].LogCount)
	}
}

func TestMoveTo(t *testing.T) {
	tests := []struct {
		name     string
		ids      []int64
		id       int64
		position int
		want     []int64
	}{
		{"to front", []int64{1, 2, 3}, 3, 0, []int64{3, 1, 2}},
		{"to middle", []int64{1, 2, 3}, 1, 1, []int64{2, 1, 3}},
		{"to end", []int64{1, 2, 3}, 1, 2, []int64{2, 3, 1}},
		{"past the end clamps", []int64{1, 2, 3}, 1, 99, []int64{2, 3, 1}},
		{"negative clamps", []int64{1, 2, 3}, 3, -5, []int64{3, 1, 2}},
		{"not present appends", []int64{1, 2}, 9, 1, []int64{1, 9, 2}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := moveTo(tc.ids, tc.id, tc.position)

			if len(got) != len(tc.want) {
				t.Fatalf("length = %d, want %d", len(got), len(tc.want))
			}

			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got %v, want %v", got, tc.want)
					break
				}
			}
		})
	}
}

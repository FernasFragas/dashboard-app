package store

import (
	"context"
	"errors"
	"testing"
)

func createTask(t *testing.T, s *Store, week, title, project string) Task {
	t.Helper()

	task, err := s.CreateTask(context.Background(), NewTask{
		Week: week, Title: title, Project: project, SkillIDs: []int64{fixtureSkillID},
	})
	if err != nil {
		t.Fatalf("create task %q: %v", title, err)
	}

	return task
}

func TestCreateTaskDefaults(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	task, err := s.CreateTask(ctx, NewTask{
		Week:      "W5",
		Title:     "Fail-open on provider timeout",
		Project:   "gateway",
		Steps:     []string{"Add a 2s context deadline."},
		DoneMeans: ptr("a killed provider surfaces no error"),
		SkillIDs:  []int64{fixtureSkillID},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if task.Status != "todo" {
		t.Errorf("status = %q, want todo", task.Status)
	}
	if task.DoneAt != nil {
		t.Errorf("done_at = %v, want nil", *task.DoneAt)
	}
	if task.SeedKey != nil {
		t.Errorf("seed_key = %v, want nil: API-created tasks are invisible to the loader", *task.SeedKey)
	}
	if len(task.Steps) != 1 || task.Steps[0] != "Add a 2s context deadline." {
		t.Errorf("steps = %v, want the round-tripped JSON array", task.Steps)
	}
	if task.Version != 1 {
		t.Errorf("version = %d, want 1", task.Version)
	}
}

// Steps default to an empty array, never null, so the API always encodes a list.
func TestCreateTaskEmptySteps(t *testing.T) {
	s := newTestStore(t)

	task := createTask(t, s, "W1", "Ad-hoc task", "dash")

	if task.Steps == nil {
		t.Fatal("steps = nil, want an empty slice")
	}
	if len(task.Steps) != 0 {
		t.Errorf("steps = %v, want empty", task.Steps)
	}
}

// Rule 5, tasks half: enum and week foreign-key violations are rejected.
func TestCreateTaskRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	tests := []struct {
		name string
		in   NewTask
	}{
		{"unknown project", NewTask{Week: "W1", Title: "x", Project: "nope", SkillIDs: []int64{fixtureSkillID}}},
		{"blank title", NewTask{Week: "W1", Title: "  ", Project: "dash", SkillIDs: []int64{fixtureSkillID}}},
		{"unknown week", NewTask{Week: "W99", Title: "x", Project: "dash", SkillIDs: []int64{fixtureSkillID}}},
		{"no skills", NewTask{Week: "W1", Title: "x", Project: "dash"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.CreateTask(ctx, tc.in); !errors.Is(err, ErrConstraint) {
				t.Errorf("error = %v, want ErrConstraint", err)
			}
		})
	}
}

// Toggling to done stamps done_at from the store's clock; toggling back clears it. The column
// CHECK means the two can never disagree.
func TestUpdateTaskTogglesDoneAt(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	task := createTask(t, s, "W5", "Re-run k6", "gateway")

	done, err := s.UpdateTask(ctx, task.ID, task.Version, TaskPatch{Status: ptr("done")})
	if err != nil {
		t.Fatalf("toggle done: %v", err)
	}

	if done.Status != "done" {
		t.Errorf("status = %q, want done", done.Status)
	}
	if done.DoneAt == nil {
		t.Fatal("done_at is nil after toggling done")
	}
	if *done.DoneAt != fixedNow.Format("2006-01-02T15:04:05Z") {
		t.Errorf("done_at = %q, want the clock's value", *done.DoneAt)
	}
	if done.Version != task.Version+1 {
		t.Errorf("version = %d, want %d", done.Version, task.Version+1)
	}

	todo, err := s.UpdateTask(ctx, done.ID, done.Version, TaskPatch{Status: ptr("todo")})
	if err != nil {
		t.Fatalf("toggle back: %v", err)
	}

	if todo.DoneAt != nil {
		t.Errorf("done_at = %v after untoggling, want nil", *todo.DoneAt)
	}
}

func TestUpdateTaskVersionGuard(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	task := createTask(t, s, "W1", "Golden set", "synapse")

	if _, err := s.UpdateTask(ctx, task.ID, task.Version, TaskPatch{Status: ptr("done")}); err != nil {
		t.Fatalf("first update: %v", err)
	}

	_, err := s.UpdateTask(ctx, task.ID, task.Version, TaskPatch{Status: ptr("todo")})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("stale update error = %v, want ErrConflict", err)
	}

	after, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if after.Status != "done" {
		t.Errorf("status = %q, want the stale write to have been rejected", after.Status)
	}
}

func TestUpdateTaskUnlinksGoal(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	g := createGoal(t, s, "OSS presence", "oss", "active")

	task, err := s.CreateTask(ctx, NewTask{
		Week: "W1", Title: "Pick a repo", Project: "oss", GoalID: &g.ID,
		SkillIDs: []int64{fixtureSkillID},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	var none *int64
	updated, err := s.UpdateTask(ctx, task.ID, task.Version, TaskPatch{GoalID: &none})
	if err != nil {
		t.Fatalf("unlink goal: %v", err)
	}

	if updated.GoalID != nil {
		t.Errorf("goal_id = %v, want nil", *updated.GoalID)
	}
}

func TestUpdateTaskMissing(t *testing.T) {
	_, err := newTestStore(t).UpdateTask(context.Background(), 999, 1, TaskPatch{Status: ptr("done")})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestListTasksFilters(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	createTask(t, s, "W1", "synapse work", "synapse")
	createTask(t, s, "W1", "gateway work", "gateway")
	createTask(t, s, "W5", "more gateway work", "gateway")

	week, err := s.ListTasks(ctx, TaskFilter{Week: "W1"})
	if err != nil {
		t.Fatalf("list by week: %v", err)
	}
	if len(week) != 2 {
		t.Errorf("W1 tasks = %d, want 2", len(week))
	}

	both, err := s.ListTasks(ctx, TaskFilter{Week: "W1", Projects: []string{"gateway"}})
	if err != nil {
		t.Fatalf("list by week and project: %v", err)
	}
	if len(both) != 1 {
		t.Errorf("W1 gateway tasks = %d, want 1", len(both))
	}

	multi, err := s.ListTasks(ctx, TaskFilter{Projects: []string{"synapse", "gateway"}})
	if err != nil {
		t.Fatalf("list by multiple projects: %v", err)
	}
	if len(multi) != 3 {
		t.Errorf("synapse+gateway tasks = %d, want 3", len(multi))
	}
}

func TestCountTasksByWeek(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	first := createTask(t, s, "W1", "one", "dash")
	createTask(t, s, "W1", "two", "dash")

	if _, err := s.UpdateTask(ctx, first.ID, first.Version, TaskPatch{Status: ptr("done")}); err != nil {
		t.Fatalf("toggle done: %v", err)
	}

	done, total, err := s.CountTasksByWeek(ctx, "W1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if done != 1 || total != 2 {
		t.Errorf("done/total = %d/%d, want 1/2", done, total)
	}

	// A week with no tasks must report zeroes rather than erroring.
	done, total, err = s.CountTasksByWeek(ctx, "W12")
	if err != nil {
		t.Fatalf("count empty week: %v", err)
	}
	if done != 0 || total != 0 {
		t.Errorf("empty week done/total = %d/%d, want 0/0", done, total)
	}
}

func TestDeleteTask(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	task := createTask(t, s, "W1", "Ad-hoc", "dash")

	if err := s.DeleteTask(ctx, task.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	if _, err := s.GetTask(ctx, task.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete error = %v, want ErrNotFound", err)
	}

	if err := s.DeleteTask(ctx, task.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete error = %v, want ErrNotFound", err)
	}
}

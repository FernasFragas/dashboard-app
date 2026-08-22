package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const taskColumns = `id, week, title, project, goal_id, steps, done_means, status, done_at,
	seed_key, sort_order, version, created_at`

func scanTask(row interface{ Scan(...any) error }) (Task, error) {
	var (
		t     Task
		steps string
	)

	err := row.Scan(&t.ID, &t.Week, &t.Title, &t.Project, &t.GoalID, &steps, &t.DoneMeans,
		&t.Status, &t.DoneAt, &t.SeedKey, &t.SortOrder, &t.Version, &t.CreatedAt)
	if err != nil {
		return Task{}, err
	}

	if err := json.Unmarshal([]byte(steps), &t.Steps); err != nil {
		return Task{}, fmt.Errorf("decode steps for task %d: %w", t.ID, err)
	}

	if t.Steps == nil {
		t.Steps = []string{}
	}

	return t, nil
}

// ListTasks returns the week checklist, optionally narrowed by week and project.
func (s *Store) ListTasks(ctx context.Context, f TaskFilter) ([]Task, error) {
	query := `SELECT ` + taskColumns + ` FROM tasks`

	var (
		where []string
		args  []any
	)

	if f.Week != "" {
		where = append(where, "week = ?")
		args = append(args, f.Week)
	}

	if len(f.Projects) > 0 {
		where = append(where, "project IN ("+placeholders(len(f.Projects))+")")
		for _, p := range f.Projects {
			args = append(args, p)
		}
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	query += " ORDER BY week, sort_order, id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, classify("list tasks", err)
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, classify("scan task", err)
		}
		out = append(out, t)
	}

	return out, classify("list tasks", rows.Err())
}

// GetTask returns one task by id.
func (s *Store) GetTask(ctx context.Context, id int64) (Task, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)

	t, err := scanTask(row)
	if err != nil {
		return Task{}, classify(fmt.Sprintf("get task %d", id), err)
	}

	return t, nil
}

// CountTasksByWeek reports done and total for one plan week, for the completion bar.
func (s *Store) CountTasksByWeek(ctx context.Context, week string) (done, total int, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT coalesce(sum(status = 'done'), 0), count(*) FROM tasks WHERE week = ?`, week,
	).Scan(&done, &total)
	if err != nil {
		return 0, 0, classify("count tasks by week", err)
	}

	return done, total, nil
}

// CreateTask inserts an ad-hoc task. seed_key is always NULL for API-created tasks, which is
// what makes them invisible to the seed loader (docs/DATABASE.md section 8).
func (s *Store) CreateTask(ctx context.Context, in NewTask) (Task, error) {
	steps := in.Steps
	if steps == nil {
		steps = []string{}
	}

	encoded, err := json.Marshal(steps)
	if err != nil {
		return Task{}, fmt.Errorf("encode steps: %w", err)
	}

	var created Task
	err = s.tx(ctx, func(tx execer) error {
		var maxOrder int
		if err := tx.QueryRowContext(ctx,
			`SELECT coalesce(max(sort_order), 0) FROM tasks WHERE week = ?`, in.Week,
		).Scan(&maxOrder); err != nil {
			return classify("read max task order", err)
		}

		res, err := tx.ExecContext(ctx, `
			INSERT INTO tasks (week, title, project, goal_id, steps, done_means, status,
				done_at, seed_key, sort_order, version, created_at)
			VALUES (?, ?, ?, ?, ?, ?, 'todo', NULL, NULL, ?, 1, ?)`,
			in.Week, in.Title, in.Project, in.GoalID, string(encoded), in.DoneMeans,
			maxOrder+sortStep, s.utcNow(),
		)
		if err != nil {
			return classify("insert task", err)
		}

		id, err := res.LastInsertId()
		if err != nil {
			return classify("insert task id", err)
		}

		row := tx.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
		created, err = scanTask(row)

		return classify("read created task", err)
	})
	if err != nil {
		return Task{}, err
	}

	return created, nil
}

// UpdateTask applies patch to the task, guarded by the expected version. done_at is stamped by
// the store and never accepted from the caller.
func (s *Store) UpdateTask(ctx context.Context, id int64, version int, patch TaskPatch) (Task, error) {
	var updated Task

	err := s.tx(ctx, func(tx execer) error {
		row := tx.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)

		before, err := scanTask(row)
		if err != nil {
			return classify(fmt.Sprintf("get task %d", id), err)
		}

		if before.Version != version {
			return fmt.Errorf("update task %d: version %d is stale: %w", id, version, ErrConflict)
		}

		sets := []string{"version = version + 1"}
		args := []any{}

		if patch.Title != nil {
			sets = append(sets, "title = ?")
			args = append(args, *patch.Title)
		}
		if patch.Project != nil {
			sets = append(sets, "project = ?")
			args = append(args, *patch.Project)
		}
		if patch.DoneMeans != nil {
			sets = append(sets, "done_means = ?")
			args = append(args, *patch.DoneMeans)
		}
		if patch.GoalID != nil {
			sets = append(sets, "goal_id = ?")
			args = append(args, *patch.GoalID)
		}

		if patch.Status != nil {
			sets = append(sets, "status = ?")
			args = append(args, *patch.Status)

			switch {
			case *patch.Status == "done" && before.Status != "done":
				sets = append(sets, "done_at = ?")
				args = append(args, s.utcNow())
			case *patch.Status != "done" && before.Status == "done":
				sets = append(sets, "done_at = NULL")
			}
		}

		args = append(args, id)

		if _, err := tx.ExecContext(ctx,
			"UPDATE tasks SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...,
		); err != nil {
			return classify(fmt.Sprintf("update task %d", id), err)
		}

		row = tx.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
		updated, err = scanTask(row)

		return classify("read updated task", err)
	})
	if err != nil {
		return Task{}, err
	}

	return updated, nil
}

// DeleteTask removes a task.
//
// Deleting a seeded task is permitted, but the next boot re-inserts it: its seed_key no longer
// matches an existing row (docs/DATABASE.md section 8).
func (s *Store) DeleteTask(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return classify(fmt.Sprintf("delete task %d", id), err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return classify("delete task rows", err)
	}

	if affected == 0 {
		return fmt.Errorf("delete task %d: %w", id, ErrNotFound)
	}

	return nil
}

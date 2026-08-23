package store

import (
	"context"
	"fmt"
	"strings"
)

// sortStep is the gap between adjacent sort_order values. Sparse integers let a card land
// between two others without float drift; a move renumbers the whole target column, so the
// gap only ever has to survive one transaction (docs/API.md, PATCH /api/goals/{id}).
const sortStep = 100

const goalColumns = `g.id, g.code, g.title, g.done_means, g.project, g.phase, g.status,
	g.target, g.sort_order, g.version, g.created_at, g.completed_at`

func scanGoal(row interface{ Scan(...any) error }, g *Goal, withCount bool) error {
	dest := []any{
		&g.ID, &g.Code, &g.Title, &g.DoneMeans, &g.Project, &g.Phase, &g.Status,
		&g.Target, &g.SortOrder, &g.Version, &g.CreatedAt, &g.CompletedAt,
	}
	if withCount {
		dest = append(dest, &g.LogCount)
	}

	return row.Scan(dest...)
}

// ListGoals returns goals ordered for the Kanban board, each with its linked-log count.
func (s *Store) ListGoals(ctx context.Context, f GoalFilter) ([]Goal, error) {
	query := `
		SELECT ` + goalColumns + `,
			(SELECT count(*) FROM log_entries le WHERE le.goal_id = g.id) AS log_count
		FROM goals g`

	var (
		where []string
		args  []any
	)

	if f.Status != "" {
		where = append(where, "g.status = ?")
		args = append(args, f.Status)
	}

	if len(f.Projects) > 0 {
		where = append(where, "g.project IN ("+placeholders(len(f.Projects))+")")
		for _, p := range f.Projects {
			args = append(args, p)
		}
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	query += " ORDER BY g.status, g.sort_order, g.id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, classify("list goals", err)
	}
	defer rows.Close()

	var out []Goal
	for rows.Next() {
		var g Goal
		if err := scanGoal(rows, &g, true); err != nil {
			return nil, classify("scan goal", err)
		}
		out = append(out, g)
	}

	return out, classify("list goals", rows.Err())
}

// GetGoal returns one goal with its linked-log count.
func (s *Store) GetGoal(ctx context.Context, id int64) (Goal, error) {
	var g Goal
	row := s.db.QueryRowContext(ctx, `
		SELECT `+goalColumns+`,
			(SELECT count(*) FROM log_entries le WHERE le.goal_id = g.id) AS log_count
		FROM goals g WHERE g.id = ?`, id)

	if err := scanGoal(row, &g, true); err != nil {
		return Goal{}, classify(fmt.Sprintf("get goal %d", id), err)
	}

	return g, nil
}

// CountGoalsByStatus reports how many goals sit in one column. It backs the Active > 3
// warning badge, which the API reports but never enforces (M5).
func (s *Store) CountGoalsByStatus(ctx context.Context, status string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM goals WHERE status = ?`, status,
	).Scan(&n)
	if err != nil {
		return 0, classify("count goals by status", err)
	}

	return n, nil
}

// CreateGoal inserts a hand-created goal. Code is always NULL: G-numbers belong to the seed.
func (s *Store) CreateGoal(ctx context.Context, in NewGoal) (Goal, error) {
	status := in.Status
	if status == "" {
		status = "backlog"
	}

	var created Goal
	err := s.tx(ctx, func(tx execer) error {
		next, err := nextSortOrder(ctx, tx, status)
		if err != nil {
			return err
		}

		res, err := tx.ExecContext(ctx, `
			INSERT INTO goals (code, title, done_means, project, phase, status, target,
				sort_order, version, created_at, completed_at)
			VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, 1, ?, NULL)`,
			in.Title, in.DoneMeans, in.Project, in.Phase, status, in.Target, next, s.utcNow(),
		)
		if err != nil {
			return classify("insert goal", err)
		}

		id, err := res.LastInsertId()
		if err != nil {
			return classify("insert goal id", err)
		}

		created, err = getGoalTx(ctx, tx, id)

		return err
	})
	if err != nil {
		return Goal{}, err
	}

	return created, nil
}

// UpdateGoal applies patch to the goal, guarded by the expected version.
//
// It returns the updated goal plus every card whose sort_order the move changed - both the
// target column and, on a cross-column move, the column the card left - so the client can
// apply one payload without a follow-up read. A stale version yields ErrConflict; the API maps
// that to 412 (docs/API.md section 1).
//
// Only the moved card's version is bumped. Renumbering is not an edit to the cards it shifts.
func (s *Store) UpdateGoal(ctx context.Context, id int64, version int, patch GoalPatch) (Goal, []Goal, error) {
	var (
		updated   Goal
		reordered []Goal
	)

	err := s.tx(ctx, func(tx execer) error {
		before, err := getGoalTx(ctx, tx, id)
		if err != nil {
			return err
		}

		if before.Version != version {
			return fmt.Errorf("update goal %d: version %d is stale: %w", id, version, ErrConflict)
		}

		sets := []string{"version = version + 1"}
		args := []any{}

		if patch.Title != nil {
			sets = append(sets, "title = ?")
			args = append(args, *patch.Title)
		}
		if patch.DoneMeans != nil {
			sets = append(sets, "done_means = ?")
			args = append(args, *patch.DoneMeans)
		}
		if patch.Project != nil {
			sets = append(sets, "project = ?")
			args = append(args, *patch.Project)
		}
		if patch.Target != nil {
			sets = append(sets, "target = ?")
			args = append(args, *patch.Target)
		}

		targetStatus := before.Status
		if patch.Status != nil {
			targetStatus = *patch.Status

			sets = append(sets, "status = ?")
			args = append(args, targetStatus)

			// Moving into done stamps completed_at; moving out clears it. The column CHECK
			// enforces that the two can never disagree.
			switch {
			case targetStatus == "done" && before.Status != "done":
				sets = append(sets, "completed_at = ?")
				args = append(args, s.utcNow())
			case targetStatus != "done" && before.Status == "done":
				sets = append(sets, "completed_at = NULL")
			}
		}

		args = append(args, id)

		if _, err := tx.ExecContext(ctx,
			"UPDATE goals SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...,
		); err != nil {
			return classify(fmt.Sprintf("update goal %d", id), err)
		}

		// A position was given, so the target column is renumbered as a whole.
		if patch.Position != nil {
			reordered, err = s.renumberColumn(ctx, tx, targetStatus, id, *patch.Position)
			if err != nil {
				return err
			}
		}

		// The card left a column: close the gap it left behind. Those cards ship back with the
		// response too, so the client can apply one payload and be done - without them it has
		// to refetch the whole board after every move (docs/API.md, PATCH /api/goals/{id}).
		if patch.Status != nil && targetStatus != before.Status {
			source, err := s.renumberColumn(ctx, tx, before.Status, 0, -1)
			if err != nil {
				return err
			}
			reordered = append(reordered, source...)
		}

		updated, err = getGoalTx(ctx, tx, id)

		return err
	})
	if err != nil {
		return Goal{}, nil, err
	}

	return updated, reordered, nil
}

// DeleteGoal removes a goal. Tasks and log entries that referenced it survive with goal_id
// nulled by the ON DELETE SET NULL rules - history is never deleted as a side effect.
func (s *Store) DeleteGoal(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM goals WHERE id = ?`, id)
	if err != nil {
		return classify(fmt.Sprintf("delete goal %d", id), err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return classify("delete goal rows", err)
	}

	if affected == 0 {
		return fmt.Errorf("delete goal %d: %w", id, ErrNotFound)
	}

	return nil
}

// renumberColumn rewrites sort_order for one status column in sparse steps, placing moved (if
// non-zero) at the given zero-based position. Pass position < 0 to renumber in place.
//
// It returns the column's cards in their new order.
func (s *Store) renumberColumn(ctx context.Context, tx execer, status string, moved int64, position int) ([]Goal, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM goals WHERE status = ? ORDER BY sort_order, id`, status)
	if err != nil {
		return nil, classify("read column order", err)
	}

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, classify("scan column order", err)
		}
		ids = append(ids, id)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, classify("read column order", err)
	}

	if moved != 0 && position >= 0 {
		ids = moveTo(ids, moved, position)
	}

	// sort_order only, deliberately: version guards user edits, and being renumbered because a
	// neighbour moved is not an edit to this card. Bumping here would double-bump the moved
	// card (the caller's UPDATE already did it) and invalidate the ETag of every other card in
	// the column, so an unrelated goal would 412 on its next write.
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE goals SET sort_order = ? WHERE id = ?`,
			(i+1)*sortStep, id,
		); err != nil {
			return nil, classify("renumber column", err)
		}
	}

	out := make([]Goal, 0, len(ids))
	for _, id := range ids {
		g, err := getGoalTx(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}

	return out, nil
}

// moveTo relocates id within ids to the given index, clamped to the slice bounds.
func moveTo(ids []int64, id int64, position int) []int64 {
	out := make([]int64, 0, len(ids))
	for _, existing := range ids {
		if existing != id {
			out = append(out, existing)
		}
	}

	if position < 0 {
		position = 0
	}
	if position > len(out) {
		position = len(out)
	}

	out = append(out, 0)
	copy(out[position+1:], out[position:])
	out[position] = id

	return out
}

func getGoalTx(ctx context.Context, tx execer, id int64) (Goal, error) {
	var g Goal
	row := tx.QueryRowContext(ctx, `
		SELECT `+goalColumns+`,
			(SELECT count(*) FROM log_entries le WHERE le.goal_id = g.id) AS log_count
		FROM goals g WHERE g.id = ?`, id)

	if err := scanGoal(row, &g, true); err != nil {
		return Goal{}, classify(fmt.Sprintf("get goal %d", id), err)
	}

	return g, nil
}

func nextSortOrder(ctx context.Context, tx execer, status string) (int, error) {
	var maxOrder int
	err := tx.QueryRowContext(ctx,
		`SELECT coalesce(max(sort_order), 0) FROM goals WHERE status = ?`, status,
	).Scan(&maxOrder)
	if err != nil {
		return 0, classify("read max sort order", err)
	}

	return maxOrder + sortStep, nil
}

// placeholders builds "?, ?, ?" for an IN clause of n values.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

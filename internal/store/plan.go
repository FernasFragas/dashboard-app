package store

import (
	"context"
	"database/sql"
	"fmt"
)

// CurrentPlan returns the loaded plan identity, or nil before any seed has been applied.
func (s *Store) CurrentPlan(ctx context.Context) (*PlanMeta, error) {
	var meta PlanMeta
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, loaded_at FROM plan_meta ORDER BY loaded_at DESC LIMIT 1`,
	).Scan(&meta.ID, &meta.Name, &meta.LoadedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, classify("current plan", err)
	}

	return &meta, nil
}

// LatestPlanSource returns the newest stored markdown source for a plan, or nil if none exists.
func (s *Store) LatestPlanSource(ctx context.Context, planID string) (*PlanSource, error) {
	var source PlanSource
	err := s.db.QueryRowContext(ctx, `
		SELECT id, plan_id, sha256, source, loaded_at
		FROM plan_sources
		WHERE plan_id = ?
		ORDER BY loaded_at DESC, id DESC
		LIMIT 1`, planID,
	).Scan(&source.ID, &source.PlanID, &source.SHA256, &source.Source, &source.LoadedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, classify("latest plan source", err)
	}

	return &source, nil
}

// PlanCounts returns a compact summary of current plan-sized tables.
func (s *Store) PlanCounts(ctx context.Context) (PlanCounts, error) {
	var counts PlanCounts
	for _, item := range []struct {
		name string
		dst  *int
	}{
		{"weeks", &counts.Weeks},
		{"goals", &counts.Goals},
		{"tasks", &counts.Tasks},
		{"skills", &counts.Skills},
		{"projects", &counts.Projects},
	} {
		if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+item.name).Scan(item.dst); err != nil {
			return PlanCounts{}, classify("count "+item.name, err)
		}
	}

	return counts, nil
}

// SavePlanSource records one uploaded markdown plan source and keeps only the newest few per plan.
func (s *Store) SavePlanSource(ctx context.Context, planID, sha256, source string) (PlanSource, error) {
	loadedAt := s.utcNow()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO plan_sources (plan_id, sha256, source, loaded_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (plan_id, sha256) DO UPDATE SET
			source = excluded.source,
			loaded_at = excluded.loaded_at`,
		planID, sha256, source, loadedAt,
	); err != nil {
		return PlanSource{}, classify("save plan source", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM plan_sources
		WHERE plan_id = ?
		  AND id NOT IN (
			SELECT id FROM plan_sources
			WHERE plan_id = ?
			ORDER BY loaded_at DESC, id DESC
			LIMIT 5
		  )`, planID, planID,
	); err != nil {
		return PlanSource{}, classify("prune plan sources", err)
	}

	var out PlanSource
	err := s.db.QueryRowContext(ctx, `
		SELECT id, plan_id, sha256, source, loaded_at
		FROM plan_sources
		WHERE plan_id = ? AND sha256 = ?`, planID, sha256,
	).Scan(&out.ID, &out.PlanID, &out.SHA256, &out.Source, &out.LoadedAt)
	if err != nil {
		return PlanSource{}, classify("read saved plan source", err)
	}

	return out, nil
}

// DiffPlan compares parsed plan fingerprints to current rows without mutating anything.
func (s *Store) DiffPlan(ctx context.Context, next PlanFingerprints) (PlanDiff, error) {
	weeks, err := s.weekFingerprints(ctx)
	if err != nil {
		return PlanDiff{}, err
	}
	tasks, err := s.seedTaskKeys(ctx)
	if err != nil {
		return PlanDiff{}, err
	}
	goals, err := s.goalFingerprints(ctx)
	if err != nil {
		return PlanDiff{}, err
	}

	return PlanDiff{
		Weeks: diffFingerprints(weeks, next.Weeks),
		Tasks: diffTaskKeys(tasks, next.Tasks),
		Goals: diffFingerprints(goals, next.Goals),
	}, nil
}

// PlanAtRiskForReplace reports user-visible state that replace mode will preserve but unlink.
func (s *Store) PlanAtRiskForReplace(ctx context.Context, newCategoryIDs []string) (PlanAtRisk, error) {
	counts := []struct {
		query string
		dst   *int
	}{
		{`SELECT count(*) FROM tasks WHERE status = 'done'`, nil},
		{`SELECT count(*) FROM goals WHERE status <> 'backlog'`, nil},
		{`SELECT count(*) FROM checkpoints WHERE completed_at IS NOT NULL OR answers IS NOT NULL`, nil},
		{`SELECT count(*) FROM log_entries WHERE goal_id IS NOT NULL`, nil},
		{`SELECT count(*) FROM log_skills`, nil},
	}

	var out PlanAtRisk
	counts[0].dst = &out.TaskCompletions
	counts[1].dst = &out.GoalPositions
	counts[2].dst = &out.CheckpointAnswers
	counts[3].dst = &out.LogGoalLinks
	counts[4].dst = &out.LogSkillLinks

	for _, count := range counts {
		if err := s.db.QueryRowContext(ctx, count.query).Scan(count.dst); err != nil {
			return PlanAtRisk{}, classify("count plan replace risk", err)
		}
	}

	retired, err := s.categoriesRetiredByReplace(ctx, newCategoryIDs)
	if err != nil {
		return PlanAtRisk{}, err
	}
	out.RetiredCategories = retired

	return out, nil
}

func (s *Store) weekFingerprints(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT code, phase, start_date, end_date, coalesce(focus, ''), sort_order FROM weeks`)
	if err != nil {
		return nil, classify("diff weeks", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var code, phase, start, end, focus string
		var sortOrder int
		if err := rows.Scan(&code, &phase, &start, &end, &focus, &sortOrder); err != nil {
			return nil, classify("scan diff weeks", err)
		}
		out[code] = fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", phase, start, end, focus, sortOrder)
	}

	return out, classify("diff weeks", rows.Err())
}

func (s *Store) seedTaskKeys(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT seed_key, title FROM tasks WHERE seed_key IS NOT NULL`)
	if err != nil {
		return nil, classify("diff tasks", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var key, title string
		if err := rows.Scan(&key, &title); err != nil {
			return nil, classify("scan diff tasks", err)
		}
		out[key] = title
	}

	return out, classify("diff tasks", rows.Err())
}

func (s *Store) goalFingerprints(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT code, title, coalesce(done_means, ''), project, coalesce(phase, ''),
			coalesce(target, ''), sort_order
		FROM goals
		WHERE code IS NOT NULL`)
	if err != nil {
		return nil, classify("diff goals", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var code, title, doneMeans, project, phase, target string
		var sortOrder int
		if err := rows.Scan(&code, &title, &doneMeans, &project, &phase, &target, &sortOrder); err != nil {
			return nil, classify("scan diff goals", err)
		}
		out[code] = fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%d",
			title, doneMeans, project, phase, target, sortOrder)
	}

	return out, classify("diff goals", rows.Err())
}

func (s *Store) categoriesRetiredByReplace(ctx context.Context, categoryIDs []string) (int, error) {
	newCategories := map[string]struct{}{}
	for _, id := range categoryIDs {
		newCategories[id] = struct{}{}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, count(le.id)
		FROM categories c
		JOIN log_entries le ON le.category_id = c.id
		GROUP BY c.id`)
	if err != nil {
		return 0, classify("count replace category risk", err)
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var id string
		var refs int
		if err := rows.Scan(&id, &refs); err != nil {
			return 0, classify("scan replace category risk", err)
		}
		if refs > 0 {
			if _, keep := newCategories[id]; !keep {
				total++
			}
		}
	}

	return total, classify("count replace category risk", rows.Err())
}

func diffFingerprints(current, next map[string]string) PlanChangeCounts {
	var out PlanChangeCounts
	for key, nextValue := range next {
		currentValue, ok := current[key]
		switch {
		case !ok:
			out.Added++
		case currentValue == nextValue:
			out.Unchanged++
		default:
			out.Updated++
		}
	}
	for key := range current {
		if _, ok := next[key]; !ok {
			out.Removed++
		}
	}
	return out
}

func diffTaskKeys(current, next map[string]string) PlanChangeCounts {
	var out PlanChangeCounts
	for key := range next {
		if _, ok := current[key]; ok {
			out.Unchanged++
		} else {
			out.Added++
		}
	}
	for key := range current {
		if _, ok := next[key]; !ok {
			out.Orphaned++
		}
	}
	return out
}

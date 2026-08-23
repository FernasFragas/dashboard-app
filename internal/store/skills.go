package store

import (
	"context"
	"fmt"
	"strings"
)

// Skill progress is derived here, never stored (ADR-007). Every statistic is one grouped query
// across all skills — never a query per skill, which is the failure mode the ADR calls out.

const skillColumns = `id, code, name, description, associate_when, target_tier, sort_order, created_at`

func scanSkill(row interface{ Scan(...any) error }) (Skill, error) {
	var s Skill
	err := row.Scan(&s.ID, &s.Code, &s.Name, &s.Description, &s.AssociateWhen,
		&s.TargetTier, &s.SortOrder, &s.CreatedAt)

	return s, err
}

// ListSkills returns every skill with its derived stats: how many linked tasks are done out of
// how many exist, how many log entries cite it, and when it was last touched.
func (s *Store) ListSkills(ctx context.Context) ([]Skill, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+skillColumns+` FROM skills ORDER BY sort_order, id`)
	if err != nil {
		return nil, classify("list skills", err)
	}
	defer rows.Close()

	var (
		skills []Skill
		index  = map[int64]int{}
	)

	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, classify("scan skill", err)
		}
		index[skill.ID] = len(skills)
		skills = append(skills, skill)
	}

	if err := rows.Err(); err != nil {
		return nil, classify("list skills", err)
	}

	if err := s.attachTaskStats(ctx, skills, index); err != nil {
		return nil, err
	}

	if err := s.attachEvidenceStats(ctx, skills, index); err != nil {
		return nil, err
	}

	return skills, nil
}

// attachTaskStats fills TaskCount, TaskDone and part of LastActivity in one grouped query.
func (s *Store) attachTaskStats(ctx context.Context, skills []Skill, index map[int64]int) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts.skill_id,
			count(*),
			coalesce(sum(t.status = 'done'), 0),
			max(t.done_at)
		FROM task_skills ts
		JOIN tasks t ON t.id = ts.task_id
		GROUP BY ts.skill_id`)
	if err != nil {
		return classify("aggregate task skills", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			skillID int64
			total   int
			done    int
			lastAt  *string
		)

		if err := rows.Scan(&skillID, &total, &done, &lastAt); err != nil {
			return classify("scan task skill stats", err)
		}

		if i, ok := index[skillID]; ok {
			skills[i].TaskCount = total
			skills[i].TaskDone = done
			skills[i].LastActivity = laterOf(skills[i].LastActivity, lastAt)
		}
	}

	return classify("aggregate task skills", rows.Err())
}

// attachEvidenceStats fills EvidenceCount and the log side of LastActivity in one query.
func (s *Store) attachEvidenceStats(ctx context.Context, skills []Skill, index map[int64]int) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ls.skill_id, count(*), max(le.occurred_at)
		FROM log_skills ls
		JOIN log_entries le ON le.id = ls.log_id
		GROUP BY ls.skill_id`)
	if err != nil {
		return classify("aggregate log skills", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			skillID int64
			count   int
			lastAt  *string
		)

		if err := rows.Scan(&skillID, &count, &lastAt); err != nil {
			return classify("scan log skill stats", err)
		}

		if i, ok := index[skillID]; ok {
			skills[i].EvidenceCount = count
			skills[i].LastActivity = laterOf(skills[i].LastActivity, lastAt)
		}
	}

	return classify("aggregate log skills", rows.Err())
}

// GetSkillDetail returns one skill with the tasks that build it and the log entries citing it.
func (s *Store) GetSkillDetail(ctx context.Context, id int64) (SkillDetail, error) {
	all, err := s.ListSkills(ctx)
	if err != nil {
		return SkillDetail{}, err
	}

	var detail SkillDetail
	found := false

	for _, skill := range all {
		if skill.ID == id {
			detail.Skill = skill
			found = true
			break
		}
	}

	if !found {
		return SkillDetail{}, fmt.Errorf("get skill %d: %w", id, ErrNotFound)
	}

	taskRows, err := s.db.QueryContext(ctx, `
		SELECT `+taskColumns+`
		FROM tasks
		JOIN task_skills ts ON ts.task_id = tasks.id
		WHERE ts.skill_id = ?
		ORDER BY week, sort_order, id`, id)
	if err != nil {
		return SkillDetail{}, classify("list skill tasks", err)
	}
	defer taskRows.Close()

	for taskRows.Next() {
		task, err := scanTask(taskRows)
		if err != nil {
			return SkillDetail{}, classify("scan skill task", err)
		}
		detail.Tasks = append(detail.Tasks, task)
	}

	if err := taskRows.Err(); err != nil {
		return SkillDetail{}, classify("list skill tasks", err)
	}

	logRows, err := s.db.QueryContext(ctx, `
		SELECT le.id, le.category_id, le.title, le.note, le.url, le.goal_id,
			le.occurred_at, le.created_at, c.label, c.icon, g.code
		FROM log_entries le
		JOIN log_skills ls ON ls.log_id = le.id
		JOIN categories c ON c.id = le.category_id
		LEFT JOIN goals g ON g.id = le.goal_id
		WHERE ls.skill_id = ?
		ORDER BY le.occurred_at DESC, le.id DESC`, id)
	if err != nil {
		return SkillDetail{}, classify("list skill evidence", err)
	}
	defer logRows.Close()

	for logRows.Next() {
		var e LogEntry
		if err := logRows.Scan(&e.ID, &e.CategoryID, &e.Title, &e.Note, &e.URL, &e.GoalID,
			&e.OccurredAt, &e.CreatedAt, &e.CategoryLabel, &e.Icon, &e.GoalCode); err != nil {
			return SkillDetail{}, classify("scan skill evidence", err)
		}
		detail.Evidence = append(detail.Evidence, e)
	}

	return detail, classify("list skill evidence", logRows.Err())
}

// SkillIDsByCode resolves seed codes to ids. Used by the seed loader.
func (s *Store) SkillIDsByCode(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, id FROM skills`)
	if err != nil {
		return nil, classify("map skill codes", err)
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var (
			code string
			id   int64
		)
		if err := rows.Scan(&code, &id); err != nil {
			return nil, classify("scan skill code", err)
		}
		out[code] = id
	}

	return out, classify("map skill codes", rows.Err())
}

// TaskSkillIDs returns the skills linked to each of the given tasks, so a list of tasks can be
// decorated in one query rather than one per row.
func (s *Store) TaskSkillIDs(ctx context.Context, taskIDs []int64) (map[int64][]int64, error) {
	out := map[int64][]int64{}
	if len(taskIDs) == 0 {
		return out, nil
	}

	args := make([]any, 0, len(taskIDs))
	for _, id := range taskIDs {
		args = append(args, id)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ts.task_id, ts.skill_id
		FROM task_skills ts
		JOIN skills sk ON sk.id = ts.skill_id
		WHERE ts.task_id IN (`+placeholders(len(taskIDs))+`)
		ORDER BY sk.sort_order, sk.id`, args...)
	if err != nil {
		return nil, classify("list task skills", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskID, skillID int64
		if err := rows.Scan(&taskID, &skillID); err != nil {
			return nil, classify("scan task skill", err)
		}
		out[taskID] = append(out[taskID], skillID)
	}

	return out, classify("list task skills", rows.Err())
}

// SetTaskSkills replaces a task's skill links wholesale.
//
// Rejects an empty list: the invariant is that every task builds at least one skill, and this
// is the store's share of holding it (docs/DATABASE.md §4.7).
func (s *Store) SetTaskSkills(ctx context.Context, taskID int64, skillIDs []int64) error {
	if len(skillIDs) == 0 {
		return fmt.Errorf("set skills for task %d: at least one skill is required: %w",
			taskID, ErrConstraint)
	}

	return s.tx(ctx, func(tx execer) error {
		return replaceLinks(ctx, tx, "task_skills", "task_id", taskID, skillIDs)
	})
}

// SetLogSkills replaces a log entry's skill links. An empty list is allowed - evidence is
// optional, unlike a task's skills.
func (s *Store) SetLogSkills(ctx context.Context, logID int64, skillIDs []int64) error {
	return s.tx(ctx, func(tx execer) error {
		return replaceLinks(ctx, tx, "log_skills", "log_id", logID, skillIDs)
	})
}

// LogSkillIDs returns the skills cited by each of the given log entries.
func (s *Store) LogSkillIDs(ctx context.Context, logIDs []int64) (map[int64][]int64, error) {
	out := map[int64][]int64{}
	if len(logIDs) == 0 {
		return out, nil
	}

	args := make([]any, 0, len(logIDs))
	for _, id := range logIDs {
		args = append(args, id)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ls.log_id, ls.skill_id
		FROM log_skills ls
		JOIN skills sk ON sk.id = ls.skill_id
		WHERE ls.log_id IN (`+placeholders(len(logIDs))+`)
		ORDER BY sk.sort_order, sk.id`, args...)
	if err != nil {
		return nil, classify("list log skills", err)
	}
	defer rows.Close()

	for rows.Next() {
		var logID, skillID int64
		if err := rows.Scan(&logID, &skillID); err != nil {
			return nil, classify("scan log skill", err)
		}
		out[logID] = append(out[logID], skillID)
	}

	return out, classify("list log skills", rows.Err())
}

// UnlinkedTaskIDs lists tasks with no skill at all. These are rows that predate M8; the UI
// shows them a "needs skill" badge rather than pretending the invariant already holds.
func (s *Store) UnlinkedTaskIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id FROM tasks t
		LEFT JOIN task_skills ts ON ts.task_id = t.id
		WHERE ts.task_id IS NULL
		ORDER BY t.id`)
	if err != nil {
		return nil, classify("list unlinked tasks", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, classify("scan unlinked task", err)
		}
		out = append(out, id)
	}

	return out, classify("list unlinked tasks", rows.Err())
}

func replaceLinks(
	ctx context.Context, tx execer, table, ownerColumn string, ownerID int64, skillIDs []int64,
) error {
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM "+table+" WHERE "+ownerColumn+" = ?", ownerID,
	); err != nil {
		return classify("clear "+table, err)
	}

	seen := map[int64]bool{}
	for _, skillID := range skillIDs {
		if seen[skillID] {
			continue
		}
		seen[skillID] = true

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO "+table+" ("+ownerColumn+", skill_id) VALUES (?, ?)", ownerID, skillID,
		); err != nil {
			return classify("link "+table, err)
		}
	}

	return nil
}

// laterOf returns whichever RFC3339 timestamp is later, ignoring nils. Both stats feed one
// LastActivity, so the skill's freshness is the newer of "a task finished" and "a log cited it".
func laterOf(current, candidate *string) *string {
	switch {
	case candidate == nil:
		return current
	case current == nil:
		return candidate
	case strings.Compare(*candidate, *current) > 0:
		return candidate
	default:
		return current
	}
}

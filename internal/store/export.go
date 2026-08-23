package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExportData is the complete JSON backup document returned by GET /api/export.
type ExportData struct {
	ExportedAt         string                    `json:"exported_at"`
	SchemaVersion      int                       `json:"schema_version"`
	PlanMeta           []ExportPlanMeta          `json:"plan_meta"`
	PlanConfig         []ExportConfig            `json:"plan_config"`
	PlanSources        []PlanSource              `json:"plan_sources"`
	Projects           []Project                 `json:"projects"`
	Phases             []Phase                   `json:"phases"`
	SkillTiers         []SkillTier               `json:"skill_tiers"`
	Weeks              []Week                    `json:"weeks"`
	Rhythm             []Rhythm                  `json:"rhythm"`
	Categories         []Category                `json:"categories"`
	MetricDefs         []MetricDef               `json:"metric_defs"`
	Goals              []Goal                    `json:"goals"`
	Tasks              []ExportTask              `json:"tasks"`
	LogEntries         []ExportLogEntry          `json:"log_entries"`
	DailyReviews       []DailyReview             `json:"daily_reviews"`
	Metrics            []Metric                  `json:"metrics"`
	Checkpoints        []ExportCheckpoint        `json:"checkpoints"`
	Skills             []Skill                   `json:"skills"`
	TaskSkills         []ExportSkillLink         `json:"task_skills"`
	LogSkills          []ExportSkillLink         `json:"log_skills"`
	XPRules            []ExportXPRule            `json:"xp_rules"`
	XPEvents           []ExportXPEvent           `json:"xp_events"`
	XPEventSkills      []ExportXPEventSkill      `json:"xp_event_skills"`
	Achievements       []ExportAchievement       `json:"achievements"`
	AchievementUnlocks []ExportAchievementUnlock `json:"achievement_unlocks"`
}

// ExportPlanMeta is one row from plan_meta.
type ExportPlanMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LoadedAt string `json:"loaded_at"`
}

// ExportConfig is one row from plan_config.
type ExportConfig struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ExportSkillLink is one row from either task_skills or log_skills.
type ExportSkillLink struct {
	TaskID  *int64 `json:"task_id,omitempty"`
	LogID   *int64 `json:"log_id,omitempty"`
	SkillID int64  `json:"skill_id"`
}

// ExportXPRule is one tunable XP rule row.
type ExportXPRule struct {
	Source string `json:"source"`
	Amount int    `json:"amount"`
}

// ExportXPEvent is one raw ledger row, without derived labels or skill names.
type ExportXPEvent struct {
	ID         int64  `json:"id"`
	SourceType string `json:"source_type"`
	SourceID   int64  `json:"source_id"`
	Amount     int    `json:"amount"`
	CreatedAt  string `json:"created_at"`
}

// ExportXPEventSkill is one event-to-skill mirror row.
type ExportXPEventSkill struct {
	EventID int64 `json:"event_id"`
	SkillID int64 `json:"skill_id"`
}

// ExportAchievement is one seeded badge row.
type ExportAchievement struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

// ExportAchievementUnlock is one historical badge unlock.
type ExportAchievementUnlock struct {
	AchievementID int64  `json:"achievement_id"`
	UnlockedAt    string `json:"unlocked_at"`
}

// ExportTask includes seed_key, which the screen-facing Task type deliberately hides.
type ExportTask struct {
	ID        int64    `json:"id"`
	Week      string   `json:"week"`
	Title     string   `json:"title"`
	Project   string   `json:"project"`
	GoalID    *int64   `json:"goal_id"`
	Steps     []string `json:"steps"`
	DoneMeans *string  `json:"done_means"`
	Status    string   `json:"status"`
	DoneAt    *string  `json:"done_at"`
	SeedKey   *string  `json:"seed_key"`
	SkillIDs  []int64  `json:"skill_ids"`
	SortOrder int      `json:"sort_order"`
	Version   int      `json:"version"`
	CreatedAt string   `json:"created_at"`
}

// ExportLogEntry is the raw log_entries row, without display joins.
type ExportLogEntry struct {
	ID         int64   `json:"id"`
	CategoryID string  `json:"category_id"`
	Title      string  `json:"title"`
	Note       *string `json:"note"`
	URL        *string `json:"url"`
	GoalID     *int64  `json:"goal_id"`
	OccurredAt string  `json:"occurred_at"`
	CreatedAt  string  `json:"created_at"`
}

// ExportCheckpoint includes the row id for a complete checkpoint dump.
type ExportCheckpoint struct {
	ID          int64    `json:"id"`
	Week        string   `json:"week"`
	Questions   []string `json:"questions"`
	Helpers     []string `json:"helpers"`
	Answers     []string `json:"answers"`
	CompletedAt *string  `json:"completed_at"`
}

// Export returns every application table complete and unpaginated.
func (s *Store) Export(ctx context.Context) (ExportData, error) {
	version, err := s.SchemaVersion(ctx)
	if err != nil {
		return ExportData{}, err
	}

	planMeta, err := s.exportPlanMeta(ctx)
	if err != nil {
		return ExportData{}, err
	}

	planConfig, err := s.exportPlanConfig(ctx)
	if err != nil {
		return ExportData{}, err
	}

	planSources, err := s.exportPlanSources(ctx)
	if err != nil {
		return ExportData{}, err
	}

	projects, err := s.ListProjects(ctx)
	if err != nil {
		return ExportData{}, err
	}

	phases, err := s.exportPhases(ctx)
	if err != nil {
		return ExportData{}, err
	}

	skillTiers, err := s.exportSkillTiers(ctx)
	if err != nil {
		return ExportData{}, err
	}

	weeks, err := s.ListWeeks(ctx)
	if err != nil {
		return ExportData{}, err
	}

	rhythm, err := s.ListRhythm(ctx)
	if err != nil {
		return ExportData{}, err
	}

	categories, err := s.exportCategories(ctx)
	if err != nil {
		return ExportData{}, err
	}

	metricDefs, err := s.ListMetricDefs(ctx)
	if err != nil {
		return ExportData{}, err
	}

	goals, err := s.exportGoals(ctx)
	if err != nil {
		return ExportData{}, err
	}

	tasks, err := s.exportTasks(ctx)
	if err != nil {
		return ExportData{}, err
	}

	logs, err := s.exportLogEntries(ctx)
	if err != nil {
		return ExportData{}, err
	}

	reviews, err := s.ListReviews(ctx, "", "")
	if err != nil {
		return ExportData{}, err
	}

	metrics, err := s.exportMetrics(ctx)
	if err != nil {
		return ExportData{}, err
	}

	skills, err := s.ListSkills(ctx)
	if err != nil {
		return ExportData{}, err
	}

	taskSkills, err := s.exportSkillLinks(ctx, "task_skills", "task_id")
	if err != nil {
		return ExportData{}, err
	}

	logSkills, err := s.exportSkillLinks(ctx, "log_skills", "log_id")
	if err != nil {
		return ExportData{}, err
	}

	xpRules, err := s.exportXPRules(ctx)
	if err != nil {
		return ExportData{}, err
	}

	xpEvents, err := s.exportXPEvents(ctx)
	if err != nil {
		return ExportData{}, err
	}

	xpEventSkills, err := s.exportXPEventSkills(ctx)
	if err != nil {
		return ExportData{}, err
	}

	achievements, err := s.exportAchievements(ctx)
	if err != nil {
		return ExportData{}, err
	}

	achievementUnlocks, err := s.exportAchievementUnlocks(ctx)
	if err != nil {
		return ExportData{}, err
	}

	checkpoints, err := s.exportCheckpoints(ctx)
	if err != nil {
		return ExportData{}, err
	}

	return ExportData{
		ExportedAt:         s.utcNow(),
		SchemaVersion:      version,
		PlanMeta:           planMeta,
		PlanConfig:         planConfig,
		PlanSources:        planSources,
		Projects:           projects,
		Phases:             phases,
		SkillTiers:         skillTiers,
		Weeks:              weeks,
		Rhythm:             rhythm,
		Categories:         categories,
		MetricDefs:         metricDefs,
		Goals:              goals,
		Tasks:              tasks,
		LogEntries:         logs,
		DailyReviews:       reviews,
		Metrics:            metrics,
		Checkpoints:        checkpoints,
		Skills:             skills,
		TaskSkills:         taskSkills,
		LogSkills:          logSkills,
		XPRules:            xpRules,
		XPEvents:           xpEvents,
		XPEventSkills:      xpEventSkills,
		Achievements:       achievements,
		AchievementUnlocks: achievementUnlocks,
	}, nil
}

func (s *Store) exportPlanMeta(ctx context.Context) ([]ExportPlanMeta, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, loaded_at FROM plan_meta ORDER BY id`)
	if err != nil {
		return nil, classify("export plan meta", err)
	}
	defer rows.Close()

	var out []ExportPlanMeta
	for rows.Next() {
		var p ExportPlanMeta
		if err := rows.Scan(&p.ID, &p.Name, &p.LoadedAt); err != nil {
			return nil, classify("scan export plan meta", err)
		}
		out = append(out, p)
	}

	return out, classify("export plan meta", rows.Err())
}

func (s *Store) exportPlanConfig(ctx context.Context) ([]ExportConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM plan_config ORDER BY key`)
	if err != nil {
		return nil, classify("export plan config", err)
	}
	defer rows.Close()

	var out []ExportConfig
	for rows.Next() {
		var c ExportConfig
		if err := rows.Scan(&c.Key, &c.Value); err != nil {
			return nil, classify("scan export plan config", err)
		}
		out = append(out, c)
	}

	return out, classify("export plan config", rows.Err())
}

func (s *Store) exportPlanSources(ctx context.Context) ([]PlanSource, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, plan_id, sha256, source, loaded_at FROM plan_sources ORDER BY loaded_at DESC, id DESC`)
	if err != nil {
		return nil, classify("export plan sources", err)
	}
	defer rows.Close()

	var out []PlanSource
	for rows.Next() {
		var source PlanSource
		if err := rows.Scan(
			&source.ID,
			&source.PlanID,
			&source.SHA256,
			&source.Source,
			&source.LoadedAt,
		); err != nil {
			return nil, classify("scan export plan sources", err)
		}
		out = append(out, source)
	}

	return out, classify("export plan sources", rows.Err())
}

func (s *Store) exportPhases(ctx context.Context) ([]Phase, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, label, sort_order FROM phases ORDER BY sort_order, id`)
	if err != nil {
		return nil, classify("export phases", err)
	}
	defer rows.Close()

	var out []Phase
	for rows.Next() {
		var p Phase
		if err := rows.Scan(&p.ID, &p.Label, &p.SortOrder); err != nil {
			return nil, classify("scan export phase", err)
		}
		out = append(out, p)
	}

	return out, classify("export phases", rows.Err())
}

func (s *Store) exportSkillTiers(ctx context.Context) ([]SkillTier, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, label, sort_order FROM skill_tiers ORDER BY sort_order, id`)
	if err != nil {
		return nil, classify("export skill tiers", err)
	}
	defer rows.Close()

	var out []SkillTier
	for rows.Next() {
		var t SkillTier
		if err := rows.Scan(&t.ID, &t.Label, &t.SortOrder); err != nil {
			return nil, classify("scan export skill tier", err)
		}
		out = append(out, t)
	}

	return out, classify("export skill tiers", rows.Err())
}

func (s *Store) exportSkillLinks(ctx context.Context, table, ownerColumn string) ([]ExportSkillLink, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT %s, skill_id FROM %s ORDER BY %s, skill_id`,
		ownerColumn, table, ownerColumn,
	))
	if err != nil {
		return nil, classify("export "+table, err)
	}
	defer rows.Close()

	var out []ExportSkillLink
	for rows.Next() {
		var (
			ownerID int64
			link    ExportSkillLink
		)
		if err := rows.Scan(&ownerID, &link.SkillID); err != nil {
			return nil, classify("scan export "+table, err)
		}
		if ownerColumn == "task_id" {
			link.TaskID = &ownerID
		} else {
			link.LogID = &ownerID
		}
		out = append(out, link)
	}

	return out, classify("export "+table, rows.Err())
}

func (s *Store) exportXPRules(ctx context.Context) ([]ExportXPRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source, amount FROM xp_rules ORDER BY source`)
	if err != nil {
		return nil, classify("export xp rules", err)
	}
	defer rows.Close()

	var out []ExportXPRule
	for rows.Next() {
		var rule ExportXPRule
		if err := rows.Scan(&rule.Source, &rule.Amount); err != nil {
			return nil, classify("scan export xp rule", err)
		}
		out = append(out, rule)
	}

	return out, classify("export xp rules", rows.Err())
}

func (s *Store) exportXPEvents(ctx context.Context) ([]ExportXPEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, source_type, source_id, amount, created_at FROM xp_events ORDER BY id`)
	if err != nil {
		return nil, classify("export xp events", err)
	}
	defer rows.Close()

	var out []ExportXPEvent
	for rows.Next() {
		var event ExportXPEvent
		if err := rows.Scan(&event.ID, &event.SourceType, &event.SourceID, &event.Amount,
			&event.CreatedAt); err != nil {
			return nil, classify("scan export xp event", err)
		}
		out = append(out, event)
	}

	return out, classify("export xp events", rows.Err())
}

func (s *Store) exportXPEventSkills(ctx context.Context) ([]ExportXPEventSkill, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, skill_id FROM xp_event_skills ORDER BY event_id, skill_id`)
	if err != nil {
		return nil, classify("export xp event skills", err)
	}
	defer rows.Close()

	var out []ExportXPEventSkill
	for rows.Next() {
		var link ExportXPEventSkill
		if err := rows.Scan(&link.EventID, &link.SkillID); err != nil {
			return nil, classify("scan export xp event skill", err)
		}
		out = append(out, link)
	}

	return out, classify("export xp event skills", rows.Err())
}

func (s *Store) exportAchievements(ctx context.Context) ([]ExportAchievement, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, code, name, description, sort_order FROM achievements ORDER BY sort_order, id`)
	if err != nil {
		return nil, classify("export achievements", err)
	}
	defer rows.Close()

	var out []ExportAchievement
	for rows.Next() {
		var achievement ExportAchievement
		if err := rows.Scan(&achievement.ID, &achievement.Code, &achievement.Name,
			&achievement.Description, &achievement.SortOrder); err != nil {
			return nil, classify("scan export achievement", err)
		}
		out = append(out, achievement)
	}

	return out, classify("export achievements", rows.Err())
}

func (s *Store) exportAchievementUnlocks(ctx context.Context) ([]ExportAchievementUnlock, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT achievement_id, unlocked_at FROM achievement_unlocks ORDER BY achievement_id`)
	if err != nil {
		return nil, classify("export achievement unlocks", err)
	}
	defer rows.Close()

	var out []ExportAchievementUnlock
	for rows.Next() {
		var unlock ExportAchievementUnlock
		if err := rows.Scan(&unlock.AchievementID, &unlock.UnlockedAt); err != nil {
			return nil, classify("scan export achievement unlock", err)
		}
		out = append(out, unlock)
	}

	return out, classify("export achievement unlocks", rows.Err())
}

func (s *Store) exportCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, label, icon, sort_order, retired_at FROM categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, classify("export categories", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Label, &c.Icon, &c.SortOrder, &c.RetiredAt); err != nil {
			return nil, classify("scan export categories", err)
		}
		out = append(out, c)
	}

	return out, classify("export categories", rows.Err())
}

func (s *Store) exportGoals(ctx context.Context) ([]Goal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+goalColumns+` FROM goals g ORDER BY g.id`)
	if err != nil {
		return nil, classify("export goals", err)
	}
	defer rows.Close()

	var out []Goal
	for rows.Next() {
		var g Goal
		if err := scanGoal(rows, &g, false); err != nil {
			return nil, classify("scan export goal", err)
		}
		out = append(out, g)
	}

	return out, classify("export goals", rows.Err())
}

func (s *Store) exportTasks(ctx context.Context) ([]ExportTask, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+taskColumns+` FROM tasks ORDER BY id`)
	if err != nil {
		return nil, classify("export tasks", err)
	}
	defer rows.Close()

	var (
		out []ExportTask
		ids []int64
	)

	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, classify("scan export task", err)
		}
		ids = append(ids, task.ID)
		out = append(out, ExportTask(task))
	}

	if err := rows.Err(); err != nil {
		return nil, classify("export tasks", err)
	}

	// The join table has no export row of its own; a task carries its own links so the dump
	// stays restorable from one document.
	links, err := s.TaskSkillIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	for i := range out {
		out[i].SkillIDs = links[out[i].ID]
	}

	return out, nil
}

func (s *Store) exportLogEntries(ctx context.Context) ([]ExportLogEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, category_id, title, note, url, goal_id, occurred_at, created_at
		FROM log_entries ORDER BY id`)
	if err != nil {
		return nil, classify("export log entries", err)
	}
	defer rows.Close()

	var out []ExportLogEntry
	for rows.Next() {
		var e ExportLogEntry
		if err := rows.Scan(&e.ID, &e.CategoryID, &e.Title, &e.Note, &e.URL,
			&e.GoalID, &e.OccurredAt, &e.CreatedAt); err != nil {
			return nil, classify("scan export log entry", err)
		}
		out = append(out, e)
	}

	return out, classify("export log entries", rows.Err())
}

func (s *Store) exportMetrics(ctx context.Context) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, value, unit, note, recorded_at FROM metrics ORDER BY id`)
	if err != nil {
		return nil, classify("export metrics", err)
	}
	defer rows.Close()

	var out []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ID, &m.Name, &m.Value, &m.Unit, &m.Note, &m.RecordedAt); err != nil {
			return nil, classify("scan export metric", err)
		}
		out = append(out, m)
	}

	return out, classify("export metrics", rows.Err())
}

func (s *Store) exportCheckpoints(ctx context.Context) ([]ExportCheckpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, week, questions, helpers, answers, completed_at FROM checkpoints ORDER BY id`)
	if err != nil {
		return nil, classify("export checkpoints", err)
	}
	defer rows.Close()

	var out []ExportCheckpoint
	for rows.Next() {
		var (
			c         ExportCheckpoint
			questions string
			helpers   *string
			answers   *string
		)
		if err := rows.Scan(&c.ID, &c.Week, &questions, &helpers, &answers, &c.CompletedAt); err != nil {
			return nil, classify("scan export checkpoint", err)
		}
		if err := json.Unmarshal([]byte(questions), &c.Questions); err != nil {
			return nil, fmt.Errorf("decode export checkpoint %s questions: %w", c.Week, err)
		}
		if helpers != nil {
			if err := json.Unmarshal([]byte(*helpers), &c.Helpers); err != nil {
				return nil, fmt.Errorf("decode export checkpoint %s helpers: %w", c.Week, err)
			}
		}
		if answers != nil {
			if err := json.Unmarshal([]byte(*answers), &c.Answers); err != nil {
				return nil, fmt.Errorf("decode export checkpoint %s answers: %w", c.Week, err)
			}
		}
		out = append(out, c)
	}

	return out, classify("export checkpoints", rows.Err())
}

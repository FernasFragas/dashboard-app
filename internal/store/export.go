package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExportData is the complete JSON backup document returned by GET /api/export.
type ExportData struct {
	ExportedAt    string             `json:"exported_at"`
	SchemaVersion int                `json:"schema_version"`
	Weeks         []Week             `json:"weeks"`
	Rhythm        []Rhythm           `json:"rhythm"`
	Categories    []Category         `json:"categories"`
	MetricDefs    []MetricDef        `json:"metric_defs"`
	Goals         []Goal             `json:"goals"`
	Tasks         []ExportTask       `json:"tasks"`
	LogEntries    []ExportLogEntry   `json:"log_entries"`
	DailyReviews  []DailyReview      `json:"daily_reviews"`
	Metrics       []Metric           `json:"metrics"`
	Checkpoints   []ExportCheckpoint `json:"checkpoints"`
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
	Answers     []string `json:"answers"`
	CompletedAt *string  `json:"completed_at"`
}

// Export returns every application table complete and unpaginated.
func (s *Store) Export(ctx context.Context) (ExportData, error) {
	version, err := s.SchemaVersion(ctx)
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

	categories, err := s.ListCategories(ctx)
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

	checkpoints, err := s.exportCheckpoints(ctx)
	if err != nil {
		return ExportData{}, err
	}

	return ExportData{
		ExportedAt:    s.utcNow(),
		SchemaVersion: version,
		Weeks:         weeks,
		Rhythm:        rhythm,
		Categories:    categories,
		MetricDefs:    metricDefs,
		Goals:         goals,
		Tasks:         tasks,
		LogEntries:    logs,
		DailyReviews:  reviews,
		Metrics:       metrics,
		Checkpoints:   checkpoints,
	}, nil
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

	var out []ExportTask
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, classify("scan export task", err)
		}
		out = append(out, ExportTask(task))
	}

	return out, classify("export tasks", rows.Err())
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
		`SELECT id, week, questions, answers, completed_at FROM checkpoints ORDER BY id`)
	if err != nil {
		return nil, classify("export checkpoints", err)
	}
	defer rows.Close()

	var out []ExportCheckpoint
	for rows.Next() {
		var (
			c         ExportCheckpoint
			questions string
			answers   *string
		)
		if err := rows.Scan(&c.ID, &c.Week, &questions, &answers, &c.CompletedAt); err != nil {
			return nil, classify("scan export checkpoint", err)
		}
		if err := json.Unmarshal([]byte(questions), &c.Questions); err != nil {
			return nil, fmt.Errorf("decode export checkpoint %s questions: %w", c.Week, err)
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

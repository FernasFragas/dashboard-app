package seed

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"time"
)

//go:embed seed.json
var seedFS embed.FS

// Load parses the embedded seed document.
func Load() (Document, error) {
	body, err := seedFS.ReadFile("seed.json")
	if err != nil {
		return Document{}, fmt.Errorf("read embedded seed.json: %w", err)
	}

	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return Document{}, fmt.Errorf("decode seed.json: %w", err)
	}

	return doc, nil
}

// Result reports how many rows each table gained. Every count is zero on a re-run against an
// unchanged plan, which is what makes the loader safe to run at every boot.
type Result struct {
	Weeks       int
	Rhythm      int
	Categories  int
	MetricDefs  int
	Goals       int
	Tasks       int
	Checkpoints int
}

// Total reports the number of rows inserted across all tables.
func (r Result) Total() int {
	return r.Weeks + r.Rhythm + r.Categories + r.MetricDefs + r.Goals + r.Tasks + r.Checkpoints
}

// Apply writes the document to the database as an additive upsert (docs/DATABASE.md section 8).
//
// Insert only: never UPDATE, never DELETE. A row that already exists keeps its status, done_at,
// completed_at, sort_order and any edits. Rows with a NULL code or seed_key - the goals and
// tasks created in the app - are invisible here and can never be touched.
//
// The whole load runs in one transaction: a partial seed is worse than none.
func Apply(ctx context.Context, db *sql.DB, doc Document) (Result, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var res Result

	for _, w := range doc.Weeks {
		n, err := exec(ctx, tx, `
			INSERT INTO weeks (code, phase, start_date, end_date, focus, sort_order)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (code) DO NOTHING`,
			w.Code, w.Phase, w.StartDate, w.EndDate, w.Focus, w.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed week %s: %w", w.Code, err)
		}
		res.Weeks += n
	}

	for _, r := range doc.Rhythm {
		n, err := exec(ctx, tx, `
			INSERT INTO rhythm (label, weekdays, slot, sort_order)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (label) DO NOTHING`,
			r.Label, r.Weekdays, r.Slot, r.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed rhythm %s: %w", r.Label, err)
		}
		res.Rhythm += n
	}

	for _, c := range doc.Categories {
		n, err := exec(ctx, tx, `
			INSERT INTO categories (id, label, icon, sort_order)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (id) DO NOTHING`,
			c.ID, c.Label, c.Icon, c.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed category %s: %w", c.ID, err)
		}
		res.Categories += n
	}

	for _, m := range doc.MetricDefs {
		n, err := exec(ctx, tx, `
			INSERT INTO metric_defs (name, unit, baseline, target, sort_order)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (name) DO NOTHING`,
			m.Name, m.Unit, m.Baseline, m.Target, m.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed metric def %q: %w", m.Name, err)
		}
		res.MetricDefs += n
	}

	for _, g := range doc.Goals {
		n, err := exec(ctx, tx, `
			INSERT INTO goals (code, title, done_means, project, phase, status, target,
				sort_order, version, created_at, completed_at)
			VALUES (?, ?, ?, ?, ?, 'backlog', ?, ?, 1, ?, NULL)
			ON CONFLICT (code) DO NOTHING`,
			g.Code, g.Title, g.DoneMeans, g.Project, g.Phase, g.Target, g.SortOrder, now)
		if err != nil {
			return Result{}, fmt.Errorf("seed goal %s: %w", g.Code, err)
		}
		res.Goals += n
	}

	for _, t := range doc.Tasks {
		steps, err := json.Marshal(t.Steps)
		if err != nil {
			return Result{}, fmt.Errorf("seed task %s: encode steps: %w", t.SeedKey, err)
		}

		n, err := exec(ctx, tx, `
			INSERT INTO tasks (week, title, project, goal_id, steps, done_means, status,
				done_at, seed_key, sort_order, version, created_at)
			VALUES (?, ?, ?, NULL, ?, ?, 'todo', NULL, ?, ?, 1, ?)
			ON CONFLICT (seed_key) DO NOTHING`,
			t.Week, t.Title, t.Project, string(steps), t.DoneMeans, t.SeedKey, t.SortOrder, now)
		if err != nil {
			return Result{}, fmt.Errorf("seed task %s: %w", t.SeedKey, err)
		}
		res.Tasks += n
	}

	for _, c := range doc.Checkpoints {
		questions, err := json.Marshal(c.Questions)
		if err != nil {
			return Result{}, fmt.Errorf("seed checkpoint %s: encode questions: %w", c.Week, err)
		}

		n, err := exec(ctx, tx, `
			INSERT INTO checkpoints (week, questions, answers, completed_at)
			VALUES (?, ?, NULL, NULL)
			ON CONFLICT (week) DO NOTHING`,
			c.Week, string(questions))
		if err != nil {
			return Result{}, fmt.Errorf("seed checkpoint %s: %w", c.Week, err)
		}
		res.Checkpoints += n
	}

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit seed transaction: %w", err)
	}

	return res, nil
}

func exec(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

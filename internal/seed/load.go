package seed

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
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
	Projects    int
	Phases      int
	SkillTiers  int
	Weeks       int
	Rhythm      int
	Categories  int
	MetricDefs  int
	Goals       int
	Tasks       int
	Checkpoints int
	Skills      int
}

// Total reports the number of rows inserted across all tables.
func (r Result) Total() int {
	return r.Projects + r.Phases + r.SkillTiers + r.Weeks + r.Rhythm + r.Categories +
		r.MetricDefs + r.Goals + r.Tasks + r.Checkpoints + r.Skills
}

// ErrPlanMismatch is returned when a database already holds a different plan. The caller should
// use another data directory or explicitly reset after taking a backup.
var ErrPlanMismatch = errors.New("database already holds a different plan")

// Apply writes the document to the database as an additive reference upsert
// (docs/DATABASE.md section 8).
//
// Reference tables are updated from the seed. Progress tables and state columns are never
// overwritten: goals and tasks are insert-only, and checkpoint answers/completed_at survive.
//
// The whole load runs in one transaction: a partial seed is worse than none.
func Apply(ctx context.Context, db *sql.DB, doc Document) (Result, error) {
	return apply(ctx, db, doc, false)
}

// Replace swaps plan-owned rows for a different uploaded plan while preserving history tables.
// Log entries, daily reviews and metrics are never deleted. Log goal links are nulled by the
// existing FK rule; log skill links cascade when the old skill catalogue is replaced.
func Replace(ctx context.Context, db *sql.DB, doc Document) (Result, error) {
	return apply(ctx, db, doc, true)
}

func apply(ctx context.Context, db *sql.DB, doc Document, replace bool) (Result, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	plan := planWithDefaults(doc.Plan)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var res Result

	if replace {
		if err := resetPlanOwnedRows(ctx, tx, doc, now); err != nil {
			return Result{}, err
		}
	}

	if err := ensurePlan(ctx, tx, plan, now); err != nil {
		return Result{}, err
	}
	if err := upsertPlanConfig(ctx, tx, "active_goal_limit", fmt.Sprintf("%d", plan.ActiveGoalLimit)); err != nil {
		return Result{}, err
	}

	for _, p := range doc.Projects {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM projects WHERE id = ?`,
			[]any{p.ID},
			`
			INSERT INTO projects (id, label, sort_order)
			VALUES (?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				label = excluded.label,
				sort_order = excluded.sort_order`,
			p.ID, p.Label, p.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed project %s: %w", p.ID, err)
		}
		res.Projects += n
	}

	for _, p := range doc.Phases {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM phases WHERE id = ?`,
			[]any{p.ID},
			`
			INSERT INTO phases (id, label, sort_order)
			VALUES (?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				label = excluded.label,
				sort_order = excluded.sort_order`,
			p.ID, p.Label, p.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed phase %s: %w", p.ID, err)
		}
		res.Phases += n
	}

	for _, tier := range doc.SkillTiers {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM skill_tiers WHERE id = ?`,
			[]any{tier.ID},
			`
			INSERT INTO skill_tiers (id, label, sort_order)
			VALUES (?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				label = excluded.label,
				sort_order = excluded.sort_order`,
			tier.ID, tier.Label, tier.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed skill tier %s: %w", tier.ID, err)
		}
		res.SkillTiers += n
	}

	for _, w := range doc.Weeks {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM weeks WHERE code = ?`,
			[]any{w.Code},
			`
			INSERT INTO weeks (code, phase, start_date, end_date, focus, sort_order)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (code) DO UPDATE SET
				phase = excluded.phase,
				start_date = excluded.start_date,
				end_date = excluded.end_date,
				focus = excluded.focus,
				sort_order = excluded.sort_order`,
			w.Code, w.Phase, w.StartDate, w.EndDate, w.Focus, w.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed week %s: %w", w.Code, err)
		}
		res.Weeks += n
	}

	for _, r := range doc.Rhythm {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM rhythm WHERE label = ?`,
			[]any{r.Label},
			`
			INSERT INTO rhythm (label, weekdays, slot, sort_order)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (label) DO UPDATE SET
				weekdays = excluded.weekdays,
				slot = excluded.slot,
				sort_order = excluded.sort_order`,
			r.Label, r.Weekdays, r.Slot, r.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed rhythm %s: %w", r.Label, err)
		}
		res.Rhythm += n
	}

	for _, c := range doc.Categories {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM categories WHERE id = ?`,
			[]any{c.ID},
			`
			INSERT INTO categories (id, label, icon, sort_order, retired_at)
			VALUES (?, ?, ?, ?, NULL)
			ON CONFLICT (id) DO UPDATE SET
				label = excluded.label,
				icon = excluded.icon,
				sort_order = excluded.sort_order,
				retired_at = NULL`,
			c.ID, c.Label, c.Icon, c.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed category %s: %w", c.ID, err)
		}
		res.Categories += n
	}

	for _, m := range doc.MetricDefs {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM metric_defs WHERE name = ?`,
			[]any{m.Name},
			`
			INSERT INTO metric_defs (
				name, slug, unit, baseline, target, definition, how_to_measure, sort_order
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (name) DO UPDATE SET
				slug = excluded.slug,
				unit = excluded.unit,
				baseline = excluded.baseline,
				target = excluded.target,
				definition = excluded.definition,
				how_to_measure = excluded.how_to_measure,
				sort_order = excluded.sort_order`,
			m.Name, m.Slug, m.Unit, m.Baseline, m.Target, m.Definition, m.HowToMeasure, m.SortOrder)
		if err != nil {
			return Result{}, fmt.Errorf("seed metric def %q: %w", m.Name, err)
		}
		res.MetricDefs += n
	}

	for _, s := range doc.Skills {
		n, err := upsert(ctx, tx,
			`SELECT 1 FROM skills WHERE code = ?`,
			[]any{s.Code},
			`
			INSERT INTO skills (code, name, description, associate_when, target_tier,
				sort_order, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (code) DO UPDATE SET
				name = excluded.name,
				description = excluded.description,
				associate_when = excluded.associate_when,
				target_tier = excluded.target_tier,
				sort_order = excluded.sort_order`,
			s.Code, s.Name, s.Description, s.AssociateWhen, s.TargetTier, s.SortOrder, now)
		if err != nil {
			return Result{}, fmt.Errorf("seed skill %s: %w", s.Code, err)
		}
		res.Skills += n
	}

	skillIDs, err := skillIDsByCode(ctx, tx)
	if err != nil {
		return Result{}, err
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

		if err := linkTaskSkills(ctx, tx, skillIDs, t); err != nil {
			return Result{}, err
		}
	}

	for _, c := range doc.Checkpoints {
		if len(c.Helpers) > 0 && len(c.Helpers) != len(c.Questions) {
			return Result{}, fmt.Errorf(
				"seed checkpoint %s: got %d helpers for %d questions",
				c.Week, len(c.Helpers), len(c.Questions),
			)
		}

		questions, err := json.Marshal(c.Questions)
		if err != nil {
			return Result{}, fmt.Errorf("seed checkpoint %s: encode questions: %w", c.Week, err)
		}

		var helpers any
		if len(c.Helpers) > 0 {
			encodedHelpers, err := json.Marshal(c.Helpers)
			if err != nil {
				return Result{}, fmt.Errorf("seed checkpoint %s: encode helpers: %w", c.Week, err)
			}
			helpers = string(encodedHelpers)
		}

		n, err := upsert(ctx, tx,
			`SELECT 1 FROM checkpoints WHERE week = ?`,
			[]any{c.Week},
			`
			INSERT INTO checkpoints (week, questions, helpers, answers, completed_at)
			VALUES (?, ?, ?, NULL, NULL)
			ON CONFLICT (week) DO UPDATE SET
				questions = excluded.questions,
				helpers = excluded.helpers`,
			c.Week, string(questions), helpers)
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

func resetPlanOwnedRows(ctx context.Context, tx *sql.Tx, doc Document, now string) error {
	newCategories := make(map[string]struct{}, len(doc.Categories))
	for _, category := range doc.Categories {
		newCategories[category.ID] = struct{}{}
	}

	rows, err := tx.QueryContext(ctx, `SELECT id FROM categories`)
	if err != nil {
		return fmt.Errorf("list categories before replace: %w", err)
	}
	var oldCategories []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan category before replace: %w", err)
		}
		oldCategories = append(oldCategories, id)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("list categories before replace: %w", err)
	}

	statements := []string{
		`DELETE FROM task_skills`,
		`DELETE FROM tasks`,
		`DELETE FROM checkpoints`,
		`DELETE FROM goals`,
		`DELETE FROM skills`,
		`DELETE FROM weeks`,
		`DELETE FROM rhythm`,
		`DELETE FROM metric_defs`,
		`DELETE FROM skill_tiers`,
		`DELETE FROM phases`,
		`DELETE FROM projects`,
		`DELETE FROM plan_config`,
		`DELETE FROM plan_meta`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("replace plan data: %w", err)
		}
	}

	for _, id := range oldCategories {
		if _, keep := newCategories[id]; keep {
			continue
		}

		var refs int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM log_entries WHERE category_id = ?`, id).Scan(&refs); err != nil {
			return fmt.Errorf("count log refs for category %s: %w", id, err)
		}

		if refs > 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE categories SET retired_at = coalesce(retired_at, ?) WHERE id = ?`,
				now, id,
			); err != nil {
				return fmt.Errorf("retire category %s: %w", id, err)
			}
			continue
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete category %s: %w", id, err)
		}
	}

	return nil
}

func planWithDefaults(plan Plan) Plan {
	if plan.ID == "" {
		plan.ID = "embedded"
	}
	if plan.Name == "" {
		plan.Name = plan.ID
	}
	if plan.ActiveGoalLimit < 1 {
		plan.ActiveGoalLimit = 3
	}
	return plan
}

func ensurePlan(ctx context.Context, tx *sql.Tx, plan Plan, loadedAt string) error {
	var existingID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM plan_meta ORDER BY loaded_at LIMIT 1`).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read plan identity: %w", err)
	}

	if err == nil && existingID != plan.ID {
		return fmt.Errorf("database holds plan %q, refused to load %q: %w",
			existingID, plan.ID, ErrPlanMismatch)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO plan_meta (id, name, loaded_at)
		VALUES (?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = excluded.name,
			loaded_at = excluded.loaded_at`,
		plan.ID, plan.Name, loadedAt,
	); err != nil {
		return fmt.Errorf("record plan identity: %w", err)
	}

	return nil
}

func upsertPlanConfig(ctx context.Context, tx *sql.Tx, key, value string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO plan_config (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		key, value,
	); err != nil {
		return fmt.Errorf("seed plan config %s: %w", key, err)
	}

	return nil
}

// linkTaskSkills replaces a seeded task's skill links.
//
// Replaced rather than inserted-if-missing, so correcting the mapping in cmd/seedgen actually
// reaches an existing database. Only rows the seed owns are touched: the lookup is by seed_key,
// so a task you created in the app has no links rewritten under it.
func linkTaskSkills(ctx context.Context, tx *sql.Tx, skillIDs map[string]int64, t Task) error {
	if len(t.Skills) == 0 {
		if len(skillIDs) == 0 {
			return nil
		}
		return fmt.Errorf("seed task %s: no skills: every task must build at least one", t.SeedKey)
	}

	var taskID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM tasks WHERE seed_key = ?`, t.SeedKey).Scan(&taskID)
	if err != nil {
		return fmt.Errorf("seed task %s: find id: %w", t.SeedKey, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM task_skills WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("seed task %s: clear skills: %w", t.SeedKey, err)
	}

	for _, code := range t.Skills {
		skillID, ok := skillIDs[code]
		if !ok {
			return fmt.Errorf("seed task %s: unknown skill %q", t.SeedKey, code)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_skills (task_id, skill_id) VALUES (?, ?)
			 ON CONFLICT DO NOTHING`, taskID, skillID,
		); err != nil {
			return fmt.Errorf("seed task %s: link %q: %w", t.SeedKey, code, err)
		}
	}

	return nil
}

func skillIDsByCode(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT code, id FROM skills`)
	if err != nil {
		return nil, fmt.Errorf("map skill codes: %w", err)
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var (
			code string
			id   int64
		)
		if err := rows.Scan(&code, &id); err != nil {
			return nil, fmt.Errorf("scan skill code: %w", err)
		}
		out[code] = id
	}

	return out, rows.Err()
}

func upsert(
	ctx context.Context,
	tx *sql.Tx,
	existsQuery string,
	existsArgs []any,
	upsertQuery string,
	upsertArgs ...any,
) (int, error) {
	existed, err := rowExists(ctx, tx, existsQuery, existsArgs...)
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, upsertQuery, upsertArgs...); err != nil {
		return 0, err
	}

	if existed {
		return 0, nil
	}
	return 1, nil
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

func rowExists(ctx context.Context, tx *sql.Tx, query string, args ...any) (bool, error) {
	var one int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

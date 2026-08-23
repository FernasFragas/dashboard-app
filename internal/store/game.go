package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

const xpEventColumns = `id, source_type, source_id, amount, created_at`

type skillTierRule struct {
	name string
	xp   int
}

var gameSkillTiers = []skillTierRule{
	{name: "Untrained", xp: 0},
	{name: "Novice", xp: 1},
	{name: "Apprentice", xp: 60},
	{name: "Practitioner", xp: 150},
	{name: "Adept", xp: 300},
	{name: "Expert", xp: 500},
}

func scanXPEvent(row interface{ Scan(...any) error }) (XPEvent, error) {
	var event XPEvent
	err := row.Scan(&event.ID, &event.SourceType, &event.SourceID, &event.Amount, &event.CreatedAt)
	return event, err
}

// SyncTaskXP makes the task ledger row match the task state. Checking a task writes one event;
// unchecking deletes exactly that event. Updating skills on an already-done task rewrites only
// the event skill links, keeping the original XP award idempotent.
func (s *Store) SyncTaskXP(ctx context.Context, taskID int64) (GameAward, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return GameAward{}, err
	}

	if task.Status != "done" {
		return GameAward{}, s.deleteXPEvent(ctx, "task", task.ID)
	}

	source := "task_ad_hoc"
	if task.SeedKey != nil {
		source = "task_seeded"
	}

	amount, ok, err := s.xpRuleAmount(ctx, source)
	if err != nil || !ok {
		return GameAward{}, err
	}

	return s.ensureXPEvent(ctx, "task", task.ID, amount, task.SkillIDs)
}

// AwardGoalDoneXP awards a goal once after it has moved to Done.
func (s *Store) AwardGoalDoneXP(ctx context.Context, goalID int64) (GameAward, error) {
	goal, err := s.GetGoal(ctx, goalID)
	if err != nil {
		return GameAward{}, err
	}
	if goal.Status != "done" {
		return GameAward{}, nil
	}

	amount, ok, err := s.xpRuleAmount(ctx, "goal_done")
	if err != nil || !ok {
		return GameAward{}, err
	}

	return s.ensureXPEvent(ctx, "goal", goal.ID, amount, nil)
}

// AwardLogXP awards one category-specific log event and mirrors it to every linked skill.
func (s *Store) AwardLogXP(ctx context.Context, logID int64) (GameAward, error) {
	entry, err := s.GetLogEntry(ctx, logID)
	if err != nil {
		return GameAward{}, err
	}

	amount, ok, err := s.xpRuleAmount(ctx, "log:"+entry.CategoryID)
	if err != nil || !ok {
		return GameAward{}, err
	}

	return s.ensureXPEvent(ctx, "log", entry.ID, amount, entry.SkillIDs)
}

// AwardReviewXP awards a daily review once per local date.
func (s *Store) AwardReviewXP(ctx context.Context, date string) (GameAward, error) {
	id, err := reviewSourceID(date)
	if err != nil {
		return GameAward{}, err
	}

	amount, ok, err := s.xpRuleAmount(ctx, "daily_review")
	if err != nil || !ok {
		return GameAward{}, err
	}

	return s.ensureXPEvent(ctx, "review", id, amount, nil)
}

// AwardMetricXP awards the "review included a metric" bonus. Metrics have no skill links today.
func (s *Store) AwardMetricXP(ctx context.Context, metricID int64) (GameAward, error) {
	amount, ok, err := s.xpRuleAmount(ctx, "metric")
	if err != nil || !ok {
		return GameAward{}, err
	}

	return s.ensureXPEvent(ctx, "metric", metricID, amount, nil)
}

// GameProfile derives player XP, level, streak and unlocked badges from the ledger.
func (s *Store) GameProfile(ctx context.Context, now time.Time, loc *time.Location) (GameProfile, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT coalesce(sum(amount), 0) FROM xp_events`).Scan(&total); err != nil {
		return GameProfile{}, classify("sum xp", err)
	}

	events, err := s.xpEventTimestamps(ctx)
	if err != nil {
		return GameProfile{}, err
	}

	streak, err := GameStreakFor(events, now, loc)
	if err != nil {
		return GameProfile{}, err
	}

	achievements, err := s.ListAchievements(ctx)
	if err != nil {
		return GameProfile{}, err
	}
	badges := make([]Achievement, 0, len(achievements))
	for _, achievement := range achievements {
		if achievement.UnlockedAt != nil {
			badges = append(badges, achievement)
		}
	}

	level, title, current, nextLevel, next := GameLevelForXP(total)

	return GameProfile{
		TotalXP:          total,
		Level:            level,
		Title:            title,
		CurrentThreshold: current,
		NextLevel:        nextLevel,
		NextThreshold:    next,
		ProgressXP:       total - current,
		ProgressRequired: next - current,
		Streak:           streak,
		Badges:           badges,
	}, nil
}

// GameSkills derives per-skill XP and tier names from xp_event_skills.
func (s *Store) GameSkills(ctx context.Context) ([]GameSkill, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sk.id, sk.code, sk.name, sk.target_tier, coalesce(sum(xe.amount), 0) AS xp
		FROM skills sk
		LEFT JOIN xp_event_skills xes ON xes.skill_id = sk.id
		LEFT JOIN xp_events xe ON xe.id = xes.event_id
		GROUP BY sk.id, sk.code, sk.name, sk.target_tier, sk.sort_order
		ORDER BY sk.sort_order, sk.id`)
	if err != nil {
		return nil, classify("game skills", err)
	}
	defer rows.Close()

	var out []GameSkill
	for rows.Next() {
		var skill GameSkill
		if err := rows.Scan(&skill.SkillID, &skill.Code, &skill.Name, &skill.TargetTier, &skill.XP); err != nil {
			return nil, classify("scan game skill", err)
		}

		skill.Tier, skill.TierMin, skill.NextTier, skill.NextTierXP = GameSkillTierForXP(skill.XP)
		out = append(out, skill)
	}

	return out, classify("game skills", rows.Err())
}

// ListXPEvents returns the recent ledger feed, newest first.
func (s *Store) ListXPEvents(ctx context.Context, limit int) ([]XPEvent, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT xe.id, xe.source_type, xe.source_id, xe.amount, xe.created_at,
			coalesce(
				CASE xe.source_type
				WHEN 'task' THEN (
					SELECT 'Task - ' || t.title FROM tasks t WHERE t.id = xe.source_id
				)
				WHEN 'log' THEN (
					SELECT c.label || ' - ' || le.title
					FROM log_entries le
					JOIN categories c ON c.id = le.category_id
					WHERE le.id = xe.source_id
				)
				WHEN 'goal' THEN (
					SELECT 'Goal - ' || g.title FROM goals g WHERE g.id = xe.source_id
				)
				WHEN 'review' THEN 'Daily review'
				WHEN 'metric' THEN (
					SELECT 'Metric - ' || m.name FROM metrics m WHERE m.id = xe.source_id
				)
				WHEN 'achievement' THEN 'Achievement'
				END,
				xe.source_type || ' #' || xe.source_id
			) AS source_label
		FROM xp_events xe
		ORDER BY xe.created_at DESC, xe.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, classify("list xp events", err)
	}
	defer rows.Close()

	var out []XPEvent
	for rows.Next() {
		var event XPEvent
		if err := rows.Scan(&event.ID, &event.SourceType, &event.SourceID, &event.Amount,
			&event.CreatedAt, &event.SourceLabel); err != nil {
			return nil, classify("scan xp event", err)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, classify("list xp events", err)
	}

	return out, s.attachXPEventSkills(ctx, out)
}

// ListAchievements returns all seeded achievements with any unlock timestamp attached.
func (s *Store) ListAchievements(ctx context.Context) ([]Achievement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.code, a.name, a.description, a.sort_order, au.unlocked_at
		FROM achievements a
		LEFT JOIN achievement_unlocks au ON au.achievement_id = a.id
		ORDER BY a.sort_order, a.id`)
	if err != nil {
		return nil, classify("list achievements", err)
	}
	defer rows.Close()

	var out []Achievement
	for rows.Next() {
		var a Achievement
		if err := rows.Scan(&a.ID, &a.Code, &a.Name, &a.Description, &a.SortOrder, &a.UnlockedAt); err != nil {
			return nil, classify("scan achievement", err)
		}
		out = append(out, a)
	}

	return out, classify("list achievements", rows.Err())
}

// UnlockAchievement records an achievement once and reports whether this call created it.
func (s *Store) UnlockAchievement(ctx context.Context, code string) (Achievement, bool, error) {
	var unlocked Achievement
	inserted := false

	err := s.tx(ctx, func(tx execer) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, code, name, description, sort_order FROM achievements WHERE code = ?`, code)
		if err := row.Scan(&unlocked.ID, &unlocked.Code, &unlocked.Name, &unlocked.Description,
			&unlocked.SortOrder); err != nil {
			return classify("get achievement", err)
		}

		now := s.utcNow()
		res, err := tx.ExecContext(ctx, `
			INSERT INTO achievement_unlocks (achievement_id, unlocked_at)
			VALUES (?, ?)
			ON CONFLICT (achievement_id) DO NOTHING`, unlocked.ID, now)
		if err != nil {
			return classify("unlock achievement", err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return classify("unlock achievement rows", err)
		}
		inserted = affected > 0
		if inserted {
			unlocked.UnlockedAt = &now
			return nil
		}

		return tx.QueryRowContext(ctx, `
			SELECT au.unlocked_at
			FROM achievement_unlocks au
			WHERE au.achievement_id = ?`, unlocked.ID).Scan(&unlocked.UnlockedAt)
	})
	if err != nil {
		return Achievement{}, false, err
	}

	return unlocked, inserted, nil
}

// GameAchievementFacts reads the database once into the counters and sets the API predicates use.
func (s *Store) GameAchievementFacts(ctx context.Context) (GameAchievementFacts, error) {
	facts := GameAchievementFacts{
		DoneSeedKeys:         map[string]bool{},
		DoneGoalCodes:        map[string]bool{},
		CompletedCheckpoints: map[string]bool{},
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM tasks WHERE status = 'done'`).Scan(&facts.DoneTasks); err != nil {
		return facts, classify("count done tasks", err)
	}

	categoryRows, err := s.db.QueryContext(ctx, `
		SELECT category_id, count(*)
		FROM log_entries
		WHERE category_id IN ('number', 'application', 'post')
		GROUP BY category_id`)
	if err != nil {
		return facts, classify("count achievement logs", err)
	}
	for categoryRows.Next() {
		var (
			category string
			count    int
		)
		if err := categoryRows.Scan(&category, &count); err != nil {
			_ = categoryRows.Close()
			return facts, classify("scan achievement logs", err)
		}
		switch category {
		case "number":
			facts.NumberLogs = count
		case "application":
			facts.ApplicationLogs = count
		case "post":
			facts.PostLogs = count
		}
	}
	if err := categoryRows.Close(); err != nil {
		return facts, classify("count achievement logs", err)
	}
	if err := categoryRows.Err(); err != nil {
		return facts, classify("count achievement logs", err)
	}

	seedRows, err := s.db.QueryContext(ctx, `
		SELECT seed_key FROM tasks WHERE status = 'done' AND seed_key IS NOT NULL`)
	if err != nil {
		return facts, classify("list done seed keys", err)
	}
	for seedRows.Next() {
		var key string
		if err := seedRows.Scan(&key); err != nil {
			_ = seedRows.Close()
			return facts, classify("scan done seed key", err)
		}
		facts.DoneSeedKeys[key] = true
	}
	if err := seedRows.Close(); err != nil {
		return facts, classify("list done seed keys", err)
	}
	if err := seedRows.Err(); err != nil {
		return facts, classify("list done seed keys", err)
	}

	goalRows, err := s.db.QueryContext(ctx, `
		SELECT code FROM goals WHERE status = 'done' AND code IS NOT NULL`)
	if err != nil {
		return facts, classify("list done goal codes", err)
	}
	for goalRows.Next() {
		var code string
		if err := goalRows.Scan(&code); err != nil {
			_ = goalRows.Close()
			return facts, classify("scan done goal code", err)
		}
		facts.DoneGoalCodes[code] = true
	}
	if err := goalRows.Close(); err != nil {
		return facts, classify("list done goal codes", err)
	}
	if err := goalRows.Err(); err != nil {
		return facts, classify("list done goal codes", err)
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM (
			SELECT week
			FROM tasks
			WHERE seed_key IS NOT NULL
			GROUP BY week
			HAVING count(*) > 0 AND coalesce(sum(status = 'done'), 0) = count(*)
		)`).Scan(&facts.CompleteWeeks); err != nil {
		return facts, classify("count complete weeks", err)
	}

	checkpointRows, err := s.db.QueryContext(ctx, `
		SELECT week FROM checkpoints WHERE completed_at IS NOT NULL`)
	if err != nil {
		return facts, classify("list completed checkpoints", err)
	}
	for checkpointRows.Next() {
		var week string
		if err := checkpointRows.Scan(&week); err != nil {
			_ = checkpointRows.Close()
			return facts, classify("scan completed checkpoint", err)
		}
		facts.CompletedCheckpoints[week] = true
	}
	if err := checkpointRows.Close(); err != nil {
		return facts, classify("list completed checkpoints", err)
	}
	if err := checkpointRows.Err(); err != nil {
		return facts, classify("list completed checkpoints", err)
	}

	facts.XPEventTimes, err = s.xpEventTimestamps(ctx)
	if err != nil {
		return facts, err
	}

	return facts, nil
}

// GameLevelForXP maps cumulative XP to the level threshold curve from M9.
func GameLevelForXP(total int) (level int, title string, currentThreshold int, nextLevel int, nextThreshold int) {
	for total >= gameLevelThreshold(level+1) {
		level++
	}

	currentThreshold = gameLevelThreshold(level)
	nextLevel = level + 1
	nextThreshold = gameLevelThreshold(nextLevel)

	return level, gameTitleForLevel(level), currentThreshold, nextLevel, nextThreshold
}

func gameLevelThreshold(level int) int {
	if level <= 0 {
		return 0
	}
	return 25 * level * (level + 1)
}

func gameTitleForLevel(level int) string {
	switch {
	case level <= 3:
		return "Backend Engineer"
	case level <= 6:
		return "Systems Engineer"
	case level <= 9:
		return "Platform Engineer"
	case level <= 12:
		return "Distributed Systems Engineer"
	case level <= 15:
		return "AI Infrastructure Engineer"
	default:
		return "Staff / Founder track"
	}
}

// GameSkillTierForXP maps a skill's XP to its current and next named tier.
func GameSkillTierForXP(xp int) (tier string, tierMin int, nextTier *string, nextTierXP int) {
	current := gameSkillTiers[0]
	next := (*skillTierRule)(nil)

	for i, rule := range gameSkillTiers {
		if xp >= rule.xp {
			current = rule
			next = nil
			if i+1 < len(gameSkillTiers) {
				next = &gameSkillTiers[i+1]
			}
		}
	}

	if next == nil {
		return current.name, current.xp, nil, current.xp
	}

	nextName := next.name
	return current.name, current.xp, &nextName, next.xp
}

// GameStreakFor computes consecutive active Lisbon days from XP event timestamps. Sundays with
// no XP after the first event are shield days: they do not count, but they also do not break.
func GameStreakFor(eventTimestamps []string, now time.Time, loc *time.Location) (GameStreak, error) {
	active := map[string]struct{}{}
	var firstEvent *time.Time

	for _, raw := range eventTimestamps {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return GameStreak{}, fmt.Errorf("parse xp timestamp %q: %w", raw, err)
		}

		day := gameLocalDate(ts, loc)
		key := day.Format("2006-01-02")
		active[key] = struct{}{}
		if firstEvent == nil || day.Before(*firstEvent) {
			firstDay := day
			firstEvent = &firstDay
		}
	}

	if firstEvent == nil {
		return GameStreak{}, nil
	}

	today := gameLocalDate(now, loc)
	todayKey := today.Format("2006-01-02")
	_, countsToday := active[todayKey]

	start := today
	shielded := false
	if !countsToday {
		switch {
		case shieldedSunday(today, firstEvent, active):
			shielded = true
			start = today.AddDate(0, 0, -1)
		case hasXP(active, today.AddDate(0, 0, -1)):
			start = today.AddDate(0, 0, -1)
		case shieldedSunday(today.AddDate(0, 0, -1), firstEvent, active):
			shielded = true
			start = today.AddDate(0, 0, -2)
		default:
			return GameStreak{Days: 0, CountsToday: false}, nil
		}
	}

	days := 0
	for day := start; ; day = day.AddDate(0, 0, -1) {
		if hasXP(active, day) {
			days++
			continue
		}

		if shieldedSunday(day, firstEvent, active) {
			shielded = true
			continue
		}

		break
	}

	return GameStreak{Days: days, CountsToday: countsToday, Shielded: shielded}, nil
}

func (s *Store) ensureXPEvent(
	ctx context.Context,
	sourceType string,
	sourceID int64,
	amount int,
	skillIDs []int64,
) (GameAward, error) {
	if amount <= 0 {
		return GameAward{}, nil
	}

	var (
		event    XPEvent
		inserted bool
	)

	err := s.tx(ctx, func(tx execer) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO xp_events (source_type, source_id, amount, created_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (source_type, source_id) DO NOTHING`,
			sourceType, sourceID, amount, s.utcNow())
		if err != nil {
			return classify("insert xp event", err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return classify("insert xp event rows", err)
		}
		inserted = affected > 0

		var readErr error
		event, readErr = scanXPEvent(tx.QueryRowContext(ctx,
			`SELECT `+xpEventColumns+` FROM xp_events WHERE source_type = ? AND source_id = ?`,
			sourceType, sourceID))
		if readErr != nil {
			return classify("read xp event", readErr)
		}

		return replaceLinks(ctx, tx, "xp_event_skills", "event_id", event.ID, skillIDs)
	})
	if err != nil {
		return GameAward{}, err
	}

	if !inserted {
		return GameAward{}, nil
	}

	event, err = s.getXPEvent(ctx, event.ID)
	if err != nil {
		return GameAward{}, err
	}

	return GameAward{XPAwarded: event.Amount, Event: &event}, nil
}

func (s *Store) deleteXPEvent(ctx context.Context, sourceType string, sourceID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM xp_events WHERE source_type = ? AND source_id = ?`, sourceType, sourceID)
	return classify("delete xp event", err)
}

func (s *Store) xpRuleAmount(ctx context.Context, source string) (int, bool, error) {
	var amount int
	err := s.db.QueryRowContext(ctx, `SELECT amount FROM xp_rules WHERE source = ?`, source).Scan(&amount)
	if err == nil {
		return amount, true, nil
	}
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return 0, false, classify("read xp rule", err)
}

func (s *Store) getXPEvent(ctx context.Context, id int64) (XPEvent, error) {
	events, err := s.listXPEventsByWhere(ctx, `xe.id = ?`, id, 1)
	if err != nil {
		return XPEvent{}, err
	}
	if len(events) == 0 {
		return XPEvent{}, fmt.Errorf("get xp event %d: %w", id, ErrNotFound)
	}

	return events[0], nil
}

func (s *Store) listXPEventsByWhere(
	ctx context.Context,
	where string,
	arg any,
	limit int,
) ([]XPEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT xe.id, xe.source_type, xe.source_id, xe.amount, xe.created_at,
			coalesce(
				CASE xe.source_type
				WHEN 'task' THEN (SELECT 'Task - ' || t.title FROM tasks t WHERE t.id = xe.source_id)
				WHEN 'log' THEN (
					SELECT c.label || ' - ' || le.title
					FROM log_entries le
					JOIN categories c ON c.id = le.category_id
					WHERE le.id = xe.source_id
				)
				WHEN 'goal' THEN (SELECT 'Goal - ' || g.title FROM goals g WHERE g.id = xe.source_id)
				WHEN 'review' THEN 'Daily review'
				WHEN 'metric' THEN (SELECT 'Metric - ' || m.name FROM metrics m WHERE m.id = xe.source_id)
				WHEN 'achievement' THEN 'Achievement'
				END,
				xe.source_type || ' #' || xe.source_id
			) AS source_label
		FROM xp_events xe
		WHERE `+where+`
		ORDER BY xe.created_at DESC, xe.id DESC
		LIMIT ?`, arg, limit)
	if err != nil {
		return nil, classify("list xp events", err)
	}
	defer rows.Close()

	var out []XPEvent
	for rows.Next() {
		var event XPEvent
		if err := rows.Scan(&event.ID, &event.SourceType, &event.SourceID, &event.Amount,
			&event.CreatedAt, &event.SourceLabel); err != nil {
			return nil, classify("scan xp event", err)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, classify("list xp events", err)
	}

	return out, s.attachXPEventSkills(ctx, out)
}

func (s *Store) attachXPEventSkills(ctx context.Context, events []XPEvent) error {
	if len(events) == 0 {
		return nil
	}

	ids := make([]any, 0, len(events))
	index := map[int64]int{}
	for i, event := range events {
		ids = append(ids, event.ID)
		index[event.ID] = i
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT xes.event_id, sk.id, sk.code, sk.name
		FROM xp_event_skills xes
		JOIN skills sk ON sk.id = xes.skill_id
		WHERE xes.event_id IN (`+placeholders(len(ids))+`)
		ORDER BY sk.sort_order, sk.id`, ids...)
	if err != nil {
		return classify("list xp event skills", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			eventID int64
			skillID int64
			code    string
			name    string
		)
		if err := rows.Scan(&eventID, &skillID, &code, &name); err != nil {
			return classify("scan xp event skill", err)
		}
		i, ok := index[eventID]
		if !ok {
			continue
		}
		events[i].SkillIDs = append(events[i].SkillIDs, skillID)
		events[i].SkillCodes = append(events[i].SkillCodes, code)
		events[i].SkillNames = append(events[i].SkillNames, name)
	}

	return classify("list xp event skills", rows.Err())
}

func (s *Store) xpEventTimestamps(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT created_at FROM xp_events ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, classify("list xp timestamps", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, classify("scan xp timestamp", err)
		}
		out = append(out, raw)
	}

	return out, classify("list xp timestamps", rows.Err())
}

func reviewSourceID(date string) (int64, error) {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0, fmt.Errorf("review source id %q: %w", date, err)
	}

	id, err := strconv.ParseInt(parsed.Format("20060102"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("review source id %q: %w", date, err)
	}

	return id, nil
}

func gameLocalDate(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func hasXP(active map[string]struct{}, day time.Time) bool {
	_, ok := active[day.Format("2006-01-02")]
	return ok
}

func shieldedSunday(
	day time.Time,
	firstEvent *time.Time,
	active map[string]struct{},
) bool {
	if day.Weekday() != time.Sunday || firstEvent == nil || day.Before(*firstEvent) {
		return false
	}
	return !hasXP(active, day)
}

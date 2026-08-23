package store

// Types mirror docs/DATABASE.md section 4 one-for-one. Nullable columns are pointers so that
// "absent" and "empty string" stay distinguishable through to JSON.
//
// JSON tags match docs/API.md so the API layer can encode these directly; fields the API
// composes rather than stores (log_count, category_label) are marked omitempty.

// Week is a seeded plan window: W1..W12 and B1..B7.
type Week struct {
	Code      string  `json:"code"`
	Phase     string  `json:"phase"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Focus     *string `json:"focus"`
	SortOrder int     `json:"sort_order"`
}

// Rhythm is one row of the operating-system table. Weekdays is a CSV of ISO weekday numbers
// (1=Mon .. 7=Sun) because the plan groups days: "Tue-Wed" is "2,3".
type Rhythm struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	Weekdays  string `json:"weekdays"`
	Slot      string `json:"slot"`
	SortOrder int    `json:"sort_order"`
}

// Category is one of the eight quick-log buttons.
type Category struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Icon      string  `json:"icon"`
	SortOrder int     `json:"sort_order"`
	RetiredAt *string `json:"retired_at,omitempty"`
}

// Project is plan vocabulary for goals and tasks.
type Project struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// Phase is plan vocabulary for week windows and optional goal phase labels.
type Phase struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// SkillTier is plan vocabulary for a skill's target tier.
type SkillTier struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// PlanConfig is plan-level behavior that used to be compiled into handlers.
type PlanConfig struct {
	ActiveGoalLimit int `json:"active_goal_limit"`
}

// PlanMeta is the identity of the plan currently loaded into this database.
type PlanMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LoadedAt string `json:"loaded_at"`
}

// PlanSource is one retained uploaded markdown source for a plan.
type PlanSource struct {
	ID       int64  `json:"id"`
	PlanID   string `json:"plan_id"`
	SHA256   string `json:"sha256"`
	Source   string `json:"source"`
	LoadedAt string `json:"loaded_at"`
}

// PlanCounts is the small current-plan summary shown on the Plan screen.
type PlanCounts struct {
	Weeks    int `json:"weeks"`
	Goals    int `json:"goals"`
	Tasks    int `json:"tasks"`
	Skills   int `json:"skills"`
	Projects int `json:"projects"`
}

// PlanChangeCounts is a compact added/updated/removed summary for one seeded table.
type PlanChangeCounts struct {
	Added     int `json:"added,omitempty"`
	Updated   int `json:"updated,omitempty"`
	Removed   int `json:"removed,omitempty"`
	Unchanged int `json:"unchanged,omitempty"`
	Orphaned  int `json:"orphaned,omitempty"`
}

// PlanDiff compares a parsed uploaded plan to rows already in the database.
type PlanDiff struct {
	Weeks PlanChangeCounts `json:"weeks"`
	Tasks PlanChangeCounts `json:"tasks"`
	Goals PlanChangeCounts `json:"goals"`
}

// PlanFingerprints is the store-neutral shape used to diff parsed plan data against rows.
type PlanFingerprints struct {
	Weeks map[string]string
	Tasks map[string]string
	Goals map[string]string
}

// PlanAtRisk names preserved user state whose links may be affected by replacing a plan.
type PlanAtRisk struct {
	TaskCompletions   int `json:"task_completions"`
	GoalPositions     int `json:"goal_positions"`
	CheckpointAnswers int `json:"checkpoint_answers"`
	LogGoalLinks      int `json:"log_goal_links"`
	LogSkillLinks     int `json:"log_skill_links"`
	RetiredCategories int `json:"retired_categories"`
}

// MetricDef is a seeded metric name with its baseline and target as written in the plan.
type MetricDef struct {
	Name         string  `json:"name"`
	Slug         *string `json:"slug"`
	Unit         *string `json:"unit"`
	Baseline     *string `json:"baseline"`
	Target       *string `json:"target"`
	Definition   *string `json:"definition"`
	HowToMeasure *string `json:"how_to_measure"`
	SortOrder    int     `json:"sort_order"`
}

// Goal is a Kanban card. Code is non-nil only for seeded goals (G0..G17).
type Goal struct {
	ID          int64   `json:"id"`
	Code        *string `json:"code"`
	Title       string  `json:"title"`
	DoneMeans   *string `json:"done_means"`
	Project     string  `json:"project"`
	Phase       *string `json:"phase"`
	Status      string  `json:"status"`
	Target      *string `json:"target"`
	SortOrder   int     `json:"sort_order"`
	Version     int     `json:"version"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at"`

	// LogCount is the "proof of motion" chip: log entries pointing at this goal. Populated by
	// ListGoals, zero elsewhere.
	LogCount int `json:"log_count"`
}

// Task is one checklist item for a plan week. SeedKey is non-nil only for seeded tasks.
type Task struct {
	ID        int64    `json:"id"`
	Week      string   `json:"week"`
	Title     string   `json:"title"`
	Project   string   `json:"project"`
	GoalID    *int64   `json:"goal_id"`
	Steps     []string `json:"steps"`
	DoneMeans *string  `json:"done_means"`
	Status    string   `json:"status"`
	DoneAt    *string  `json:"done_at"`
	SeedKey   *string  `json:"-"`
	SkillIDs  []int64  `json:"skill_ids"`
	SortOrder int      `json:"sort_order"`
	Version   int      `json:"version"`
	CreatedAt string   `json:"created_at"`
}

// LogEntry is an accomplishment. Append and delete only; never updated.
type LogEntry struct {
	ID         int64   `json:"id"`
	CategoryID string  `json:"category_id"`
	Title      string  `json:"title"`
	Note       *string `json:"note"`
	URL        *string `json:"url"`
	GoalID     *int64  `json:"goal_id"`
	OccurredAt string  `json:"occurred_at"`
	CreatedAt  string  `json:"created_at"`

	SkillIDs []int64 `json:"skill_ids,omitempty"`

	// Joined for display by ListLogEntries.
	CategoryLabel string  `json:"category_label,omitempty"`
	Icon          string  `json:"icon,omitempty"`
	GoalCode      *string `json:"goal_code,omitempty"`
}

// DailyReview is the 3-bullet daily entry, keyed by a Europe/Lisbon calendar date.
type DailyReview struct {
	Date      string  `json:"date"`
	Learned   *string `json:"learned"`
	Issue     *string `json:"issue"`
	Next      *string `json:"next"`
	Minutes   *int    `json:"minutes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// Metric is one reading in a free-form time series.
type Metric struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Unit       *string `json:"unit"`
	Note       *string `json:"note"`
	RecordedAt string  `json:"recorded_at"`
}

// Checkpoint is the five-question review at W12 and B7. Answers is index-aligned with
// Questions and nil until first saved.
type Checkpoint struct {
	ID          int64    `json:"-"`
	Week        string   `json:"week"`
	Questions   []string `json:"questions"`
	Helpers     []string `json:"helpers"`
	Answers     []string `json:"answers"`
	CompletedAt *string  `json:"completed_at"`
}

// Skill is a named capability the work builds. Progress fields are derived at read time from
// the association tables and never stored (ADR-007).
type Skill struct {
	ID            int64   `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	AssociateWhen string  `json:"associate_when"`
	TargetTier    *string `json:"target_tier"`
	SortOrder     int     `json:"sort_order"`
	CreatedAt     string  `json:"created_at"`

	// Derived by ListSkills; zero elsewhere.
	TaskCount     int     `json:"task_count"`
	TaskDone      int     `json:"task_done"`
	EvidenceCount int     `json:"evidence_count"`
	LastActivity  *string `json:"last_activity"`
}

// XPEvent is one immutable ledger entry. Totals, levels, skill XP and streaks are derived from
// these rows; the source fields point back to the action that earned the XP.
type XPEvent struct {
	ID          int64    `json:"id"`
	SourceType  string   `json:"source_type"`
	SourceID    int64    `json:"source_id"`
	Amount      int      `json:"amount"`
	CreatedAt   string   `json:"created_at"`
	SkillIDs    []int64  `json:"skill_ids,omitempty"`
	SkillCodes  []string `json:"skill_codes,omitempty"`
	SkillNames  []string `json:"skill_names,omitempty"`
	SourceLabel string   `json:"source_label,omitempty"`
}

// GameAward is the result of attempting to write one XP event. XPAwarded is zero when the
// ledger already had that source event or when an undone task removed its event.
type GameAward struct {
	XPAwarded int      `json:"xp_awarded"`
	Event     *XPEvent `json:"event,omitempty"`
}

// GameLevelUp carries the post-write player level when an action crosses a threshold.
type GameLevelUp struct {
	Level         int    `json:"level"`
	Title         string `json:"title"`
	TotalXP       int    `json:"total_xp"`
	NextThreshold int    `json:"next_threshold"`
}

// GameEnvelope is optionally attached to write responses that award XP or unlock achievements.
type GameEnvelope struct {
	XPAwarded int           `json:"xp_awarded"`
	LevelUp   *GameLevelUp  `json:"level_up,omitempty"`
	Unlocks   []Achievement `json:"unlocks"`
	Event     *XPEvent      `json:"event,omitempty"`
}

// Achievement is seeded reference data plus an optional historical unlock timestamp.
type Achievement struct {
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SortOrder   int     `json:"sort_order"`
	UnlockedAt  *string `json:"unlocked_at,omitempty"`
}

// GameProfile is the player's derived headline state.
type GameProfile struct {
	TotalXP          int           `json:"total_xp"`
	Level            int           `json:"level"`
	Title            string        `json:"title"`
	CurrentThreshold int           `json:"current_threshold"`
	NextLevel        int           `json:"next_level"`
	NextThreshold    int           `json:"next_threshold"`
	ProgressXP       int           `json:"progress_xp"`
	ProgressRequired int           `json:"progress_required"`
	Streak           GameStreak    `json:"streak"`
	Badges           []Achievement `json:"badges"`
}

// GameStreak counts consecutive Lisbon calendar days with at least one XP event. A zero-XP
// Sunday shields the run instead of breaking it.
type GameStreak struct {
	Days        int  `json:"days"`
	CountsToday bool `json:"counts_today"`
	Shielded    bool `json:"shielded"`
}

// GameSkill is the per-skill XP projection used by the Skills tab and profile sheet.
type GameSkill struct {
	SkillID    int64   `json:"skill_id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	TargetTier *string `json:"target_tier"`
	XP         int     `json:"xp"`
	Tier       string  `json:"tier"`
	TierMin    int     `json:"tier_min"`
	NextTierXP int     `json:"next_tier_xp"`
	NextTier   *string `json:"next_tier"`
}

// GameAchievementFacts is a compact read model for API-layer achievement predicates.
type GameAchievementFacts struct {
	DoneTasks            int
	NumberLogs           int
	ApplicationLogs      int
	PostLogs             int
	DoneSeedKeys         map[string]bool
	DoneGoalCodes        map[string]bool
	CompleteWeeks        int
	CompletedCheckpoints map[string]bool
	XPEventTimes         []string
}

// SkillDetail is one skill plus the work behind its numbers.
type SkillDetail struct {
	Skill
	Tasks    []Task     `json:"tasks"`
	Evidence []LogEntry `json:"evidence"`
}

// CategoryCount is one row of the per-category counters used by the dashboard bundle and the
// Log screen's weekly recap.
type CategoryCount struct {
	CategoryID string `json:"category_id"`
	Label      string `json:"label"`
	Icon       string `json:"icon"`
	Count      int    `json:"count"`
}

// LogFilter narrows the accomplishment feed. Zero values mean "no filter".
type LogFilter struct {
	Categories []string
	// From and To are inclusive RFC3339 UTC bounds on occurred_at.
	From string
	To   string
	// Cursor is the occurred_at of the last row of the previous page.
	Cursor string
	Limit  int
}

// GoalFilter narrows the Kanban query.
type GoalFilter struct {
	Status   string
	Projects []string
}

// TaskFilter narrows the week checklist.
type TaskFilter struct {
	Week     string
	Projects []string
}

// MetricFilter narrows a time-series read.
type MetricFilter struct {
	Name  string
	From  string
	To    string
	Limit int
}

// NewGoal is the input to CreateGoal. Code is not settable: it belongs to seeded goals.
type NewGoal struct {
	Title     string
	DoneMeans *string
	Project   string
	Phase     *string
	Status    string
	Target    *string
}

// GoalPatch carries the fields UpdateGoal may change. Nil means "leave alone".
type GoalPatch struct {
	Title     *string
	DoneMeans *string
	Project   *string
	Target    *string
	// Status and Position move the card between or within columns. Both must be set together.
	Status   *string
	Position *int
}

// NewTask is the input to CreateTask.
type NewTask struct {
	Week      string
	Title     string
	Project   string
	GoalID    *int64
	Steps     []string
	DoneMeans *string
	// SkillIDs must hold at least one skill: every task builds one (docs/DATABASE.md §4.7).
	SkillIDs []int64
}

// TaskPatch carries the fields UpdateTask may change. Nil means "leave alone".
type TaskPatch struct {
	Title     *string
	Project   *string
	GoalID    **int64
	Status    *string
	DoneMeans *string
	// SkillIDs replaces the task's links when non-nil. An empty slice is rejected.
	SkillIDs *[]int64
}

// NewLogEntry is the input to CreateLogEntry.
type NewLogEntry struct {
	CategoryID string
	Title      string
	Note       *string
	URL        *string
	GoalID     *int64
	// OccurredAt may be backdated; empty means now.
	OccurredAt string
	// SkillIDs is optional evidence: the quick-log path must stay cheap.
	SkillIDs []int64
}

// NewMetric is the input to CreateMetric.
type NewMetric struct {
	Name  string
	Value float64
	Unit  *string
	Note  *string
	// RecordedAt empty means now.
	RecordedAt string
}

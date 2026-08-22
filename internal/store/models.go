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
	ID        string `json:"id"`
	Label     string `json:"label"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
}

// MetricDef is a seeded metric name with its baseline and target as written in the plan.
type MetricDef struct {
	Name      string  `json:"name"`
	Unit      *string `json:"unit"`
	Baseline  *string `json:"baseline"`
	Target    *string `json:"target"`
	SortOrder int     `json:"sort_order"`
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
	Answers     []string `json:"answers"`
	CompletedAt *string  `json:"completed_at"`
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
}

// TaskPatch carries the fields UpdateTask may change. Nil means "leave alone".
type TaskPatch struct {
	Title     *string
	Project   *string
	GoalID    **int64
	Status    *string
	DoneMeans *string
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

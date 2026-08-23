// Package seed holds the generated seed document and the additive reference-upsert loader that
// runs at boot. See docs/DATABASE.md section 8 for the policy.
package seed

// Document is the shape of seed.json. It is generated from master-plan-v5.md by cmd/seedgen
// (`make seed-gen`) and committed, so a parser bug can never prevent a boot: the server only
// ever reads this JSON, never the markdown.
type Document struct {
	GeneratedFrom string       `json:"generated_from"`
	Plan          Plan         `json:"plan"`
	Projects      []Project    `json:"projects"`
	Phases        []Phase      `json:"phases"`
	SkillTiers    []SkillTier  `json:"skill_tiers"`
	Weeks         []Week       `json:"weeks"`
	Rhythm        []Rhythm     `json:"rhythm"`
	Categories    []Category   `json:"categories"`
	MetricDefs    []MetricDef  `json:"metric_defs"`
	Goals         []Goal       `json:"goals"`
	Tasks         []Task       `json:"tasks"`
	Checkpoints   []Checkpoint `json:"checkpoints"`
	Skills        []Skill      `json:"skills"`
}

// Plan identifies the one plan this database may hold and plan-level behavior settings.
type Plan struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ActiveGoalLimit int    `json:"active_goal_limit"`
}

// Project is plan vocabulary for goals and tasks. Match key: ID.
type Project struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// Phase is plan vocabulary for windows and optional goal phase labels. Match key: ID.
type Phase struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// SkillTier is plan vocabulary for skill targets. Match key: ID.
type SkillTier struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// Week is a plan window. Match key for the loader: Code.
type Week struct {
	Code      string  `json:"code"`
	Phase     string  `json:"phase"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Focus     *string `json:"focus"`
	SortOrder int     `json:"sort_order"`
}

// Rhythm is one row of the operating-system table. Match key: Label.
type Rhythm struct {
	Label     string `json:"label"`
	Weekdays  string `json:"weekdays"`
	Slot      string `json:"slot"`
	SortOrder int    `json:"sort_order"`
}

// Category is one quick-log button. Match key: ID.
type Category struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
}

// MetricDef is a metric name with its baseline and target as written. Match key: Name.
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

// Goal is a seeded Kanban card, G0..G17. Match key: Code.
type Goal struct {
	Code      string  `json:"code"`
	Title     string  `json:"title"`
	DoneMeans *string `json:"done_means"`
	Project   string  `json:"project"`
	Phase     *string `json:"phase"`
	Target    *string `json:"target"`
	SortOrder int     `json:"sort_order"`
}

// Task is a seeded checklist item. Match key: SeedKey ("<week>:<slug(title)>").
type Task struct {
	SeedKey   string   `json:"seed_key"`
	Week      string   `json:"week"`
	Title     string   `json:"title"`
	Project   string   `json:"project"`
	Steps     []string `json:"steps"`
	DoneMeans *string  `json:"done_means"`
	SortOrder int      `json:"sort_order"`
	// Skills is the list of skill codes this task builds. Never empty: every task builds at
	// least one skill, and the generator refuses to emit a document that breaks that.
	Skills []string `json:"skills"`
}

// Skill is a named capability the work builds. Match key: Code.
type Skill struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	AssociateWhen string  `json:"associate_when"`
	TargetTier    *string `json:"target_tier"`
	SortOrder     int     `json:"sort_order"`
}

// Checkpoint is the five-question review at W12 and B7. Match key: Week.
type Checkpoint struct {
	Week      string   `json:"week"`
	Questions []string `json:"questions"`
	Helpers   []string `json:"helpers"`
}

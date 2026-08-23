package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FernasFragas/dashboard-app/internal/seed"
)

// Profile is the bounded parsing configuration. It is deliberately not a parser generator:
// section names, marker labels and the two document-shaped regexes are configurable.
type Profile struct {
	Plan        Settings
	Sections    SectionProfile
	Markers     MarkerProfile
	WeekHeading WeekHeadingProfile
	TaskBullet  TaskBulletProfile
}

// Settings configures plan identity and plan-level behavior.
type Settings struct {
	ID              string
	Name            string
	StartYear       int
	ActiveGoalLimit int
}

// SectionProfile maps markdown section headings to parser roles.
type SectionProfile struct {
	Projects    string
	Goals       string
	Skills      string
	Rhythm      string
	Categories  string
	Metrics     string
	Checkpoints string
}

// MarkerProfile configures task-detail marker labels.
type MarkerProfile struct {
	Done   string
	Skills string
}

// WeekHeadingProfile configures how week headings are recognized.
type WeekHeadingProfile struct {
	Pattern    string
	DateFormat string
}

// TaskBulletProfile configures how task bullets and project tags are recognized.
type TaskBulletProfile struct {
	Pattern    string
	TagPattern string
}

// DefaultProfile matches master-plan-v5.md's shape.
func DefaultProfile() Profile {
	return Profile{
		Plan: Settings{
			ID:              "master-plan-v5",
			Name:            "Master Plan v5",
			StartYear:       2026,
			ActiveGoalLimit: 3,
		},
		Sections: SectionProfile{
			Projects:    "Projects",
			Goals:       "Goals",
			Skills:      "Skills",
			Rhythm:      "Operating system",
			Categories:  "Log categories",
			Metrics:     "Metrics targets",
			Checkpoints: "Checkpoint questions",
		},
		Markers: MarkerProfile{
			Done:   "Done",
			Skills: "Skills",
		},
		WeekHeading: WeekHeadingProfile{
			Pattern:    `^##\s+(?P<code>[WB]\d+)\s*·\s*(?P<dates>.+?)(?:\s+—\s+(?P<focus>.*)|\s+\((?P<focus_paren>[^)]*)\))?$`,
			DateFormat: "Mon D-D",
		},
		TaskBullet: TaskBulletProfile{
			Pattern:    `^-\s+\[[ x]\]\s+\*\*(?P<bold>.+?)\*\*(?P<trailing>.*)$`,
			TagPattern: `^\[([^\]]+)\]\s*(.*)$`,
		},
	}
}

func (p Profile) withDefaults() Profile {
	defaults := DefaultProfile()

	if p.Plan.ID == "" {
		p.Plan.ID = defaults.Plan.ID
	}
	if p.Plan.Name == "" {
		p.Plan.Name = defaults.Plan.Name
	}
	if p.Plan.StartYear == 0 {
		p.Plan.StartYear = defaults.Plan.StartYear
	}
	if p.Plan.ActiveGoalLimit == 0 {
		p.Plan.ActiveGoalLimit = defaults.Plan.ActiveGoalLimit
	}

	if p.Sections.Projects == "" {
		p.Sections.Projects = defaults.Sections.Projects
	}
	if p.Sections.Goals == "" {
		p.Sections.Goals = defaults.Sections.Goals
	}
	if p.Sections.Skills == "" {
		p.Sections.Skills = defaults.Sections.Skills
	}
	if p.Sections.Rhythm == "" {
		p.Sections.Rhythm = defaults.Sections.Rhythm
	}
	if p.Sections.Categories == "" {
		p.Sections.Categories = defaults.Sections.Categories
	}
	if p.Sections.Metrics == "" {
		p.Sections.Metrics = defaults.Sections.Metrics
	}
	if p.Sections.Checkpoints == "" {
		p.Sections.Checkpoints = defaults.Sections.Checkpoints
	}

	if p.Markers.Done == "" {
		p.Markers.Done = defaults.Markers.Done
	}
	if p.Markers.Skills == "" {
		p.Markers.Skills = defaults.Markers.Skills
	}

	if p.WeekHeading.Pattern == "" {
		p.WeekHeading.Pattern = defaults.WeekHeading.Pattern
	}
	if p.WeekHeading.DateFormat == "" {
		p.WeekHeading.DateFormat = defaults.WeekHeading.DateFormat
	}
	if p.TaskBullet.Pattern == "" {
		p.TaskBullet.Pattern = defaults.TaskBullet.Pattern
	}
	if p.TaskBullet.TagPattern == "" {
		p.TaskBullet.TagPattern = defaults.TaskBullet.TagPattern
	}

	return p
}

// ParseFile reads a plan document and an optional plan.yaml beside it.
func ParseFile(path string) (seed.Document, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return seed.Document{}, fmt.Errorf("read plan %s: %w", path, err)
	}

	profile, err := LoadProfileForPlan(path)
	if err != nil {
		return seed.Document{}, err
	}

	doc, err := ParseWithProfile(string(source), profile)
	if err != nil {
		return seed.Document{}, err
	}
	doc.GeneratedFrom = filepath.Base(path)

	return doc, nil
}

// LoadProfileForPlan reads plan.yaml beside a plan document if present; absence means defaults.
func LoadProfileForPlan(planPath string) (Profile, error) {
	profile := DefaultProfile()
	configPath := filepath.Join(filepath.Dir(planPath), "plan.yaml")

	body, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return profile, nil
		}
		return Profile{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	if err := applyYAMLProfile(&profile, string(body)); err != nil {
		return Profile{}, fmt.Errorf("%s: %w", configPath, err)
	}

	return profile.withDefaults(), nil
}

func applyFrontMatter(source string, profile Profile) (string, Profile, error) {
	lines := strings.Split(source, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return source, profile, nil
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "---" {
			continue
		}

		frontMatter := strings.Join(lines[1:i], "\n")
		if err := applyYAMLProfile(&profile, frontMatter); err != nil {
			return "", Profile{}, fmt.Errorf("front matter: %w", err)
		}

		for j := 0; j <= i; j++ {
			lines[j] = ""
		}

		return strings.Join(lines, "\n"), profile, nil
	}

	return "", Profile{}, fmt.Errorf("line 1: front matter missing closing ---")
}

func applyYAMLProfile(profile *Profile, source string) error {
	section := ""

	for i, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}

		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			return fmt.Errorf("line %d: expected key: value", i+1)
		}
		key = strings.TrimSpace(key)
		value = cleanYAMLScalar(value)

		switch section + "." + key {
		case "plan.id":
			profile.Plan.ID = value
		case "plan.name":
			profile.Plan.Name = value
		case "plan.start_year":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("line %d: bad start_year %q", i+1, value)
			}
			profile.Plan.StartYear = n
		case "plan.active_goal_limit":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("line %d: bad active_goal_limit %q", i+1, value)
			}
			profile.Plan.ActiveGoalLimit = n
		case "sections.projects":
			profile.Sections.Projects = value
		case "sections.goals":
			profile.Sections.Goals = value
		case "sections.skills":
			profile.Sections.Skills = value
		case "sections.rhythm":
			profile.Sections.Rhythm = value
		case "sections.categories":
			profile.Sections.Categories = value
		case "sections.metrics":
			profile.Sections.Metrics = value
		case "sections.checkpoints":
			profile.Sections.Checkpoints = value
		case "markers.done":
			profile.Markers.Done = value
		case "markers.skills":
			profile.Markers.Skills = value
		case "week_heading.pattern":
			profile.WeekHeading.Pattern = value
		case "week_heading.date_format":
			profile.WeekHeading.DateFormat = value
		case "task_bullet.pattern":
			profile.TaskBullet.Pattern = value
		case "task_bullet.tag_pattern":
			profile.TaskBullet.TagPattern = value
		default:
			return fmt.Errorf("line %d: unknown key %s.%s", i+1, section, key)
		}
	}

	return nil
}

func cleanYAMLScalar(raw string) string {
	value := strings.TrimSpace(raw)
	if cut, _, found := strings.Cut(value, " #"); found {
		value = strings.TrimSpace(cut)
	}
	if len(value) >= 2 {
		quote := value[0]
		if quote == value[len(value)-1] && (quote == '\'' || quote == '"') {
			value = value[1 : len(value)-1]
		}
	}
	return value
}

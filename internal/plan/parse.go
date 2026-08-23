// Package plan parses a portable markdown plan into the seed document loaded at boot.
package plan

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/seed"
)

// The parser is deliberately strict inside week sections: an unrecognised line is an error,
// never a silent skip. Silent skips are how a week of tasks disappears unnoticed.

var (
	phaseHeadingRe = regexp.MustCompile(`^#\s+Phase\s+(\d)\b`)
	sectionRe      = regexp.MustCompile(`^##\s+(.*)$`)
	stepRe         = regexp.MustCompile(`^\s{2,}(\d+)\.\s+(.*)$`)
	tableRowRe     = regexp.MustCompile(`^\|(.+)\|\s*$`)
	slugRe         = regexp.MustCompile(`[^a-z0-9]+`)

	// Lines that carry no data: separators, italic asides, block quotes.
	noiseRe = regexp.MustCompile(`^(---+|\*[^*].*\*|>.*)$`)
)

var months = map[string]int{
	"Jan": 1, "Feb": 2, "Mar": 3, "Apr": 4, "May": 5, "Jun": 6,
	"Jul": 7, "Aug": 8, "Sep": 9, "Oct": 10, "Nov": 11, "Dec": 12,
}

var weekdayNumbers = map[string]int{
	"Mon": 1, "Tue": 2, "Wed": 3, "Thu": 4, "Fri": 5, "Sat": 6, "Sun": 7,
}

// Parse turns markdown into a seed document using the default profile and the supplied start year.
func Parse(source string, startYear int) (seed.Document, error) {
	profile := DefaultProfile()
	profile.Plan.StartYear = startYear
	return ParseWithProfile(source, profile)
}

// ParseWithProfile turns markdown into a seed document using a bounded parsing profile.
func ParseWithProfile(source string, profile Profile) (seed.Document, error) {
	var err error
	source, profile, err = applyFrontMatter(source, profile)
	if err != nil {
		return seed.Document{}, err
	}
	profile = profile.withDefaults()

	weekHeadingRe, err := regexp.Compile(profile.WeekHeading.Pattern)
	if err != nil {
		return seed.Document{}, fmt.Errorf("compile week_heading.pattern: %w", err)
	}
	taskRe, err := regexp.Compile(profile.TaskBullet.Pattern)
	if err != nil {
		return seed.Document{}, fmt.Errorf("compile task_bullet.pattern: %w", err)
	}

	p := &parser{
		lines:         strings.Split(source, "\n"),
		doc:           seed.Document{GeneratedFrom: profile.Plan.ID},
		profile:       profile,
		weekHeadingRe: weekHeadingRe,
		taskRe:        taskRe,
		doneRe: regexp.MustCompile(
			`^\s*→\s*\*\*` + regexp.QuoteMeta(profile.Markers.Done) + `\s*=\*\*\s*(.*)$`,
		),
		skillsRe: regexp.MustCompile(
			`^\s*→\s*\*\*` + regexp.QuoteMeta(profile.Markers.Skills) + `\s*=\*\*\s*(.*)$`,
		),
		tagRe:     regexp.MustCompile(profile.TaskBullet.TagPattern),
		year:      profile.Plan.StartYear,
		phase:     "P1",
		lastMonth: 0,
	}

	p.doc.Plan = seed.Plan{
		ID:              profile.Plan.ID,
		Name:            profile.Plan.Name,
		ActiveGoalLimit: profile.Plan.ActiveGoalLimit,
	}

	if err := p.run(); err != nil {
		return seed.Document{}, err
	}

	return p.doc, nil
}

type parser struct {
	lines   []string
	doc     seed.Document
	profile Profile

	weekHeadingRe *regexp.Regexp
	taskRe        *regexp.Regexp
	tagRe         *regexp.Regexp
	doneRe        *regexp.Regexp
	skillsRe      *regexp.Regexp

	// year and lastMonth roll the calendar forward across a Dec/Jan boundary.
	year      int
	lastMonth int

	phase string

	seenProjectOrder []string
	seenProjects     map[string]struct{}
	weekLines        map[string]int
	taskLines        map[string]int
	checkpointWeeks  []string
	checkQuestions   []string
	checkHelpers     []string
}

func (p *parser) run() error {
	section := ""

	for i := 0; i < len(p.lines); i++ {
		line := p.lines[i]

		if m := phaseHeadingRe.FindStringSubmatch(line); m != nil {
			p.phase = "P" + m[1]
			section = ""
			continue
		}

		if m := p.weekHeadingRe.FindStringSubmatch(line); m != nil {
			consumed, err := p.parseWeek(m, i)
			if err != nil {
				return err
			}
			i = consumed
			section = ""
			continue
		}

		if m := sectionRe.FindStringSubmatch(line); m != nil {
			section = strings.TrimSpace(m[1])
			continue
		}

		var err error
		switch {
		case p.sectionIs(section, p.profile.Sections.Projects):
			err = p.parseProjectRow(line)
		case p.sectionIs(section, p.profile.Sections.Categories):
			err = p.parseCategoryRow(line)
		case p.sectionIs(section, p.profile.Sections.Goals):
			err = p.parseGoalRow(line)
		case p.sectionIs(section, p.profile.Sections.Rhythm):
			err = p.parseRhythmRow(line)
		case p.sectionIs(section, p.profile.Sections.Metrics):
			err = p.parseMetricRow(line)
		case p.sectionIs(section, p.profile.Sections.Skills):
			err = p.parseSkillRow(line)
		case p.sectionIs(section, p.profile.Sections.Checkpoints):
			err = p.parseCheckpointRow(line)
		}

		if err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}

	return p.finalise()
}

func (p *parser) sectionIs(actual string, configured string) bool {
	return configured != "" && strings.HasPrefix(actual, configured)
}

// parseWeek consumes a week heading and every task under it, returning the index of the last
// consumed line.
func (p *parser) parseWeek(m []string, start int) (int, error) {
	code, ok := namedMatch(p.weekHeadingRe, m, "code")
	if !ok {
		code = m[1]
	}
	dates, ok := namedMatch(p.weekHeadingRe, m, "dates")
	if !ok {
		return 0, fmt.Errorf("line %d: week heading has no dates capture", start+1)
	}

	startDate, endDate, err := p.parseDateRange(dates)
	if err != nil {
		return 0, fmt.Errorf("line %d: week %s dates: %w", start+1, code, err)
	}

	focusText, _ := namedMatch(p.weekHeadingRe, m, "focus")
	if focusText == "" {
		focusText, _ = namedMatch(p.weekHeadingRe, m, "focus_paren")
	}
	focusText = strings.TrimSpace(focusText)

	var focus *string
	if focusText != "" {
		focus = &focusText
	}

	p.doc.Weeks = append(p.doc.Weeks, seed.Week{
		Code:      code,
		Phase:     p.phase,
		StartDate: startDate,
		EndDate:   endDate,
		Focus:     focus,
		SortOrder: len(p.doc.Weeks) + 1,
	})
	if p.weekLines == nil {
		p.weekLines = map[string]int{}
	}
	p.weekLines[code] = start + 1

	i := start + 1
	order := 0

	for ; i < len(p.lines); i++ {
		line := p.lines[i]

		if strings.HasPrefix(line, "#") {
			return i - 1, nil
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || noiseRe.MatchString(trimmed) {
			continue
		}

		m := p.taskRe.FindStringSubmatch(line)
		if m == nil {
			return 0, fmt.Errorf("line %d: unrecognised line in week %s: %q", i+1, code, line)
		}

		task, consumed, err := p.parseTask(code, m, i)
		if err != nil {
			return 0, err
		}

		order++
		task.SortOrder = order * 100
		p.doc.Tasks = append(p.doc.Tasks, task)
		if p.taskLines == nil {
			p.taskLines = map[string]int{}
		}
		p.taskLines[task.SeedKey] = i + 1

		i = consumed
	}

	return i - 1, nil
}

func (p *parser) parseDateRange(raw string) (string, string, error) {
	m := regexp.MustCompile(
		`^\s*([A-Z][a-z]{2})\s+(\d{1,2})\s*[–-]\s*(?:([A-Z][a-z]{2})\s+)?(\d{1,2})`,
	).FindStringSubmatch(raw)
	if m == nil {
		return "", "", fmt.Errorf("unsupported date range %q", raw)
	}

	startDate, err := p.nextDate(m[1], m[2])
	if err != nil {
		return "", "", err
	}

	endMonth := m[3]
	if endMonth == "" {
		endMonth = m[1]
	}
	endDate, err := p.nextDate(endMonth, m[4])
	if err != nil {
		return "", "", err
	}

	return startDate, endDate, nil
}

// parseTask reads one checklist bullet plus its indented details.
func (p *parser) parseTask(week string, m []string, start int) (seed.Task, int, error) {
	bold, ok := namedMatch(p.taskRe, m, "bold")
	if !ok {
		bold = m[1]
	}
	trailing, _ := namedMatch(p.taskRe, m, "trailing")
	if trailing == "" && len(m) > 2 {
		trailing = m[len(m)-1]
	}
	trailing = strings.TrimSpace(trailing)

	tag := p.tagRe.FindStringSubmatch(bold)
	if tag == nil {
		return seed.Task{}, 0, fmt.Errorf("line %d: task has no [project] tag: %q", start+1, bold)
	}

	project, err := p.normaliseProject(tag[1])
	if err != nil {
		return seed.Task{}, 0, fmt.Errorf("line %d: %w", start+1, err)
	}

	title := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(tag[2]), ":"))

	var steps []string
	var doneMeans *string

	switch {
	case title == "":
		if before, done, found := cutInlineMarker(trailing, p.profile.Markers.Done); found {
			title = strings.TrimSpace(strings.TrimSuffix(before, "."))
			text := strings.TrimSpace(done)
			doneMeans = &text
		} else {
			title = strings.TrimSpace(strings.TrimSuffix(trailing, "."))
		}
	case trailing != "":
		if before, done, found := cutInlineMarker(trailing, p.profile.Markers.Done); found {
			if before != "" {
				steps = append(steps, before)
			}
			text := strings.TrimSpace(done)
			doneMeans = &text
		} else {
			steps = append(steps, trailing)
		}
	}

	if title == "" {
		return seed.Task{}, 0, fmt.Errorf("line %d: task has no title", start+1)
	}

	var skills []string
	i := start + 1
	for ; i < len(p.lines); i++ {
		line := p.lines[i]

		if strings.HasPrefix(line, "#") {
			break
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if p.taskRe.MatchString(line) || noiseRe.MatchString(trimmed) || !strings.HasPrefix(line, " ") {
			break
		}

		if sm := stepRe.FindStringSubmatch(line); sm != nil {
			steps = append(steps, strings.TrimSpace(sm[2]))
			continue
		}

		if dm := p.doneRe.FindStringSubmatch(line); dm != nil {
			text := strings.TrimSpace(dm[1])
			doneMeans = &text
			continue
		}

		if sk := p.skillsRe.FindStringSubmatch(line); sk != nil {
			skills = splitCodes(sk[1])
			continue
		}

		if len(steps) > 0 {
			steps[len(steps)-1] += " " + trimmed
			continue
		}

		return seed.Task{}, 0, fmt.Errorf("line %d: unrecognised task detail: %q", i+1, line)
	}

	if steps == nil {
		steps = []string{}
	}

	if strings.Contains(strings.ToLower(title), "checkpoint") {
		p.checkpointWeeks = append(p.checkpointWeeks, week)
	}

	return seed.Task{
		SeedKey:   week + ":" + slug(title),
		Week:      week,
		Title:     title,
		Project:   project,
		Steps:     steps,
		DoneMeans: doneMeans,
		Skills:    skills,
	}, i - 1, nil
}

func (p *parser) parseProjectRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || tableDivider(cells) || tableHeader(cells, "id") {
		return nil
	}
	if len(cells) < 2 {
		return fmt.Errorf("project row needs id and label")
	}

	id := strings.TrimSpace(cells[0])
	label := strings.TrimSpace(cells[1])
	if id == "" || label == "" {
		return fmt.Errorf("project row needs id and label")
	}

	p.doc.Projects = append(p.doc.Projects, seed.Project{
		ID:        id,
		Label:     label,
		SortOrder: (len(p.doc.Projects) + 1) * 10,
	})
	p.rememberProject(id)

	return nil
}

func (p *parser) parseCategoryRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || tableDivider(cells) || tableHeader(cells, "id") {
		return nil
	}
	if len(cells) < 3 {
		return fmt.Errorf("category row needs id, label and icon")
	}

	id := strings.TrimSpace(cells[0])
	label := strings.TrimSpace(cells[1])
	icon := strings.TrimSpace(cells[2])
	if id == "" || label == "" || icon == "" {
		return fmt.Errorf("category row needs id, label and icon")
	}

	p.doc.Categories = append(p.doc.Categories, seed.Category{
		ID:        id,
		Label:     label,
		Icon:      icon,
		SortOrder: len(p.doc.Categories) + 1,
	})

	return nil
}

func (p *parser) parseGoalRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || len(cells) < 5 || tableHeader(cells, "ID") || tableDivider(cells) {
		return nil
	}

	if !regexp.MustCompile(`^G\d+$`).MatchString(cells[0]) {
		return nil
	}

	project, err := p.normaliseProject(cells[2])
	if err != nil {
		return fmt.Errorf("goal %s: %w", cells[0], err)
	}

	phase := derivePhase(cells[4])

	p.doc.Goals = append(p.doc.Goals, seed.Goal{
		Code:      cells[0],
		Title:     cells[1],
		DoneMeans: optional(cells[3]),
		Project:   project,
		Phase:     &phase,
		Target:    optional(cells[4]),
		SortOrder: (len(p.doc.Goals) + 1) * 100,
	})

	return nil
}

func (p *parser) parseRhythmRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || len(cells) < 2 || tableHeader(cells, "Day") || tableDivider(cells) {
		return nil
	}

	weekdays, err := parseWeekdays(cells[0])
	if err != nil {
		return err
	}

	p.doc.Rhythm = append(p.doc.Rhythm, seed.Rhythm{
		Label:     cells[0],
		Weekdays:  weekdays,
		Slot:      cells[1],
		SortOrder: len(p.doc.Rhythm) + 1,
	})

	return nil
}

func (p *parser) parseMetricRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || tableDivider(cells) || tableHeader(cells, "Metric") {
		return nil
	}
	if len(cells) < 3 {
		return fmt.Errorf("metric row needs metric, baseline and target")
	}

	metric := seed.MetricDef{
		Name:      cells[0],
		Baseline:  optional(cells[1]),
		Target:    optional(cells[2]),
		SortOrder: len(p.doc.MetricDefs) + 1,
	}
	if len(cells) > 3 {
		metric.Slug = optional(cells[3])
	}
	if len(cells) > 4 {
		metric.Unit = optional(cells[4])
	}
	if len(cells) > 5 {
		metric.Definition = optional(cells[5])
	}
	if len(cells) > 6 {
		metric.HowToMeasure = optional(cells[6])
	}

	if metric.Definition == nil || metric.HowToMeasure == nil {
		return fmt.Errorf("metric %q needs definition and how to measure", metric.Name)
	}

	p.doc.MetricDefs = append(p.doc.MetricDefs, metric)

	return nil
}

func (p *parser) parseSkillRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || tableDivider(cells) || tableHeader(cells, "code") {
		return nil
	}
	if len(cells) < 5 {
		return fmt.Errorf("skill row needs code, name, description, associate when and target")
	}

	code := strings.TrimSpace(cells[0])
	name := strings.TrimSpace(cells[1])
	description := strings.TrimSpace(cells[2])
	associateWhen := strings.TrimSpace(cells[3])
	if code == "" || name == "" || description == "" || associateWhen == "" {
		return fmt.Errorf("skill row has an empty required field")
	}

	p.doc.Skills = append(p.doc.Skills, seed.Skill{
		Code:          code,
		Name:          name,
		Description:   description,
		AssociateWhen: associateWhen,
		TargetTier:    optional(cells[4]),
		SortOrder:     (len(p.doc.Skills) + 1) * 10,
	})

	return nil
}

func (p *parser) parseCheckpointRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || tableDivider(cells) || tableHeader(cells, "question") {
		return nil
	}
	if len(cells) < 2 {
		return fmt.Errorf("checkpoint row needs question and helper")
	}

	question := strings.TrimSpace(cells[0])
	helper := strings.TrimSpace(cells[1])
	if question == "" || helper == "" {
		return fmt.Errorf("checkpoint row needs question and helper")
	}
	if !strings.HasSuffix(question, "?") {
		question += "?"
	}

	p.checkQuestions = append(p.checkQuestions, question)
	p.checkHelpers = append(p.checkHelpers, helper)

	return nil
}

func (p *parser) finalise() error {
	if len(p.doc.Projects) == 0 && len(p.seenProjectOrder) > 0 {
		for i, id := range p.seenProjectOrder {
			p.doc.Projects = append(p.doc.Projects, seed.Project{
				ID:        id,
				Label:     id,
				SortOrder: (i + 1) * 10,
			})
		}
	}

	if err := p.deriveReferenceVocabulary(); err != nil {
		return err
	}
	if err := p.deriveCheckpoints(); err != nil {
		return err
	}
	if err := p.validateWeekWindows(); err != nil {
		return err
	}
	if err := p.validateSkills(); err != nil {
		return err
	}

	seen := make(map[string]string, len(p.doc.Tasks))
	for _, t := range p.doc.Tasks {
		if other, dup := seen[t.SeedKey]; dup {
			return fmt.Errorf("duplicate seed_key %q: %q and %q", t.SeedKey, other, t.Title)
		}
		seen[t.SeedKey] = t.Title
	}

	return nil
}

func (p *parser) deriveReferenceVocabulary() error {
	phaseIDs := map[string]struct{}{}
	for _, w := range p.doc.Weeks {
		phaseIDs[w.Phase] = struct{}{}
	}
	for _, g := range p.doc.Goals {
		if g.Phase != nil {
			phaseIDs[*g.Phase] = struct{}{}
		}
	}

	var phases []string
	for id := range phaseIDs {
		phases = append(phases, id)
	}
	slices.Sort(phases)
	for i, id := range phases {
		p.doc.Phases = append(p.doc.Phases, seed.Phase{
			ID:        id,
			Label:     id,
			SortOrder: (i + 1) * 10,
		})
	}

	tierIDs := map[string]struct{}{}
	for _, skill := range p.doc.Skills {
		if skill.TargetTier != nil {
			tierIDs[*skill.TargetTier] = struct{}{}
		}
	}

	var tiers []string
	for id := range tierIDs {
		tiers = append(tiers, id)
	}
	slices.Sort(tiers)
	for i, id := range tiers {
		p.doc.SkillTiers = append(p.doc.SkillTiers, seed.SkillTier{
			ID:        id,
			Label:     id,
			SortOrder: (i + 1) * 10,
		})
	}

	return nil
}

func (p *parser) deriveCheckpoints() error {
	if len(p.checkpointWeeks) == 0 {
		return nil
	}
	if len(p.checkQuestions) == 0 {
		return fmt.Errorf("found %d checkpoint tasks but no checkpoint questions section",
			len(p.checkpointWeeks))
	}
	if len(p.checkQuestions) != len(p.checkHelpers) {
		return fmt.Errorf("checkpoint questions/helpers length mismatch")
	}

	seenWeeks := map[string]struct{}{}
	for _, week := range p.checkpointWeeks {
		if _, seen := seenWeeks[week]; seen {
			continue
		}
		seenWeeks[week] = struct{}{}
		p.doc.Checkpoints = append(p.doc.Checkpoints, seed.Checkpoint{
			Week:      week,
			Questions: append([]string(nil), p.checkQuestions...),
			Helpers:   append([]string(nil), p.checkHelpers...),
		})
	}

	return nil
}

func (p *parser) validateSkills() error {
	known := map[string]struct{}{}
	for _, skill := range p.doc.Skills {
		if _, dup := known[skill.Code]; dup {
			return fmt.Errorf("duplicate skill code %q", skill.Code)
		}
		known[skill.Code] = struct{}{}
	}

	for _, task := range p.doc.Tasks {
		line := p.taskLines[task.SeedKey]
		if len(known) == 0 {
			if len(task.Skills) > 0 {
				return fmt.Errorf("line %d: task %s references skills but no skills section exists",
					line, task.SeedKey)
			}
			continue
		}
		if len(task.Skills) == 0 {
			return fmt.Errorf("line %d: task %s has no skills", line, task.SeedKey)
		}
		for _, code := range task.Skills {
			if _, ok := known[code]; !ok {
				return fmt.Errorf("line %d: task %s references undefined skill %q",
					line, task.SeedKey, code)
			}
		}
	}

	return nil
}

func (p *parser) validateWeekWindows() error {
	type weekWindow struct {
		code  string
		start time.Time
		end   time.Time
		line  int
	}

	windows := make([]weekWindow, 0, len(p.doc.Weeks))
	seen := map[string]int{}
	for _, week := range p.doc.Weeks {
		line := p.weekLines[week.Code]
		if other, dup := seen[week.Code]; dup {
			return fmt.Errorf("line %d: duplicate week code %q, first seen on line %d",
				line, week.Code, other)
		}
		seen[week.Code] = line

		start, err := time.Parse("2006-01-02", week.StartDate)
		if err != nil {
			return fmt.Errorf("line %d: week %s has bad start date %q: %w",
				line, week.Code, week.StartDate, err)
		}
		end, err := time.Parse("2006-01-02", week.EndDate)
		if err != nil {
			return fmt.Errorf("line %d: week %s has bad end date %q: %w",
				line, week.Code, week.EndDate, err)
		}
		if end.Before(start) {
			return fmt.Errorf("line %d: week %s ends before it starts", line, week.Code)
		}

		windows = append(windows, weekWindow{
			code:  week.Code,
			start: start,
			end:   end,
			line:  line,
		})
	}

	slices.SortFunc(windows, func(a, b weekWindow) int {
		return a.start.Compare(b.start)
	})

	for i := 1; i < len(windows); i++ {
		prev := windows[i-1]
		current := windows[i]
		if !current.start.After(prev.end) {
			return fmt.Errorf("line %d: week %s overlaps week %s",
				current.line, current.code, prev.code)
		}
	}

	return nil
}

func (p *parser) nextDate(month, day string) (string, error) {
	monthNum, ok := months[month]
	if !ok {
		return "", fmt.Errorf("unknown month %q", month)
	}

	dayNum, err := strconv.Atoi(day)
	if err != nil {
		return "", fmt.Errorf("bad day %q: %w", day, err)
	}

	if p.lastMonth != 0 && monthNum < p.lastMonth {
		p.year++
	}
	p.lastMonth = monthNum

	return fmt.Sprintf("%04d-%02d-%02d", p.year, monthNum, dayNum), nil
}

func parseWeekdays(label string) (string, error) {
	parts := regexp.MustCompile(`\s*[–-]\s*`).Split(label, -1)

	first, ok := weekdayNumbers[strings.TrimSpace(parts[0])]
	if !ok {
		return "", fmt.Errorf("unknown weekday %q", parts[0])
	}

	if len(parts) == 1 {
		return strconv.Itoa(first), nil
	}

	last, ok := weekdayNumbers[strings.TrimSpace(parts[len(parts)-1])]
	if !ok {
		return "", fmt.Errorf("unknown weekday %q", parts[len(parts)-1])
	}

	if last < first {
		return "", fmt.Errorf("weekday range %q runs backwards", label)
	}

	days := make([]string, 0, last-first+1)
	for d := first; d <= last; d++ {
		days = append(days, strconv.Itoa(d))
	}

	return strings.Join(days, ","), nil
}

func (p *parser) normaliseProject(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))

	if idx := strings.Index(value, "("); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}
	if idx := strings.Index(value, "+"); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}
	if strings.HasPrefix(value, "all") {
		value = "all"
	}
	if value == "" {
		return "", fmt.Errorf("unknown project tag %q", raw)
	}

	if len(p.doc.Projects) > 0 {
		for _, project := range p.doc.Projects {
			if project.ID == value {
				p.rememberProject(value)
				return value, nil
			}
		}
		return "", fmt.Errorf("unknown project tag %q", raw)
	}

	p.rememberProject(value)
	return value, nil
}

func (p *parser) rememberProject(id string) {
	if p.seenProjects == nil {
		p.seenProjects = map[string]struct{}{}
	}
	if _, ok := p.seenProjects[id]; ok {
		return
	}
	p.seenProjects[id] = struct{}{}
	p.seenProjectOrder = append(p.seenProjectOrder, id)
}

func derivePhase(target string) string {
	t := strings.TrimSpace(target)

	switch {
	case strings.HasPrefix(t, "Q"):
		return "P3"
	case strings.HasPrefix(t, "P2"), strings.HasPrefix(t, "B"):
		return "P2"
	default:
		return "P1"
	}
}

func namedMatch(re *regexp.Regexp, matches []string, name string) (string, bool) {
	index := re.SubexpIndex(name)
	if index < 0 || index >= len(matches) {
		return "", false
	}
	return matches[index], true
}

func tableCells(line string) ([]string, bool) {
	m := tableRowRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil, false
	}

	parts := strings.Split(m[1], "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}

	return cells, true
}

func tableDivider(cells []string) bool {
	return len(cells) > 0 && strings.HasPrefix(cells[0], "---")
}

func tableHeader(cells []string, first string) bool {
	return len(cells) > 0 && strings.EqualFold(strings.TrimSpace(cells[0]), first)
}

func optional(value string) *string {
	v := strings.TrimSpace(value)
	if v == "" || v == "—" || v == "-" {
		return nil
	}

	return &v
}

func splitCodes(raw string) []string {
	parts := regexp.MustCompile(`\s*(?:,|·)\s*`).Split(raw, -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		code := strings.TrimSpace(part)
		if code != "" {
			out = append(out, code)
		}
	}
	return out
}

func cutInlineMarker(text string, marker string) (before, after string, ok bool) {
	token := "**" + marker + " =**"
	idx := strings.Index(text, token)
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(text[:idx]), strings.TrimSpace(text[idx+len(token):]), true
}

func slug(value string) string {
	s := strings.ToLower(value)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "task"
	}
	return s
}

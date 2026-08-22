package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/FernasFragas/dashboard-app/internal/seed"
)

// The parser is deliberately strict: any line inside a plan week that matches none of the
// known shapes is a hard error, never a silent skip. A silent skip is how a week of tasks
// disappears without anyone noticing.

var (
	phaseHeadingRe = regexp.MustCompile(`^#\s+Phase\s+(\d)\b`)
	sectionRe      = regexp.MustCompile(`^##\s+(.*)$`)

	// "## W1 · Aug 24–30 — Golden set", "## B4 · Dec 28–Jan 10 (holiday-light)"
	weekHeadingRe = regexp.MustCompile(
		`^##\s+([WB]\d+)\s*·\s*([A-Z][a-z]{2})\s+(\d{1,2})\s*[–-]\s*(?:([A-Z][a-z]{2})\s+)?(\d{1,2})` +
			`(?:\s*\(([^)]*)\))?(?:\s*[—-]+\s*(.*))?$`)

	// "- [ ] **[synapse] Golden set committed**" and its two variants.
	taskRe = regexp.MustCompile(`^-\s+\[[ x]\]\s+\*\*(.+?)\*\*(.*)$`)
	tagRe  = regexp.MustCompile(`^\[([^\]]+)\]\s*(.*)$`)

	stepRe = regexp.MustCompile(`^\s{2,}(\d+)\.\s+(.*)$`)
	doneRe = regexp.MustCompile(`^\s*→\s*\*\*Done\s*=\*\*\s*(.*)$`)

	// Lines that carry no data: separators, italic asides, block quotes.
	noiseRe = regexp.MustCompile(`^(---+|\*[^*].*\*|>.*)$`)

	tableRowRe = regexp.MustCompile(`^\|(.+)\|\s*$`)
	slugRe     = regexp.MustCompile(`[^a-z0-9]+`)
)

// categorySlugs are the eight fixed ids, in the order the plan lists the buttons. The plan
// gives labels and emoji only; the slugs are the schema's CHECK set.
var categorySlugs = []string{
	"application", "module", "portfolio", "post", "oss", "number", "network", "exam",
}

var months = map[string]int{
	"Jan": 1, "Feb": 2, "Mar": 3, "Apr": 4, "May": 5, "Jun": 6,
	"Jul": 7, "Aug": 8, "Sep": 9, "Oct": 10, "Nov": 11, "Dec": 12,
}

var weekdayNumbers = map[string]int{
	"Mon": 1, "Tue": 2, "Wed": 3, "Thu": 4, "Fri": 5, "Sat": 6, "Sun": 7,
}

var validProjects = map[string]bool{
	"synapse": true, "gateway": true, "dash": true, "oss": true,
	"learn": true, "write": true, "career": true, "all": true,
}

// Parse turns master-plan-v5.md into a seed document. startYear is the calendar year W1 falls
// in; later windows roll into the next year when a month goes backwards.
func Parse(source string, startYear int) (seed.Document, error) {
	p := &parser{
		lines:     strings.Split(source, "\n"),
		year:      startYear,
		doc:       seed.Document{GeneratedFrom: "master-plan-v5.md"},
		phase:     "P1",
		lastMonth: 0,
	}

	if err := p.run(); err != nil {
		return seed.Document{}, err
	}

	return p.doc, nil
}

type parser struct {
	lines []string
	doc   seed.Document

	// year and lastMonth roll the calendar forward across the Dec/Jan boundary.
	year      int
	lastMonth int

	phase string
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

		if m := weekHeadingRe.FindStringSubmatch(line); m != nil {
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
		case strings.HasPrefix(section, "Log categories"):
			err = p.parseCategories(line)
		case strings.HasPrefix(section, "Goals"):
			err = p.parseGoalRow(line)
		case strings.HasPrefix(section, "Operating system"):
			err = p.parseRhythmRow(line)
		case strings.HasPrefix(section, "Metrics targets"):
			err = p.parseMetricRow(line)
		}

		if err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}

	return p.finalise()
}

// parseWeek consumes a week heading and every task under it, returning the index of the last
// line it consumed.
func (p *parser) parseWeek(m []string, start int) (int, error) {
	code := m[1]

	startDate, err := p.nextDate(m[2], m[3])
	if err != nil {
		return 0, fmt.Errorf("line %d: week %s start date: %w", start+1, code, err)
	}

	endMonth := m[4]
	if endMonth == "" {
		endMonth = m[2]
	}

	endDate, err := p.nextDate(endMonth, m[5])
	if err != nil {
		return 0, fmt.Errorf("line %d: week %s end date: %w", start+1, code, err)
	}

	// "— Golden set" wins; otherwise a bare parenthetical like "(holiday-light)" is the label.
	focusText := strings.TrimSpace(m[7])
	if focusText == "" {
		focusText = strings.TrimSpace(m[6])
	}

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

	i := start + 1
	order := 0

	for ; i < len(p.lines); i++ {
		line := p.lines[i]

		// A new heading of any level ends the week.
		if strings.HasPrefix(line, "#") {
			return i - 1, nil
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || noiseRe.MatchString(trimmed) {
			continue
		}

		m := taskRe.FindStringSubmatch(line)
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

		i = consumed
	}

	return i - 1, nil
}

// parseTask reads one checklist bullet plus its numbered steps and Done line.
func (p *parser) parseTask(week string, m []string, start int) (seed.Task, int, error) {
	bold := m[1]
	trailing := strings.TrimSpace(m[2])

	tag := tagRe.FindStringSubmatch(bold)
	if tag == nil {
		return seed.Task{}, 0, fmt.Errorf("line %d: task has no [project] tag: %q", start+1, bold)
	}

	project, err := normaliseProject(tag[1])
	if err != nil {
		return seed.Task{}, 0, fmt.Errorf("line %d: %w", start+1, err)
	}

	title := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(tag[2]), ":"))

	var steps []string

	switch {
	case title == "":
		// "- [ ] **[career]** 10 more applications" - the title lives outside the bold.
		title = strings.TrimSpace(strings.TrimSuffix(trailing, "."))
	case trailing != "":
		// "- [ ] **[dash] Checkpoint Feb 21:** written review" - the trailing text is detail.
		steps = append(steps, trailing)
	}

	if title == "" {
		return seed.Task{}, 0, fmt.Errorf("line %d: task has no title", start+1)
	}

	var doneMeans *string

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

		// A new bullet or a non-indented line ends this task.
		if taskRe.MatchString(line) || noiseRe.MatchString(trimmed) || !strings.HasPrefix(line, " ") {
			break
		}

		if sm := stepRe.FindStringSubmatch(line); sm != nil {
			steps = append(steps, strings.TrimSpace(sm[2]))
			continue
		}

		if dm := doneRe.FindStringSubmatch(line); dm != nil {
			text := strings.TrimSpace(dm[1])
			doneMeans = &text
			continue
		}

		// An indented continuation of the previous step.
		if len(steps) > 0 {
			steps[len(steps)-1] += " " + trimmed
			continue
		}

		return seed.Task{}, 0, fmt.Errorf("line %d: unrecognised task detail: %q", i+1, line)
	}

	if steps == nil {
		steps = []string{}
	}

	return seed.Task{
		SeedKey:   week + ":" + slug(title),
		Week:      week,
		Title:     title,
		Project:   project,
		Steps:     steps,
		DoneMeans: doneMeans,
	}, i - 1, nil
}

// parseCategories reads the "Application 📮 · Course module 📚 · ..." line.
func (p *parser) parseCategories(line string) error {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || p.doc.Categories != nil {
		return nil
	}

	parts := strings.Split(trimmed, "·")
	if len(parts) != len(categorySlugs) {
		return fmt.Errorf("expected %d log categories, found %d", len(categorySlugs), len(parts))
	}

	for i, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) < 2 {
			return fmt.Errorf("category %q has no label and icon", part)
		}

		icon := fields[len(fields)-1]
		label := strings.Join(fields[:len(fields)-1], " ")

		p.doc.Categories = append(p.doc.Categories, seed.Category{
			ID:        categorySlugs[i],
			Label:     label,
			Icon:      icon,
			SortOrder: i + 1,
		})
	}

	return nil
}

// parseGoalRow reads one row of the Goals (Kanban seed) table.
func (p *parser) parseGoalRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || len(cells) != 5 || cells[0] == "ID" || strings.HasPrefix(cells[0], "---") {
		return nil
	}

	if !regexp.MustCompile(`^G\d+$`).MatchString(cells[0]) {
		return nil
	}

	project, err := normaliseProject(cells[2])
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

// parseRhythmRow reads one row of the operating-system table.
func (p *parser) parseRhythmRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || len(cells) != 2 || cells[0] == "Day" || strings.HasPrefix(cells[0], "---") {
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

// parseMetricRow reads one row of the Metrics targets table.
func (p *parser) parseMetricRow(line string) error {
	cells, ok := tableCells(line)
	if !ok || len(cells) != 3 || cells[0] == "Metric" || strings.HasPrefix(cells[0], "---") {
		return nil
	}

	p.doc.MetricDefs = append(p.doc.MetricDefs, seed.MetricDef{
		Name:      cells[0],
		Unit:      nil, // the plan states no units; the Review screen shows baseline/target instead
		Baseline:  optional(cells[1]),
		Target:    optional(cells[2]),
		SortOrder: len(p.doc.MetricDefs) + 1,
	})

	return nil
}

// finalise derives the checkpoints and asserts the document is complete enough to seed with.
func (p *parser) finalise() error {
	if err := p.deriveCheckpoints(); err != nil {
		return err
	}

	switch {
	case len(p.doc.Categories) != len(categorySlugs):
		return fmt.Errorf("parsed %d categories, want %d", len(p.doc.Categories), len(categorySlugs))
	case len(p.doc.Goals) == 0:
		return fmt.Errorf("parsed no goals")
	case len(p.doc.Weeks) == 0:
		return fmt.Errorf("parsed no weeks")
	case len(p.doc.Tasks) == 0:
		return fmt.Errorf("parsed no tasks")
	case len(p.doc.Rhythm) == 0:
		return fmt.Errorf("parsed no rhythm rows")
	case len(p.doc.MetricDefs) == 0:
		return fmt.Errorf("parsed no metric definitions")
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

// deriveCheckpoints pulls the five questions out of the W12 checkpoint task's prose and
// attaches the same set to every other week whose checklist has a Checkpoint task.
//
// This is the brittle part of the parse - the questions live inside a sentence, not a list -
// which is why seed.json is committed and reviewed rather than generated at boot.
func (p *parser) deriveCheckpoints() error {
	const marker = "answer in writing:"

	var questions []string

	var weeks []string

	for _, t := range p.doc.Tasks {
		if !strings.Contains(strings.ToLower(t.Title), "checkpoint") {
			continue
		}

		weeks = append(weeks, t.Week)

		for _, step := range t.Steps {
			idx := strings.Index(strings.ToLower(step), marker)
			if idx < 0 {
				continue
			}

			questions = splitQuestions(step[idx+len(marker):])
		}
	}

	if len(weeks) == 0 {
		return nil
	}

	if len(questions) == 0 {
		return fmt.Errorf("found %d checkpoint tasks but no %q question list", len(weeks), marker)
	}

	for _, w := range weeks {
		p.doc.Checkpoints = append(p.doc.Checkpoints, seed.Checkpoint{
			Week:      w,
			Questions: questions,
		})
	}

	return nil
}

// splitQuestions turns "a? b? c?" into three questions, keeping the question marks.
func splitQuestions(text string) []string {
	var out []string

	for _, part := range strings.Split(text, "?") {
		q := strings.TrimSpace(part)
		if q == "" {
			continue
		}
		out = append(out, q+"?")
	}

	return out
}

// nextDate resolves "Aug 24" against the rolling calendar cursor.
func (p *parser) nextDate(month, day string) (string, error) {
	monthNum, ok := months[month]
	if !ok {
		return "", fmt.Errorf("unknown month %q", month)
	}

	dayNum, err := strconv.Atoi(day)
	if err != nil {
		return "", fmt.Errorf("bad day %q: %w", day, err)
	}

	// A month going backwards means the plan crossed into the next year (B4: Dec 28-Jan 10).
	if p.lastMonth != 0 && monthNum < p.lastMonth {
		p.year++
	}
	p.lastMonth = monthNum

	return fmt.Sprintf("%04d-%02d-%02d", p.year, monthNum, dayNum), nil
}

// parseWeekdays turns "Mon" or "Tue–Wed" into a CSV of ISO weekday numbers.
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

// normaliseProject maps the plan's loose tags onto the schema's CHECK set.
//
// Known lossy case: "synapse + gateway" (G11) collapses to synapse, so G11 does not appear
// under a gateway filter. Documented in docs/DATABASE.md section 3.
func normaliseProject(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))

	// Drop a parenthetical: "synapse (infra/)".
	if idx := strings.Index(value, "("); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}

	// Take the first of a multi-project tag: "synapse + gateway".
	if idx := strings.Index(value, "+"); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}

	// "all repos" is the plan's way of saying every project.
	if strings.HasPrefix(value, "all") {
		value = "all"
	}

	if !validProjects[value] {
		return "", fmt.Errorf("unknown project tag %q", raw)
	}

	return value, nil
}

// derivePhase reads a goal's phase from its target column.
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

// tableCells splits a markdown table row into trimmed cells.
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

// optional returns nil for an empty cell or the plan's em-dash placeholder.
func optional(value string) *string {
	v := strings.TrimSpace(value)
	if v == "" || v == "—" || v == "-" {
		return nil
	}

	return &v
}

// slug builds the stable half of a task's seed_key.
func slug(title string) string {
	s := slugRe.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(s, "-")
}

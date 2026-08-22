package api

import (
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// PlanWeek is the dashboard's current plan-window banner.
type PlanWeek struct {
	Code      *string `json:"code"`
	Phase     *string `json:"phase"`
	Focus     *string `json:"focus"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
	State     string  `json:"state"`
}

// Streak reports consecutive active Lisbon days ending today or yesterday.
type Streak struct {
	Days        int  `json:"days"`
	CountsToday bool `json:"counts_today"`
}

type planWindow struct {
	code      string
	phase     string
	startDate string
	endDate   string
}

var planWindows = []planWindow{
	{code: "W1", phase: "P1", startDate: "2026-08-24", endDate: "2026-08-30"},
	{code: "W2", phase: "P1", startDate: "2026-08-31", endDate: "2026-09-06"},
	{code: "W3", phase: "P1", startDate: "2026-09-07", endDate: "2026-09-13"},
	{code: "W4", phase: "P1", startDate: "2026-09-14", endDate: "2026-09-20"},
	{code: "W5", phase: "P1", startDate: "2026-09-21", endDate: "2026-09-27"},
	{code: "W6", phase: "P1", startDate: "2026-09-28", endDate: "2026-10-04"},
	{code: "W7", phase: "P1", startDate: "2026-10-05", endDate: "2026-10-11"},
	{code: "W8", phase: "P1", startDate: "2026-10-12", endDate: "2026-10-18"},
	{code: "W9", phase: "P1", startDate: "2026-10-19", endDate: "2026-10-25"},
	{code: "W10", phase: "P1", startDate: "2026-10-26", endDate: "2026-11-01"},
	{code: "W11", phase: "P1", startDate: "2026-11-02", endDate: "2026-11-08"},
	{code: "W12", phase: "P1", startDate: "2026-11-09", endDate: "2026-11-15"},
	{code: "B1", phase: "P2", startDate: "2026-11-16", endDate: "2026-11-29"},
	{code: "B2", phase: "P2", startDate: "2026-11-30", endDate: "2026-12-13"},
	{code: "B3", phase: "P2", startDate: "2026-12-14", endDate: "2026-12-27"},
	{code: "B4", phase: "P2", startDate: "2026-12-28", endDate: "2027-01-10"},
	{code: "B5", phase: "P2", startDate: "2027-01-11", endDate: "2027-01-24"},
	{code: "B6", phase: "P2", startDate: "2027-01-25", endDate: "2027-02-07"},
	{code: "B7", phase: "P2", startDate: "2027-02-08", endDate: "2027-02-21"},
}

// PlanWeekFor computes the active master-plan window using the supplied location's calendar day.
func PlanWeekFor(now time.Time, loc *time.Location) PlanWeek {
	today := now.In(loc).Format(dateLayout)

	if today < planWindows[0].startDate {
		return PlanWeek{State: "not_started"}
	}

	last := planWindows[len(planWindows)-1]
	if today > last.endDate {
		return PlanWeek{State: "plan_complete"}
	}

	for _, window := range planWindows {
		if today >= window.startDate && today <= window.endDate {
			code := window.code
			phase := window.phase
			start := window.startDate
			end := window.endDate

			return PlanWeek{
				Code:      &code,
				Phase:     &phase,
				StartDate: &start,
				EndDate:   &end,
				State:     "active",
			}
		}
	}

	return PlanWeek{State: "plan_complete"}
}

func lastPlanWeekCode() string {
	return planWindows[len(planWindows)-1].code
}

func firstPlanWeekCode() string {
	return planWindows[0].code
}

// StreakFor computes the run of consecutive active Lisbon days ending today or yesterday.
func StreakFor(logTimestamps []string, reviewDates []string, now time.Time, loc *time.Location) (Streak, error) {
	active := make(map[string]struct{}, len(logTimestamps)+len(reviewDates))

	for _, raw := range logTimestamps {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return Streak{}, fmt.Errorf("parse log timestamp %q: %w", raw, err)
		}
		active[ts.In(loc).Format(dateLayout)] = struct{}{}
	}

	for _, raw := range reviewDates {
		if _, err := time.ParseInLocation(dateLayout, raw, loc); err != nil {
			return Streak{}, fmt.Errorf("parse review date %q: %w", raw, err)
		}
		active[raw] = struct{}{}
	}

	today := localDate(now, loc)
	yesterday := today.AddDate(0, 0, -1)
	todayKey := today.Format(dateLayout)

	_, countsToday := active[todayKey]
	start := today
	if !countsToday {
		if _, ok := active[yesterday.Format(dateLayout)]; !ok {
			return Streak{Days: 0, CountsToday: false}, nil
		}
		start = yesterday
	}

	days := 0
	for day := start; ; day = day.AddDate(0, 0, -1) {
		if _, ok := active[day.Format(dateLayout)]; !ok {
			break
		}
		days++
	}

	return Streak{Days: days, CountsToday: countsToday}, nil
}

func localDate(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func parseLocalDate(raw string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(dateLayout, raw, loc)
}

func dateBounds(from, to string, loc *time.Location) (string, string, error) {
	var fromBound string
	var toBound string

	if from != "" {
		start, err := parseLocalDate(from, loc)
		if err != nil {
			return "", "", fmt.Errorf("invalid from date %q", from)
		}
		fromBound = start.UTC().Format(time.RFC3339)
	}

	if to != "" {
		end, err := parseLocalDate(to, loc)
		if err != nil {
			return "", "", fmt.Errorf("invalid to date %q", to)
		}
		toBound = end.AddDate(0, 0, 1).Add(-time.Second).UTC().Format(time.RFC3339)
	}

	if from != "" && to != "" && from > to {
		return "", "", fmt.Errorf("from must be on or before to")
	}

	return fromBound, toBound, nil
}

func currentWeekDateBounds(now time.Time, loc *time.Location) (string, string) {
	today := localDate(now, loc)
	offset := (int(today.Weekday()) + 6) % 7
	start := today.AddDate(0, 0, -offset)
	end := start.AddDate(0, 0, 6)
	return start.Format(dateLayout), end.Format(dateLayout)
}

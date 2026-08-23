package api

import (
	"fmt"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/store"
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

// PlanWeekFor computes the active plan window using the supplied location's calendar day.
func PlanWeekFor(now time.Time, loc *time.Location, weeks []store.Week) PlanWeek {
	if len(weeks) == 0 {
		return PlanWeek{State: "not_started"}
	}

	today := now.In(loc).Format(dateLayout)

	if today < weeks[0].StartDate {
		return PlanWeek{State: "not_started"}
	}

	last := weeks[len(weeks)-1]
	if today > last.EndDate {
		return PlanWeek{State: "plan_complete"}
	}

	for _, window := range weeks {
		if today >= window.StartDate && today <= window.EndDate {
			code := window.Code
			phase := window.Phase
			start := window.StartDate
			end := window.EndDate

			return PlanWeek{
				Code:      &code,
				Phase:     &phase,
				Focus:     window.Focus,
				StartDate: &start,
				EndDate:   &end,
				State:     "active",
			}
		}
	}

	return PlanWeek{State: "plan_complete"}
}

func lastPlanWeekCode(weeks []store.Week) string {
	if len(weeks) == 0 {
		return ""
	}
	return weeks[len(weeks)-1].Code
}

func firstPlanWeekCode(weeks []store.Week) string {
	if len(weeks) == 0 {
		return ""
	}
	return weeks[0].Code
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

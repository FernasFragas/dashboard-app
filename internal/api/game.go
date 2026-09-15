package api

import (
	"context"
	"net/http"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/store"
)

type gameSkillsResponse struct {
	Skills []store.GameSkill `json:"skills"`
}

type gameEventsResponse struct {
	Events []store.XPEvent `json:"events"`
}

type taskWriteResponse struct {
	store.Task
	Game *store.GameEnvelope `json:"game,omitempty"`
}

type logWriteResponse struct {
	store.LogEntry
	Game *store.GameEnvelope `json:"game,omitempty"`
}

type reviewWriteResponse struct {
	store.DailyReview
	Game *store.GameEnvelope `json:"game,omitempty"`
}

type metricWriteResponse struct {
	store.Metric
	Game *store.GameEnvelope `json:"game,omitempty"`
}

type checkpointWriteResponse struct {
	store.Checkpoint
	Game *store.GameEnvelope `json:"game,omitempty"`
}

type goalWriteResponse struct {
	Goal      store.Goal          `json:"goal"`
	Reordered []store.Goal        `json:"reordered"`
	Game      *store.GameEnvelope `json:"game,omitempty"`
}

type gameAwardFunc func(context.Context) (store.GameAward, error)
type gameAchievementCondition func(store.GameAchievementFacts, time.Time, *time.Location) bool

var gameAchievementConditions = map[string]gameAchievementCondition{
	"first_task": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.DoneTasks > 0
	},
	"first_number": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.NumberLogs > 0
	},
	"gatekeeper": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.DoneSeedKeys["W7:regression-gate-in-ci"]
	},
	"chaos_suite": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		for _, key := range []string{
			"W9:k6-mixed-load-baseline",
			"W9:kill-redis-mid-load",
			"W9:kill-jwks-mid-load",
			"W10:slot-starvation",
			"W10:failover-lands-on-vllm",
		} {
			if !facts.DoneSeedKeys[key] {
				return false
			}
		}
		return true
	},
	"honest_number": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.DoneSeedKeys["W14:the-honest-number"]
	},
	"in_the_arena": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.DoneSeedKeys["W2:pr-1-docs-or-conformance"]
	},
	"shepherd": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.DoneGoalCodes["G7"]
	},
	"cold_caller": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.ApplicationLogs >= 10
	},
	"wordsmith": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.PostLogs >= 3
	},
	"iron_week": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.CompleteWeeks > 0
	},
	"well_rested": func(facts store.GameAchievementFacts, now time.Time, loc *time.Location) bool {
		return gameWellRested(facts.XPEventTimes, now, loc)
	},
	"boss_w12": func(facts store.GameAchievementFacts, _ time.Time, _ *time.Location) bool {
		return facts.CompletedCheckpoints["W16"]
	},
	"streak_7": func(facts store.GameAchievementFacts, now time.Time, loc *time.Location) bool {
		return gameStreakAtLeast(facts.XPEventTimes, now, loc, 7)
	},
	"streak_30": func(facts store.GameAchievementFacts, now time.Time, loc *time.Location) bool {
		return gameStreakAtLeast(facts.XPEventTimes, now, loc, 30)
	},
}

func (s *Server) gameProfile(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	profile, err := s.store.GameProfile(r.Context(), s.now(), s.location)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, profile)
}

func (s *Server) gameSkills(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	skills, err := s.store.GameSkills(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if skills == nil {
		skills = []store.GameSkill{}
	}

	s.writeJSON(w, http.StatusOK, gameSkillsResponse{Skills: skills})
}

func (s *Server) gameEvents(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "limit"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit, err := parsePositiveLimit(r.URL.Query().Get("limit"), 20, 100)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	events, err := s.store.ListXPEvents(r.Context(), limit)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if events == nil {
		events = []store.XPEvent{}
	}

	s.writeJSON(w, http.StatusOK, gameEventsResponse{Events: events})
}

func (s *Server) gameEnvelope(ctx context.Context, award gameAwardFunc) (*store.GameEnvelope, error) {
	before, err := s.store.GameProfile(ctx, s.now(), s.location)
	if err != nil {
		return nil, err
	}

	var result store.GameAward
	if award != nil {
		result, err = award(ctx)
		if err != nil {
			return nil, err
		}
	}

	unlocks, err := s.evaluateGameAchievements(ctx)
	if err != nil {
		return nil, err
	}

	after, err := s.store.GameProfile(ctx, s.now(), s.location)
	if err != nil {
		return nil, err
	}

	envelope := store.GameEnvelope{
		XPAwarded: result.XPAwarded,
		Unlocks:   unlocks,
		Event:     result.Event,
	}
	if after.Level > before.Level {
		envelope.LevelUp = &store.GameLevelUp{
			Level:         after.Level,
			Title:         after.Title,
			TotalXP:       after.TotalXP,
			NextThreshold: after.NextThreshold,
		}
	}

	if envelope.XPAwarded == 0 && envelope.LevelUp == nil && len(envelope.Unlocks) == 0 {
		return nil, nil
	}
	if envelope.Unlocks == nil {
		envelope.Unlocks = []store.Achievement{}
	}

	return &envelope, nil
}

func (s *Server) evaluateGameAchievements(ctx context.Context) ([]store.Achievement, error) {
	facts, err := s.store.GameAchievementFacts(ctx)
	if err != nil {
		return nil, err
	}

	achievements, err := s.store.ListAchievements(ctx)
	if err != nil {
		return nil, err
	}

	var unlocked []store.Achievement
	for _, achievement := range achievements {
		if achievement.UnlockedAt != nil {
			continue
		}

		condition := gameAchievementConditions[achievement.Code]
		if condition == nil || !condition(facts, s.now(), s.location) {
			continue
		}

		next, created, err := s.store.UnlockAchievement(ctx, achievement.Code)
		if err != nil {
			return nil, err
		}
		if created {
			unlocked = append(unlocked, next)
		}
	}

	return unlocked, nil
}

func gameStreakAtLeast(eventTimestamps []string, now time.Time, loc *time.Location, days int) bool {
	streak, err := store.GameStreakFor(eventTimestamps, now, loc)
	if err != nil {
		return false
	}
	return streak.Days >= days
}

func gameWellRested(eventTimestamps []string, now time.Time, loc *time.Location) bool {
	active := map[string]struct{}{}
	var firstEvent *time.Time

	for _, raw := range eventTimestamps {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return false
		}

		day := localDate(ts, loc)
		active[day.Format(dateLayout)] = struct{}{}
		if firstEvent == nil || day.Before(*firstEvent) {
			firstDay := day
			firstEvent = &firstDay
		}
	}
	if firstEvent == nil {
		return false
	}

	day := *firstEvent
	for day.Weekday() != time.Sunday {
		day = day.AddDate(0, 0, 1)
	}

	today := localDate(now, loc)
	consecutive := 0
	for !day.After(today) {
		if _, ok := active[day.Format(dateLayout)]; ok {
			consecutive = 0
		} else {
			consecutive++
			if consecutive >= 4 {
				return true
			}
		}

		day = day.AddDate(0, 0, 7)
	}

	return false
}

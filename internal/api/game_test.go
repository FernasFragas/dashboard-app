package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/store"
)

func TestTaskWriteReturnsGameEnvelopeAndGameEndpoints(t *testing.T) {
	srv := newTestAPI(t, "secret")

	var taskID int64
	var version int
	if err := srv.store.DB().QueryRowContext(context.Background(), `
		SELECT id, version FROM tasks WHERE seed_key = 'W3:regression-gate-in-ci'`).Scan(&taskID, &version); err != nil {
		t.Fatalf("find seeded task: %v", err)
	}

	rec := srv.requestWithHeaders(http.MethodPatch, "/api/tasks/"+idString(taskID),
		map[string]any{"status": "done"},
		map[string]string{"If-Match": etag(taskID, version)},
	)
	assertStatus(t, rec, http.StatusOK)

	var body taskWriteResponse
	decodeBody(t, rec, &body)
	if body.Game == nil {
		t.Fatal("game envelope is nil")
	}
	if body.Game.XPAwarded != 10 {
		t.Fatalf("xp awarded = %d, want 10", body.Game.XPAwarded)
	}
	if !hasUnlock(body.Game.Unlocks, "gatekeeper") {
		t.Fatalf("unlocks = %+v, want gatekeeper", body.Game.Unlocks)
	}

	profileRec := srv.request(http.MethodGet, "/api/game/profile", nil)
	assertStatus(t, profileRec, http.StatusOK)
	var profile store.GameProfile
	decodeBody(t, profileRec, &profile)
	if profile.TotalXP != 10 {
		t.Fatalf("profile total xp = %d, want 10", profile.TotalXP)
	}

	skillsRec := srv.request(http.MethodGet, "/api/game/skills", nil)
	assertStatus(t, skillsRec, http.StatusOK)
	var skills gameSkillsResponse
	decodeBody(t, skillsRec, &skills)
	if len(skills.Skills) == 0 {
		t.Fatal("game skills endpoint returned no skills")
	}

	eventsRec := srv.request(http.MethodGet, "/api/game/events?limit=1", nil)
	assertStatus(t, eventsRec, http.StatusOK)
	var events gameEventsResponse
	decodeBody(t, eventsRec, &events)
	if len(events.Events) != 1 || events.Events[0].Amount != 10 {
		t.Fatalf("events = %+v, want one +10 event", events.Events)
	}

	rec = srv.requestWithHeaders(http.MethodPatch, "/api/tasks/"+idString(taskID),
		map[string]any{"status": "done"},
		map[string]string{"If-Match": etag(taskID, body.Version)},
	)
	assertStatus(t, rec, http.StatusOK)
	var second taskWriteResponse
	decodeBody(t, rec, &second)
	if second.Game != nil {
		t.Fatalf("second done response game = %+v, want nil", second.Game)
	}

	var unlocks int
	if err := srv.store.DB().QueryRowContext(context.Background(), `
		SELECT count(*)
		FROM achievement_unlocks au
		JOIN achievements a ON a.id = au.achievement_id
		WHERE a.code = 'gatekeeper'`).Scan(&unlocks); err != nil {
		t.Fatalf("count gatekeeper unlocks: %v", err)
	}
	if unlocks != 1 {
		t.Fatalf("gatekeeper unlock rows = %d, want 1", unlocks)
	}
}

func TestGameAchievementConditions(t *testing.T) {
	loc := lisbon(t)

	facts := store.GameAchievementFacts{
		DoneSeedKeys: map[string]bool{
			"W3:regression-gate-in-ci": true,
		},
		CompleteWeeks: 1,
	}
	if !gameAchievementConditions["gatekeeper"](facts, apiNow, loc) {
		t.Fatal("gatekeeper condition = false, want true")
	}
	if !gameAchievementConditions["iron_week"](facts, apiNow, loc) {
		t.Fatal("iron_week condition = false, want true")
	}

	wellRestedFacts := store.GameAchievementFacts{
		XPEventTimes: []string{
			localTime(loc, 2026, time.August, 3, 10, 0).UTC().Format(time.RFC3339),
		},
	}
	now := localTime(loc, 2026, time.August, 31, 10, 0)
	if !gameAchievementConditions["well_rested"](wellRestedFacts, now, loc) {
		t.Fatal("well_rested condition = false, want true after four zero-XP Sundays")
	}

	if gameAchievementConditions["well_rested"](store.GameAchievementFacts{}, now, loc) {
		t.Fatal("well_rested condition = true before first XP event")
	}
}

func hasUnlock(unlocks []store.Achievement, code string) bool {
	for _, unlock := range unlocks {
		if unlock.Code == code {
			return true
		}
	}
	return false
}

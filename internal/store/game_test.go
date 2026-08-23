package store

import (
	"context"
	"testing"
	"time"
)

func TestSyncTaskXPAwardsOnceAndUncheckRemovesEvent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	task := createTask(t, s, "W5", "Seeded task", "gateway")
	if _, err := s.db.ExecContext(ctx, `UPDATE tasks SET seed_key = 'W5:seeded-task' WHERE id = ?`, task.ID); err != nil {
		t.Fatalf("mark task seeded: %v", err)
	}

	done, err := s.UpdateTask(ctx, task.ID, task.Version, TaskPatch{Status: ptr("done")})
	if err != nil {
		t.Fatalf("toggle done: %v", err)
	}

	award, err := s.SyncTaskXP(ctx, done.ID)
	if err != nil {
		t.Fatalf("sync task xp: %v", err)
	}
	if award.XPAwarded != 10 {
		t.Fatalf("xp awarded = %d, want 10", award.XPAwarded)
	}

	again, err := s.SyncTaskXP(ctx, done.ID)
	if err != nil {
		t.Fatalf("sync task xp again: %v", err)
	}
	if again.XPAwarded != 0 {
		t.Fatalf("second xp awarded = %d, want 0", again.XPAwarded)
	}

	profile, err := s.GameProfile(ctx, fixedNow, time.UTC)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.TotalXP != 10 {
		t.Fatalf("total xp = %d, want 10", profile.TotalXP)
	}

	todo, err := s.UpdateTask(ctx, done.ID, done.Version, TaskPatch{Status: ptr("todo")})
	if err != nil {
		t.Fatalf("toggle todo: %v", err)
	}
	if _, err := s.SyncTaskXP(ctx, todo.ID); err != nil {
		t.Fatalf("sync unchecked task xp: %v", err)
	}

	profile, err = s.GameProfile(ctx, fixedNow, time.UTC)
	if err != nil {
		t.Fatalf("profile after uncheck: %v", err)
	}
	if profile.TotalXP != 0 {
		t.Fatalf("total xp after uncheck = %d, want 0", profile.TotalXP)
	}

	doneAgain, err := s.UpdateTask(ctx, todo.ID, todo.Version, TaskPatch{Status: ptr("done")})
	if err != nil {
		t.Fatalf("toggle done again: %v", err)
	}
	reaward, err := s.SyncTaskXP(ctx, doneAgain.ID)
	if err != nil {
		t.Fatalf("sync rechecked task xp: %v", err)
	}
	if reaward.XPAwarded != 10 {
		t.Fatalf("recheck xp awarded = %d, want 10", reaward.XPAwarded)
	}
}

func TestSkillXPMirrorsFullAwardToEachLinkedSkill(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO skills (id, code, name, description, associate_when, target_tier,
			sort_order, created_at)
		VALUES (2, 'chaos', 'Chaos testing', 'Break systems deliberately.',
			'the task generates load or injects failure.', 'Expert', 20, ?)`, s.utcNow()); err != nil {
		t.Fatalf("insert second skill: %v", err)
	}

	task, err := s.CreateTask(ctx, NewTask{
		Week: "W5", Title: "Ad-hoc two-skill task", Project: "gateway",
		SkillIDs: []int64{fixtureSkillID, 2},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	done, err := s.UpdateTask(ctx, task.ID, task.Version, TaskPatch{Status: ptr("done")})
	if err != nil {
		t.Fatalf("toggle done: %v", err)
	}
	if _, err := s.SyncTaskXP(ctx, done.ID); err != nil {
		t.Fatalf("sync task xp: %v", err)
	}

	profile, err := s.GameProfile(ctx, fixedNow, time.UTC)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.TotalXP != 5 {
		t.Fatalf("player xp = %d, want 5", profile.TotalXP)
	}

	skills, err := s.GameSkills(ctx)
	if err != nil {
		t.Fatalf("game skills: %v", err)
	}

	got := map[int64]int{}
	for _, skill := range skills {
		got[skill.SkillID] = skill.XP
	}
	if got[fixtureSkillID] != 5 || got[2] != 5 {
		t.Fatalf("skill xp = %v, want both linked skills at 5", got)
	}
}

func TestGameProfileLevelThresholds(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO xp_events (source_type, source_id, amount, created_at)
		VALUES ('metric', 1, 50, ?)`, s.utcNow()); err != nil {
		t.Fatalf("insert xp event: %v", err)
	}

	profile, err := s.GameProfile(ctx, fixedNow, time.UTC)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.Level != 1 || profile.Title != "Backend Engineer" || profile.NextThreshold != 150 {
		t.Fatalf("profile = %+v, want level 1 Backend Engineer with next threshold 150", profile)
	}
}

func TestGameStreakForSundayShield(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	events := []string{
		localGameTime(loc, 2026, 9, 25, 9, 0).UTC().Format(time.RFC3339),
		localGameTime(loc, 2026, 9, 26, 9, 0).UTC().Format(time.RFC3339),
	}

	monday, err := GameStreakFor(events, localGameTime(loc, 2026, 9, 28, 10, 0), loc)
	if err != nil {
		t.Fatalf("monday streak: %v", err)
	}
	if monday.Days != 2 || monday.CountsToday || !monday.Shielded {
		t.Fatalf("monday streak = %+v, want 2 days, counts_today=false, shielded=true", monday)
	}

	tuesday, err := GameStreakFor(events, localGameTime(loc, 2026, 9, 29, 10, 0), loc)
	if err != nil {
		t.Fatalf("tuesday streak: %v", err)
	}
	if tuesday.Days != 0 || tuesday.CountsToday || tuesday.Shielded {
		t.Fatalf("tuesday streak = %+v, want broken streak", tuesday)
	}
}

func localGameTime(loc *time.Location, year int, month time.Month, day int, hour int, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, loc)
}

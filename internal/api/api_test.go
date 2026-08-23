package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/seed"
	"github.com/FernasFragas/dashboard-app/internal/store"
	"github.com/FernasFragas/dashboard-app/migrations"
)

var apiNow = time.Date(2026, 9, 24, 17, 40, 0, 0, time.UTC)

func TestAPIEndpoints(t *testing.T) {
	srv := newTestAPI(t, "secret")

	assertStatus(t, srv.request(http.MethodGet, "/api/dashboard", nil), http.StatusOK)
	projectsResponse := srv.request(http.MethodGet, "/api/projects", nil)
	assertStatus(t, projectsResponse, http.StatusOK)
	var projectsBody struct {
		Projects []store.Project `json:"projects"`
	}
	decodeBody(t, projectsResponse, &projectsBody)
	if len(projectsBody.Projects) == 0 {
		t.Fatal("projects endpoint returned no projects")
	}
	assertStatus(t, srv.request(http.MethodGet, "/api/categories", nil), http.StatusOK)
	metricDefsResponse := srv.request(http.MethodGet, "/api/metric-defs", nil)
	assertStatus(t, metricDefsResponse, http.StatusOK)
	var metricDefs struct {
		MetricDefs []store.MetricDef `json:"metric_defs"`
	}
	decodeBody(t, metricDefsResponse, &metricDefs)
	if len(metricDefs.MetricDefs) == 0 ||
		metricDefs.MetricDefs[0].Slug == nil ||
		metricDefs.MetricDefs[0].Definition == nil ||
		metricDefs.MetricDefs[0].HowToMeasure == nil {
		t.Fatalf("metric defs missing field-guide help: %+v", metricDefs.MetricDefs)
	}
	assertStatus(t, srv.request(http.MethodGet, "/api/goals", nil), http.StatusOK)
	assertStatus(t, srv.request(http.MethodGet, "/api/tasks?week=W5", nil), http.StatusOK)

	createGoal := srv.request(http.MethodPost, "/api/goals", map[string]any{
		"title":      "Ship the eval CI gate",
		"project":    "synapse",
		"done_means": "CI fails on regression",
		"target":     "W6",
	})
	assertStatus(t, createGoal, http.StatusCreated)
	var goal store.Goal
	decodeBody(t, createGoal, &goal)
	if createGoal.Header().Get("Location") == "" || createGoal.Header().Get("ETag") == "" {
		t.Fatalf("created goal missing Location or ETag headers")
	}

	assertStatus(t, srv.request(http.MethodGet, "/api/goals/"+idString(goal.ID), nil), http.StatusOK)

	moveGoal := srv.requestWithHeaders(http.MethodPatch, "/api/goals/"+idString(goal.ID),
		map[string]any{"status": "active", "position": 0},
		map[string]string{"If-Match": etag(goal.ID, goal.Version)},
	)
	assertStatus(t, moveGoal, http.StatusOK)
	if moveGoal.Header().Get("ETag") == "" {
		t.Fatalf("patched goal missing ETag")
	}
	var movedGoal struct {
		Goal store.Goal `json:"goal"`
	}
	decodeBody(t, moveGoal, &movedGoal)

	// Every task builds at least one skill, so the create path needs a real skill id.
	listSkills := srv.request(http.MethodGet, "/api/skills", nil)
	assertStatus(t, listSkills, http.StatusOK)
	var skillsBody struct {
		Skills []store.Skill `json:"skills"`
	}
	decodeBody(t, listSkills, &skillsBody)
	if len(skillsBody.Skills) == 0 {
		t.Fatal("no seeded skills")
	}
	skillID := skillsBody.Skills[0].ID

	// A task with no skills is refused: the request is well formed, the model forbids it.
	assertStatus(t, srv.request(http.MethodPost, "/api/tasks", map[string]any{
		"week": "W5", "title": "No skill", "project": "gateway",
	}), http.StatusUnprocessableEntity)

	// task_week must be set even outside the plan window: it is the week the add-task sheet
	// writes to, and gating on week.code left the UI unable to add a task before W1 started.
	dashboard := srv.request(http.MethodGet, "/api/dashboard", nil)
	assertStatus(t, dashboard, http.StatusOK)
	var dash struct {
		Week     struct{ State string } `json:"week"`
		TaskWeek *struct {
			Code      string `json:"code"`
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
		} `json:"task_week"`
	}
	decodeBody(t, dashboard, &dash)
	if dash.TaskWeek == nil || dash.TaskWeek.Code == "" {
		t.Fatalf("task_week = %v in state %q, want a week to add tasks to", dash.TaskWeek, dash.Week.State)
	}
	// The dates come with it, so the banner can say when an upcoming week starts without a
	// second request.
	if dash.TaskWeek.StartDate == "" || dash.TaskWeek.EndDate == "" {
		t.Errorf("task_week = %+v, want start and end dates", *dash.TaskWeek)
	}

	createTask := srv.request(http.MethodPost, "/api/tasks", map[string]any{
		"week":       "W5",
		"title":      "Re-run k6 after the timeout change",
		"project":    "gateway",
		"goal_id":    movedGoal.Goal.ID,
		"steps":      []string{"Run baseline", "Record p95"},
		"done_means": "numbers are in the review",
		"skill_ids":  []int64{skillID},
	})
	assertStatus(t, createTask, http.StatusCreated)
	var task store.Task
	decodeBody(t, createTask, &task)

	assertStatus(t, srv.request(http.MethodGet, "/api/tasks/"+idString(task.ID), nil), http.StatusOK)

	toggleTask := srv.requestWithHeaders(http.MethodPatch, "/api/tasks/"+idString(task.ID),
		map[string]any{"status": "done"},
		map[string]string{"If-Match": etag(task.ID, task.Version)},
	)
	assertStatus(t, toggleTask, http.StatusOK)

	createLog := srv.request(http.MethodPost, "/api/logs", map[string]any{
		"category_id": "application",
		"title":       "Applied - Platform Engineer @ Acme",
		"url":         "https://acme.example/careers/1234",
		"goal_id":     movedGoal.Goal.ID,
	})
	assertStatus(t, createLog, http.StatusCreated)
	var entry store.LogEntry
	decodeBody(t, createLog, &entry)

	assertStatus(t, srv.request(http.MethodGet, "/api/logs?limit=20", nil), http.StatusOK)
	assertStatus(t, srv.request(http.MethodGet, "/api/logs/summary", nil), http.StatusOK)

	review := srv.request(http.MethodPost, "/api/reviews", map[string]any{
		"date":    "2026-09-24",
		"learned": "WAL checkpointing blocks on a long read txn",
		"minutes": 45,
	})
	assertStatus(t, review, http.StatusOK)
	assertStatus(t, srv.request(http.MethodGet, "/api/reviews?from=2026-09-01&to=2026-09-30", nil), http.StatusOK)

	metric := srv.request(http.MethodPost, "/api/metrics", map[string]any{
		"name":  "p95 latency (cached)",
		"value": 0.74,
		"unit":  "s",
		"note":  "after cache warm",
	})
	assertStatus(t, metric, http.StatusCreated)
	assertStatus(t, srv.request(http.MethodGet, "/api/metrics?name=p95+latency+%28cached%29", nil), http.StatusOK)

	checkpoint := srv.request(http.MethodGet, "/api/checkpoints/W12", nil)
	assertStatus(t, checkpoint, http.StatusOK)
	var cp store.Checkpoint
	decodeBody(t, checkpoint, &cp)
	if len(cp.Helpers) != len(cp.Questions) {
		t.Fatalf("checkpoint helpers = %d, questions = %d", len(cp.Helpers), len(cp.Questions))
	}
	answers := make([]string, len(cp.Questions))
	for i := range answers {
		answers[i] = "answer"
	}
	assertStatus(t, srv.request(http.MethodPut, "/api/checkpoints/W12", map[string]any{
		"answers": answers,
	}), http.StatusOK)

	export := srv.request(http.MethodGet, "/api/export", nil)
	assertStatus(t, export, http.StatusOK)
	if !strings.Contains(export.Header().Get("Content-Disposition"), "dashboard-export-20260924.json") {
		t.Fatalf("unexpected export disposition %q", export.Header().Get("Content-Disposition"))
	}

	assertStatus(t, srv.request(http.MethodDelete, "/api/logs/"+idString(entry.ID), nil), http.StatusNoContent)
	assertStatus(t, srv.request(http.MethodDelete, "/api/tasks/"+idString(task.ID), nil), http.StatusNoContent)
	assertStatus(t, srv.request(http.MethodDelete, "/api/goals/"+idString(movedGoal.Goal.ID), nil), http.StatusNoContent)
}

func TestAPIErrorCases(t *testing.T) {
	srv := newTestAPI(t, "secret")

	assertStatus(t, srv.requestNoToken(http.MethodGet, "/api/health", nil), http.StatusOK)
	assertErrorEnvelope(t, srv.requestWithHeaders(http.MethodGet, "/api/categories", nil,
		map[string]string{"X-Token": "wrong"}), http.StatusUnauthorized)

	assertErrorEnvelope(t, srv.request(http.MethodPost, "/api/goals", map[string]any{
		"title":   "   ",
		"project": "synapse",
	}), http.StatusBadRequest)

	assertErrorEnvelope(t, srv.request(http.MethodPost, "/api/goals", map[string]any{
		"title":   "Bad enum",
		"project": "unknown",
	}), http.StatusBadRequest)

	assertErrorEnvelope(t, srv.request(http.MethodGet, "/api/goals?typo=1", nil), http.StatusBadRequest)
	assertErrorEnvelope(t, srv.request(http.MethodGet, "/api/goals/999999", nil), http.StatusNotFound)

	// A task needs a skill, so borrow the first seeded one.
	skills := srv.request(http.MethodGet, "/api/skills", nil)
	assertStatus(t, skills, http.StatusOK)
	var skillsBody struct {
		Skills []store.Skill `json:"skills"`
	}
	decodeBody(t, skills, &skillsBody)
	if len(skillsBody.Skills) == 0 {
		t.Fatal("no seeded skills")
	}

	// An unknown skill id is a malformed reference, not a policy refusal: 400, not 422.
	assertErrorEnvelope(t, srv.request(http.MethodPost, "/api/tasks", map[string]any{
		"week": "W5", "title": "Bad skill", "project": "gateway",
		"skill_ids": []int64{999999},
	}), http.StatusBadRequest)

	created := srv.request(http.MethodPost, "/api/tasks", map[string]any{
		"week":      "W5",
		"title":     "Toggle me",
		"project":   "gateway",
		"skill_ids": []int64{skillsBody.Skills[0].ID},
	})
	assertStatus(t, created, http.StatusCreated)
	var task store.Task
	decodeBody(t, created, &task)

	assertErrorEnvelope(t, srv.request(http.MethodPatch, "/api/tasks/"+idString(task.ID),
		map[string]any{"status": "done"}), http.StatusPreconditionRequired)

	stale := srv.requestWithHeaders(http.MethodPatch, "/api/tasks/"+idString(task.ID),
		map[string]any{"status": "done"},
		map[string]string{"If-Match": etag(task.ID, task.Version+10)},
	)
	assertStatus(t, stale, http.StatusPreconditionFailed)
	var body map[string]any
	decodeBody(t, stale, &body)
	if _, ok := body["error"]; !ok {
		t.Fatalf("stale response missing error field: %#v", body)
	}
	if _, ok := body["current"]; !ok {
		t.Fatalf("stale response missing current resource: %#v", body)
	}
}

func TestAPIServesEmptyPlan(t *testing.T) {
	srv := newEmptyTestAPI(t, "secret")

	dashboard := srv.request(http.MethodGet, "/api/dashboard", nil)
	assertStatus(t, dashboard, http.StatusOK)
	var body dashboardResponse
	decodeBody(t, dashboard, &body)
	if body.Week.State != "not_started" {
		t.Fatalf("week state = %q, want not_started", body.Week.State)
	}
	if len(body.Tasks) != 0 || body.Completion.Total != 0 {
		t.Fatalf("tasks = %+v, completion = %+v; want empty", body.Tasks, body.Completion)
	}

	assertStatus(t, srv.request(http.MethodGet, "/api/projects", nil), http.StatusOK)
	assertStatus(t, srv.request(http.MethodGet, "/api/categories", nil), http.StatusOK)
	assertStatus(t, srv.request(http.MethodGet, "/api/goals", nil), http.StatusOK)
	assertStatus(t, srv.request(http.MethodGet, "/api/skills", nil), http.StatusOK)
}

func TestPlanPreviewValidPlanChangesNoRows(t *testing.T) {
	srv := newTestAPI(t, "secret")
	before := apiTableCounts(t, srv.store)

	rec := srv.requestRaw(http.MethodPost, "/api/plan/preview", []byte(uploadedPlanSource("uploaded-plan")),
		"text/markdown")
	assertStatus(t, rec, http.StatusOK)

	var body planPreviewResponse
	decodeBody(t, rec, &body)
	if len(body.Errors) != 0 {
		t.Fatalf("preview errors = %+v", body.Errors)
	}
	if body.Mode != "replace" {
		t.Fatalf("mode = %q, want replace", body.Mode)
	}
	if body.Parsed.Weeks != 1 || body.Parsed.Tasks != 1 || body.Parsed.Projects != 1 {
		t.Fatalf("parsed counts = %+v, want one-week upload", body.Parsed)
	}

	after := apiTableCounts(t, srv.store)
	if fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("preview changed rows:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestPlanPreviewMalformedPlanReturnsErrorsAndChangesNoRows(t *testing.T) {
	srv := newTestAPI(t, "secret")
	before := apiTableCounts(t, srv.store)

	source := `---
plan:
  id: broken-upload
  name: "Broken Upload"
  start_year: 2030
---
# Phase 1

## W1 · Jan 1-7 — Open
this is line ten
`
	rec := srv.requestRaw(http.MethodPost, "/api/plan/preview", []byte(source), "text/markdown")
	assertStatus(t, rec, http.StatusOK)

	var body planPreviewResponse
	decodeBody(t, rec, &body)
	if len(body.Errors) != 1 {
		t.Fatalf("errors = %+v, want one parse error", body.Errors)
	}
	if body.Errors[0].Line == nil || *body.Errors[0].Line != 10 {
		t.Fatalf("line = %v, want 10", body.Errors[0].Line)
	}
	if body.Errors[0].Excerpt == nil || *body.Errors[0].Excerpt != "this is line ten" {
		t.Fatalf("excerpt = %v, want source line", body.Errors[0].Excerpt)
	}

	after := apiTableCounts(t, srv.store)
	if fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("malformed preview changed rows:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestPlanApplyRequiresPreviewedHashAndReplaceConfirmation(t *testing.T) {
	srv := newTestAPI(t, "secret")
	source := uploadedPlanSource("uploaded-plan")
	sha := sourceSHA256(source)

	assertErrorEnvelope(t, srv.request(http.MethodPost, "/api/plan/apply", map[string]any{
		"source":        source,
		"source_sha256": "bad",
	}), http.StatusConflict)

	assertErrorEnvelope(t, srv.request(http.MethodPost, "/api/plan/apply", map[string]any{
		"source":        source,
		"source_sha256": sha,
	}), http.StatusConflict)

	preview := srv.requestRaw(http.MethodPost, "/api/plan/preview", []byte(source), "text/markdown")
	assertStatus(t, preview, http.StatusOK)

	assertErrorEnvelope(t, srv.request(http.MethodPost, "/api/plan/apply", map[string]any{
		"source":        source,
		"source_sha256": sha,
	}), http.StatusUnprocessableEntity)
}

func TestPlanApplyReplacePreservesHistoryAndWritesBackup(t *testing.T) {
	srv := newTestAPI(t, "secret")
	ctx := context.Background()

	if _, err := srv.store.DB().ExecContext(ctx, `
		UPDATE goals SET status = 'active' WHERE code = 'G0'`); err != nil {
		t.Fatalf("activate goal: %v", err)
	}
	var goalID int64
	if err := srv.store.DB().QueryRowContext(ctx, `SELECT id FROM goals WHERE code = 'G0'`).Scan(&goalID); err != nil {
		t.Fatalf("read goal id: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `
		INSERT INTO log_entries (category_id, title, goal_id, occurred_at, created_at)
		VALUES ('application', 'kept log', ?, '2026-09-24T18:00:00Z', '2026-09-24T18:00:00Z')`,
		goalID,
	); err != nil {
		t.Fatalf("insert log: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `
		INSERT INTO daily_reviews (date, learned, created_at, updated_at)
		VALUES ('2026-09-24', 'keep me', '2026-09-24T18:00:00Z', '2026-09-24T18:00:00Z')`); err != nil {
		t.Fatalf("insert review: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `
		INSERT INTO metrics (name, value, recorded_at)
		VALUES ('p95 latency', 0.8, '2026-09-24T18:00:00Z')`); err != nil {
		t.Fatalf("insert metric: %v", err)
	}

	source := uploadedPlanSource("uploaded-plan")
	sha := sourceSHA256(source)
	assertStatus(t, srv.requestRaw(http.MethodPost, "/api/plan/preview", []byte(source),
		"text/markdown"), http.StatusOK)

	rec := srv.request(http.MethodPost, "/api/plan/apply", map[string]any{
		"source":            source,
		"source_sha256":     sha,
		"confirm_plan_name": "Uploaded Plan",
	})
	assertStatus(t, rec, http.StatusOK)

	var body applyPlanResponse
	decodeBody(t, rec, &body)
	if body.BackupPath == nil || !strings.Contains(*body.BackupPath, "pre-plan-uploaded-plan-") {
		t.Fatalf("backup_path = %v, want pre-plan backup", body.BackupPath)
	}
	if _, err := os.Stat(*body.BackupPath); err != nil {
		t.Fatalf("backup path does not exist: %v", err)
	}
	if body.Source.SHA256 != sha || body.Source.Source != source {
		t.Fatalf("stored source = %+v, want uploaded source", body.Source)
	}

	for _, tc := range []struct {
		table string
		want  int
	}{
		{"log_entries", 1},
		{"daily_reviews", 1},
		{"metrics", 1},
	} {
		if got := apiCount(t, srv.store, tc.table); got != tc.want {
			t.Errorf("%s rows = %d, want %d", tc.table, got, tc.want)
		}
	}
	var retiredAt *string
	if err := srv.store.DB().QueryRowContext(ctx,
		`SELECT retired_at FROM categories WHERE id = 'application'`).Scan(&retiredAt); err != nil {
		t.Fatalf("read retired category: %v", err)
	}
	if retiredAt == nil {
		t.Fatal("referenced old category was not retired")
	}
}

func TestPlanUploadRejectsOversizedAndBinaryBodies(t *testing.T) {
	srv := newTestAPI(t, "secret")

	tooLarge := strings.Repeat("a", maxPlanSourceBytes+1)
	assertStatus(t, srv.requestRaw(http.MethodPost, "/api/plan/preview", []byte(tooLarge),
		"text/markdown"), http.StatusRequestEntityTooLarge)

	assertErrorEnvelope(t, srv.requestRaw(http.MethodPost, "/api/plan/preview",
		[]byte{'#', ' ', 'x', 0}, "text/markdown"), http.StatusBadRequest)
}

func TestPlanWeekFor(t *testing.T) {
	loc := lisbon(t)
	alphaFocus := "Alpha"
	weeks := []store.Week{
		{
			Code:      "A1",
			Phase:     "Build",
			StartDate: "2030-01-10",
			EndDate:   "2030-01-16",
			Focus:     &alphaFocus,
			SortOrder: 1,
		},
		{
			Code:      "A2",
			Phase:     "Launch",
			StartDate: "2030-01-17",
			EndDate:   "2030-01-23",
			SortOrder: 2,
		},
	}

	tests := []struct {
		name  string
		now   time.Time
		code  *string
		phase *string
		focus *string
		state string
	}{
		{name: "empty calendar", now: localTime(loc, 2030, 1, 12, 12, 0), state: "not_started"},
		{name: "before plan", now: localTime(loc, 2030, 1, 9, 12, 0), state: "not_started"},
		{
			name:  "first fabricated window",
			now:   localTime(loc, 2030, 1, 10, 12, 0),
			code:  stringPointer("A1"),
			phase: stringPointer("Build"),
			focus: stringPointer("Alpha"),
			state: "active",
		},
		{
			name:  "second fabricated window end",
			now:   localTime(loc, 2030, 1, 23, 23, 0),
			code:  stringPointer("A2"),
			phase: stringPointer("Launch"),
			state: "active",
		},
		{name: "after plan", now: localTime(loc, 2030, 1, 24, 0, 0), state: "plan_complete"},
		{
			name:  "lisbon date beats utc date",
			now:   localTime(loc, 2030, 1, 10, 0, 30),
			code:  stringPointer("A1"),
			phase: stringPointer("Build"),
			focus: stringPointer("Alpha"),
			state: "active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := weeks
			if tt.name == "empty calendar" {
				input = nil
			}

			got := PlanWeekFor(tt.now, loc, input)
			if got.State != tt.state {
				t.Fatalf("state = %q, want %q", got.State, tt.state)
			}
			switch {
			case tt.code == nil && got.Code != nil:
				t.Fatalf("code = %q, want nil", *got.Code)
			case tt.code != nil && got.Code == nil:
				t.Fatalf("code = nil, want %q", *tt.code)
			case tt.code != nil && got.Code != nil && *got.Code != *tt.code:
				t.Fatalf("code = %q, want %q", *got.Code, *tt.code)
			}
			if tt.phase != nil && (got.Phase == nil || *got.Phase != *tt.phase) {
				t.Fatalf("phase = %v, want %q", got.Phase, *tt.phase)
			}
			if tt.focus != nil && (got.Focus == nil || *got.Focus != *tt.focus) {
				t.Fatalf("focus = %v, want %q", got.Focus, *tt.focus)
			}
		})
	}
}

func TestStreakFor(t *testing.T) {
	loc := lisbon(t)
	now := localTime(loc, 2026, 9, 24, 12, 0)

	t.Run("empty today holds yesterday streak", func(t *testing.T) {
		got, err := StreakFor(
			[]string{
				localTime(loc, 2026, 9, 23, 20, 0).UTC().Format(time.RFC3339),
			},
			[]string{"2026-09-22"},
			now,
			loc,
		)
		if err != nil {
			t.Fatalf("streak: %v", err)
		}
		if got.Days != 2 || got.CountsToday {
			t.Fatalf("streak = %+v, want 2 days and counts_today=false", got)
		}
	})

	t.Run("two empty days breaks", func(t *testing.T) {
		got, err := StreakFor(nil, []string{"2026-09-22"}, now, loc)
		if err != nil {
			t.Fatalf("streak: %v", err)
		}
		if got.Days != 0 || got.CountsToday {
			t.Fatalf("streak = %+v, want zero", got)
		}
	})
}

type apiTest struct {
	handler http.Handler
	token   string
	store   *store.Store
}

type apiTestConfig struct {
	token     string
	publicURL string
	now       func() time.Time
	logger    *slog.Logger
	seeded    bool
}

func newTestAPI(t *testing.T, token string) apiTest {
	t.Helper()

	return newSeededTestAPI(t, token, true)
}

func newEmptyTestAPI(t *testing.T, token string) apiTest {
	t.Helper()

	return newSeededTestAPI(t, token, false)
}

func newSeededTestAPI(t *testing.T, token string, seeded bool) apiTest {
	t.Helper()

	return newSeededTestAPIWithConfig(t, apiTestConfig{
		token:     token,
		publicURL: "http://dash.test:8484",
		now:       func() time.Time { return apiNow },
		seeded:    seeded,
	})
}

func newSeededTestAPIWithConfig(t *testing.T, cfg apiTestConfig) apiTest {
	t.Helper()
	if cfg.publicURL == "" {
		cfg.publicURL = "http://dash.test:8484"
	}
	if cfg.now == nil {
		cfg.now = func() time.Time { return apiNow }
	}

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	db.SetClock(cfg.now)
	if err := db.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if cfg.seeded {
		doc, err := seed.Load()
		if err != nil {
			t.Fatalf("load seed: %v", err)
		}
		if _, err := seed.Apply(context.Background(), db.DB(), doc); err != nil {
			t.Fatalf("apply seed: %v", err)
		}
	}

	return apiTest{
		handler: NewMux(Config{
			Store:     db,
			Version:   "test-build",
			Token:     cfg.token,
			PublicURL: cfg.publicURL,
			Location:  lisbon(t),
			Now:       cfg.now,
			Logger:    cfg.logger,
		}),
		token: cfg.token,
		store: db,
	}
}

func (a apiTest) request(method, target string, body any) *httptest.ResponseRecorder {
	return a.requestWithHeaders(method, target, body, nil)
}

func (a apiTest) requestNoToken(method, target string, body any) *httptest.ResponseRecorder {
	return a.do(method, target, body, nil)
}

func (a apiTest) requestWithHeaders(
	method string,
	target string,
	body any,
	headers map[string]string,
) *httptest.ResponseRecorder {
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["X-Token"]; !ok && a.token != "" {
		headers["X-Token"] = a.token
	}
	return a.do(method, target, body, headers)
}

func (a apiTest) requestRaw(
	method string,
	target string,
	body []byte,
	contentType string,
) *httptest.ResponseRecorder {
	headers := map[string]string{"Content-Type": contentType}
	if a.token != "" {
		headers["X-Token"] = a.token
	}

	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func (a apiTest) do(method, target string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", jsonContentType)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, want, rec.Body.String())
	}
}

func assertErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	assertStatus(t, rec, want)

	var body map[string]any
	decodeBody(t, rec, &body)
	if _, ok := body["error"]; !ok {
		t.Fatalf("error envelope missing error field: %#v", body)
	}
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()

	if err := json.NewDecoder(rec.Body).Decode(dst); err != nil {
		t.Fatalf("decode body: %v; raw=%s", err, rec.Body.String())
	}
}

func idString(id int64) string {
	return strconvFormatInt(id)
}

func strconvFormatInt(id int64) string {
	return fmt.Sprintf("%d", id)
}

func lisbon(t *testing.T) *time.Location {
	t.Helper()

	loc, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		t.Fatalf("load Lisbon timezone: %v", err)
	}

	return loc
}

func localTime(loc *time.Location, year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, loc)
}

func uploadedPlanSource(id string) string {
	return fmt.Sprintf(`---
plan:
  id: %s
  name: "Uploaded Plan"
  start_year: 2030
  active_goal_limit: 2
---
## Log categories

| id | label | icon |
|---|---|---|
| shipped | Shipped | ok |

# Phase 1

## W1 · Jan 1-7 — Open

- [ ] **[ops] Ship the first slice**
`, id)
}

func apiTableCounts(t *testing.T, s *store.Store) map[string]int {
	t.Helper()

	tables := []string{
		"plan_meta", "plan_config", "plan_sources", "projects", "phases", "skill_tiers",
		"weeks", "rhythm", "categories", "metric_defs", "goals", "tasks", "log_entries",
		"daily_reviews", "metrics", "checkpoints", "skills", "task_skills", "log_skills",
		"xp_rules", "xp_events", "xp_event_skills", "achievements", "achievement_unlocks",
		"schema_migrations",
	}

	out := make(map[string]int, len(tables))
	for _, table := range tables {
		out[table] = apiCount(t, s, table)
	}
	return out
}

func apiCount(t *testing.T, s *store.Store, table string) int {
	t.Helper()

	var n int
	if err := s.DB().QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

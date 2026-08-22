package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	assertStatus(t, srv.request(http.MethodGet, "/api/categories", nil), http.StatusOK)
	assertStatus(t, srv.request(http.MethodGet, "/api/metric-defs", nil), http.StatusOK)
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

	createTask := srv.request(http.MethodPost, "/api/tasks", map[string]any{
		"week":       "W5",
		"title":      "Re-run k6 after the timeout change",
		"project":    "gateway",
		"goal_id":    movedGoal.Goal.ID,
		"steps":      []string{"Run baseline", "Record p95"},
		"done_means": "numbers are in the review",
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

	created := srv.request(http.MethodPost, "/api/tasks", map[string]any{
		"week":    "W5",
		"title":   "Toggle me",
		"project": "gateway",
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

func TestPlanWeekFor(t *testing.T) {
	loc := lisbon(t)

	tests := []struct {
		name  string
		now   time.Time
		code  *string
		state string
	}{
		{name: "before plan", now: localTime(loc, 2026, 8, 23, 12, 0), state: "not_started"},
		{name: "w1 start", now: localTime(loc, 2026, 8, 24, 12, 0), code: stringPointer("W1"), state: "active"},
		{name: "w12 end", now: localTime(loc, 2026, 11, 15, 23, 0), code: stringPointer("W12"), state: "active"},
		{name: "b1 start", now: localTime(loc, 2026, 11, 16, 0, 0), code: stringPointer("B1"), state: "active"},
		{name: "b7 end", now: localTime(loc, 2027, 2, 21, 23, 0), code: stringPointer("B7"), state: "active"},
		{name: "after plan", now: localTime(loc, 2027, 2, 22, 0, 0), state: "plan_complete"},
		{
			name:  "lisbon date beats utc date",
			now:   localTime(loc, 2026, 8, 24, 0, 30),
			code:  stringPointer("W1"),
			state: "active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanWeekFor(tt.now, loc)
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
}

func newTestAPI(t *testing.T, token string) apiTest {
	t.Helper()

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

	db.SetClock(func() time.Time { return apiNow })
	if err := db.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	doc, err := seed.Load()
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if _, err := seed.Apply(context.Background(), db.DB(), doc); err != nil {
		t.Fatalf("apply seed: %v", err)
	}

	return apiTest{
		handler: NewMux(Config{
			Store:    db,
			Version:  "test-build",
			Token:    token,
			Location: lisbon(t),
			Now:      func() time.Time { return apiNow },
		}),
		token: token,
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

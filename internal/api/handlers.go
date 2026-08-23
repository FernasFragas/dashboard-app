package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/store"
)

type goalsResponse struct {
	Goals       []store.Goal `json:"goals"`
	ActiveCount int          `json:"active_count"`
	ActiveLimit int          `json:"active_limit"`
}

type tasksResponse struct {
	Tasks []store.Task `json:"tasks"`
}

type projectsResponse struct {
	Projects []store.Project `json:"projects"`
}

type logsResponse struct {
	Entries    []store.LogEntry `json:"entries"`
	NextCursor *string          `json:"next_cursor"`
}

type logSummaryResponse struct {
	Range  string                `json:"range"`
	From   *string               `json:"from"`
	To     *string               `json:"to"`
	Counts []store.CategoryCount `json:"counts"`
	Total  int                   `json:"total"`
}

type dashboardResponse struct {
	Today string   `json:"today"`
	Week  PlanWeek `json:"week"`
	// TaskWeek is the week the task list below actually came from. It differs from Week before
	// the plan starts and after it ends, when there is no active week but the screen still
	// shows the first or last week's tasks. Clients that need a week to write to - the add-task
	// sheet - must use this, not Week.Code, or they are dead outside the plan window.
	//
	// It carries the dates and focus too, so a client can say *when* an upcoming week starts
	// without a second request.
	TaskWeek   *dashboardTaskWeek    `json:"task_week"`
	Rhythm     dashboardRhythm       `json:"rhythm"`
	Tasks      []store.Task          `json:"tasks"`
	Completion taskCompletion        `json:"completion"`
	Counters   []store.CategoryCount `json:"counters"`
	Streak     Streak                `json:"streak"`
}

// dashboardTaskWeek describes the week whose tasks are on screen, in any plan state.
type dashboardTaskWeek struct {
	Code      string  `json:"code"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Focus     *string `json:"focus"`
}

type dashboardRhythm struct {
	Label string `json:"label"`
	Slot  string `json:"slot"`
}

type taskCompletion struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "project"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	projects := splitCSV(r.URL.Query().Get("project"))
	if err := s.validateProjects(r.Context(), projects); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	today := localDate(s.now(), s.location)
	weeks, err := s.store.ListWeeks(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	plan := PlanWeekFor(s.now(), s.location, weeks)

	taskWeek := ""
	if plan.Code != nil {
		taskWeek = *plan.Code
	} else if plan.State == "plan_complete" {
		taskWeek = lastPlanWeekCode(weeks)
	} else {
		taskWeek = firstPlanWeekCode(weeks)
	}

	rhythm, err := s.store.RhythmForWeekday(r.Context(), isoWeekday(today))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.handleStoreError(w, err)
			return
		}
	}

	var tasks []store.Task
	var done, total int
	if taskWeek != "" {
		tasks, err = s.store.ListTasks(r.Context(), store.TaskFilter{Week: taskWeek, Projects: projects})
		if err != nil {
			s.handleStoreError(w, err)
			return
		}
		done, total, err = s.store.CountTasksByWeek(r.Context(), taskWeek)
		if err != nil {
			s.handleStoreError(w, err)
			return
		}
	}
	if tasks == nil {
		tasks = []store.Task{}
	}

	fromDate, toDate := currentWeekDateBounds(s.now(), s.location)
	fromBound, toBound, err := dateBounds(fromDate, toDate, s.location)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	counters, err := s.store.SummarizeLogs(r.Context(), fromBound, toBound)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if counters == nil {
		counters = []store.CategoryCount{}
	}

	logDays, err := s.store.LogDays(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	reviewDates, err := s.store.ReviewDates(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	streak, err := StreakFor(logDays, reviewDates, s.now(), s.location)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	var taskWeekOut *dashboardTaskWeek
	for _, week := range weeks {
		if week.Code == taskWeek {
			taskWeekOut = &dashboardTaskWeek{
				Code:      week.Code,
				StartDate: week.StartDate,
				EndDate:   week.EndDate,
				Focus:     week.Focus,
			}
			break
		}
	}

	s.writeJSON(w, http.StatusOK, dashboardResponse{
		Today:    today.Format(dateLayout),
		Week:     plan,
		TaskWeek: taskWeekOut,
		Rhythm: dashboardRhythm{
			Label: rhythm.Label,
			Slot:  rhythm.Slot,
		},
		Tasks:      tasks,
		Completion: taskCompletion{Done: done, Total: total},
		Counters:   counters,
		Streak:     streak,
	})
}

func (s *Server) listGoals(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "status", "project"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status := r.URL.Query().Get("status")
	if status != "" {
		if err := validateOneOf(status, "status", allowedGoalStatuses); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	projects := splitCSV(r.URL.Query().Get("project"))
	if err := s.validateProjects(r.Context(), projects); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	goals, err := s.store.ListGoals(r.Context(), store.GoalFilter{Status: status, Projects: projects})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if goals == nil {
		goals = []store.Goal{}
	}

	activeCount, err := s.store.CountGoalsByStatus(r.Context(), "active")
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	config, err := s.store.PlanConfig(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, goalsResponse{
		Goals:       goals,
		ActiveCount: activeCount,
		ActiveLimit: config.ActiveGoalLimit,
	})
}

func (s *Server) createGoal(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     string  `json:"title"`
		DoneMeans *string `json:"done_means"`
		Project   string  `json:"project"`
		Phase     *string `json:"phase"`
		Status    *string `json:"status"`
		Target    *string `json:"target"`
	}

	if err := decodeJSON(r, &in); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(in.Title)
	if err := requireNonBlank(title, "title"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := requireNonBlank(in.Project, "project"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validateProjectID(r.Context(), in.Project); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status := "backlog"
	if in.Status != nil {
		status = *in.Status
		if err := validateOneOf(status, "status", allowedGoalStatuses); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if in.Phase != nil {
		if err := s.validatePhaseID(r.Context(), *in.Phase); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	goal, err := s.store.CreateGoal(r.Context(), store.NewGoal{
		Title:     title,
		DoneMeans: optionalText(in.DoneMeans),
		Project:   in.Project,
		Phase:     in.Phase,
		Status:    status,
		Target:    optionalText(in.Target),
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/goals/%d", goal.ID))
	w.Header().Set("ETag", etag(goal.ID, goal.Version))
	s.writeJSON(w, http.StatusCreated, goal)
}

func (s *Server) getGoal(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	goal, err := s.store.GetGoal(r.Context(), id)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("ETag", etag(goal.ID, goal.Version))
	s.writeJSON(w, http.StatusOK, goal)
}

func (s *Server) patchGoal(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	version, err := parseIfMatch(r.Header.Get("If-Match"), id)
	if err != nil {
		var precondition preconditionError
		if errors.As(err, &precondition) {
			s.writeError(w, precondition.status, precondition.message)
			return
		}
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var in struct {
		Title     *string `json:"title"`
		DoneMeans *string `json:"done_means"`
		Project   *string `json:"project"`
		Target    *string `json:"target"`
		Status    *string `json:"status"`
		Position  *int    `json:"position"`
	}

	if err := decodeJSON(r, &in); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	patch := store.GoalPatch{
		DoneMeans: optionalText(in.DoneMeans),
		Target:    optionalText(in.Target),
		Status:    in.Status,
		Position:  in.Position,
	}

	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if err := requireNonBlank(title, "title"); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		patch.Title = &title
	}

	if in.Project != nil {
		if err := s.validateProjectID(r.Context(), *in.Project); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		patch.Project = in.Project
	}

	if in.Status != nil {
		if err := validateOneOf(*in.Status, "status", allowedGoalStatuses); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if (in.Status == nil) != (in.Position == nil) {
		s.writeError(w, http.StatusBadRequest, "status and position must be sent together")
		return
	}
	if in.Position != nil && *in.Position < 0 {
		s.writeError(w, http.StatusBadRequest, "position must be non-negative")
		return
	}

	goal, reordered, err := s.store.UpdateGoal(r.Context(), id, version, patch)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			current, getErr := s.store.GetGoal(r.Context(), id)
			if getErr != nil {
				s.handleStoreError(w, getErr)
				return
			}
			s.writeConflictCurrent(w, "goal was modified elsewhere", current)
			return
		}
		s.handleStoreError(w, err)
		return
	}
	if reordered == nil {
		reordered = []store.Goal{}
	}

	game, err := s.gameEnvelope(r.Context(), func(ctx context.Context) (store.GameAward, error) {
		return s.store.AwardGoalDoneXP(ctx, goal.ID)
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("ETag", etag(goal.ID, goal.Version))
	s.writeJSON(w, http.StatusOK, goalWriteResponse{Goal: goal, Reordered: reordered, Game: game})
}

func (s *Server) deleteGoal(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.DeleteGoal(r.Context(), id); err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeNoContent(w)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "week", "project"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	week := r.URL.Query().Get("week")
	projects := splitCSV(r.URL.Query().Get("project"))
	if err := s.validateProjects(r.Context(), projects); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tasks, err := s.store.ListTasks(r.Context(), store.TaskFilter{Week: week, Projects: projects})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if tasks == nil {
		tasks = []store.Task{}
	}

	s.writeJSON(w, http.StatusOK, tasksResponse{Tasks: tasks})
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Week      string   `json:"week"`
		Title     string   `json:"title"`
		Project   string   `json:"project"`
		GoalID    *int64   `json:"goal_id"`
		Steps     []string `json:"steps"`
		DoneMeans *string  `json:"done_means"`
		SkillIDs  []int64  `json:"skill_ids"`
	}

	if err := decodeJSON(r, &in); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if status, message := s.validateSkillIDs(r, in.SkillIDs, true); status != 0 {
		s.writeError(w, status, message)
		return
	}

	title := strings.TrimSpace(in.Title)
	if err := requireNonBlank(in.Week, "week"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := requireNonBlank(title, "title"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := requireNonBlank(in.Project, "project"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validateProjectID(r.Context(), in.Project); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.GoalID != nil && *in.GoalID <= 0 {
		s.writeError(w, http.StatusBadRequest, "goal_id must be positive")
		return
	}
	if _, err := s.store.GetWeek(r.Context(), in.Week); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.writeError(w, http.StatusBadRequest, "invalid week")
			return
		}
		s.handleStoreError(w, err)
		return
	}

	task, err := s.store.CreateTask(r.Context(), store.NewTask{
		Week:      in.Week,
		Title:     title,
		Project:   in.Project,
		GoalID:    in.GoalID,
		Steps:     in.Steps,
		DoneMeans: optionalText(in.DoneMeans),
		SkillIDs:  in.SkillIDs,
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/tasks/%d", task.ID))
	w.Header().Set("ETag", etag(task.ID, task.Version))
	s.writeJSON(w, http.StatusCreated, task)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := s.store.GetTask(r.Context(), id)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("ETag", etag(task.ID, task.Version))
	s.writeJSON(w, http.StatusOK, task)
}

func (s *Server) patchTask(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	version, err := parseIfMatch(r.Header.Get("If-Match"), id)
	if err != nil {
		var precondition preconditionError
		if errors.As(err, &precondition) {
			s.writeError(w, precondition.status, precondition.message)
			return
		}
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	patch, err := decodeTaskPatch(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if patch.Project != nil {
		if err := s.validateProjectID(r.Context(), *patch.Project); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if patch.SkillIDs != nil {
		if status, message := s.validateSkillIDs(r, *patch.SkillIDs, true); status != 0 {
			s.writeError(w, status, message)
			return
		}
	}

	task, err := s.store.UpdateTask(r.Context(), id, version, patch)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			current, getErr := s.store.GetTask(r.Context(), id)
			if getErr != nil {
				s.handleStoreError(w, getErr)
				return
			}
			s.writeConflictCurrent(w, "task was modified elsewhere", current)
			return
		}
		s.handleStoreError(w, err)
		return
	}

	game, err := s.gameEnvelope(r.Context(), func(ctx context.Context) (store.GameAward, error) {
		return s.store.SyncTaskXP(ctx, task.ID)
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("ETag", etag(task.ID, task.Version))
	s.writeJSON(w, http.StatusOK, taskWriteResponse{Task: task, Game: game})
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.DeleteTask(r.Context(), id); err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeNoContent(w)
}

func (s *Server) listLogs(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "category", "from", "to", "limit", "cursor"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	categories := splitCSV(r.URL.Query().Get("category"))
	for _, category := range categories {
		if err := s.validateCategoryID(r.Context(), category); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	from, to, err := dateBounds(r.URL.Query().Get("from"), r.URL.Query().Get("to"), s.location)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit, err := parsePositiveLimit(r.URL.Query().Get("limit"), 200, 500)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	cursor := r.URL.Query().Get("cursor")
	if cursor != "" {
		if _, err := time.Parse(time.RFC3339, cursor); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
	}

	entries, err := s.store.ListLogEntries(r.Context(), store.LogFilter{
		Categories: categories,
		From:       from,
		To:         to,
		Cursor:     cursor,
		Limit:      limit,
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if entries == nil {
		entries = []store.LogEntry{}
	}

	var nextCursor *string
	if len(entries) == limit {
		last := entries[len(entries)-1].OccurredAt
		nextCursor = &last
	}

	s.writeJSON(w, http.StatusOK, logsResponse{Entries: entries, NextCursor: nextCursor})
}

func (s *Server) createLog(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CategoryID string  `json:"category_id"`
		Title      string  `json:"title"`
		Note       *string `json:"note"`
		URL        *string `json:"url"`
		GoalID     *int64  `json:"goal_id"`
		OccurredAt *string `json:"occurred_at"`
		SkillIDs   []int64 `json:"skill_ids"`
	}

	if err := decodeJSON(r, &in); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(in.Title)
	if err := requireNonBlank(in.CategoryID, "category_id"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := requireNonBlank(title, "title"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validateCategoryID(r.Context(), in.CategoryID); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.GoalID != nil && *in.GoalID <= 0 {
		s.writeError(w, http.StatusBadRequest, "goal_id must be positive")
		return
	}
	if in.URL != nil {
		if err := validateHTTPURL(*in.URL); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	occurredAt := ""
	if in.OccurredAt != nil {
		if _, err := time.Parse(time.RFC3339, *in.OccurredAt); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid occurred_at")
			return
		}
		occurredAt = *in.OccurredAt
	}

	if status, message := s.validateSkillIDs(r, in.SkillIDs, false); status != 0 {
		s.writeError(w, status, message)
		return
	}

	entry, err := s.store.CreateLogEntry(r.Context(), store.NewLogEntry{
		CategoryID: in.CategoryID,
		Title:      title,
		Note:       optionalText(in.Note),
		URL:        optionalText(in.URL),
		GoalID:     in.GoalID,
		OccurredAt: occurredAt,
		SkillIDs:   in.SkillIDs,
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	game, err := s.gameEnvelope(r.Context(), func(ctx context.Context) (store.GameAward, error) {
		return s.store.AwardLogXP(ctx, entry.ID)
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/logs/%d", entry.ID))
	s.writeJSON(w, http.StatusCreated, logWriteResponse{LogEntry: entry, Game: game})
}

func (s *Server) deleteLog(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.DeleteLogEntry(r.Context(), id); err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeNoContent(w)
}

func (s *Server) logSummary(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "range"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	selectedRange := r.URL.Query().Get("range")
	if selectedRange == "" {
		selectedRange = "week"
	}
	if selectedRange != "week" && selectedRange != "all" {
		s.writeError(w, http.StatusBadRequest, "range must be week or all")
		return
	}

	var fromDate *string
	var toDate *string
	var fromBound string
	var toBound string

	if selectedRange == "week" {
		from, to := currentWeekDateBounds(s.now(), s.location)
		fromDate = &from
		toDate = &to
		var err error
		fromBound, toBound, err = dateBounds(from, to, s.location)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	counts, err := s.store.SummarizeLogs(r.Context(), fromBound, toBound)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if counts == nil {
		counts = []store.CategoryCount{}
	}

	total := 0
	for _, count := range counts {
		total += count.Count
	}

	s.writeJSON(w, http.StatusOK, logSummaryResponse{
		Range:  selectedRange,
		From:   fromDate,
		To:     toDate,
		Counts: counts,
		Total:  total,
	})
}

func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	categories, err := s.store.ListCategories(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if categories == nil {
		categories = []store.Category{}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}

	s.writeJSON(w, http.StatusOK, projectsResponse{Projects: projects})
}

func (s *Server) listReviews(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "from", "to"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" && to == "" {
		today := localDate(s.now(), s.location)
		from = today.AddDate(0, 0, -29).Format(dateLayout)
		to = today.Format(dateLayout)
	}
	if _, _, err := dateBounds(from, to, s.location); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reviews, err := s.store.ListReviews(r.Context(), from, to)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if reviews == nil {
		reviews = []store.DailyReview{}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (s *Server) upsertReview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date    *string `json:"date"`
		Learned *string `json:"learned"`
		Issue   *string `json:"issue"`
		Next    *string `json:"next"`
		Minutes *int    `json:"minutes"`
	}

	if err := decodeJSON(r, &in); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	date := localDate(s.now(), s.location).Format(dateLayout)
	if in.Date != nil {
		if _, err := parseLocalDate(*in.Date, s.location); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid date")
			return
		}
		date = *in.Date
	}
	if date > localDate(s.now(), s.location).Format(dateLayout) {
		s.writeError(w, http.StatusBadRequest, "date cannot be in the future")
		return
	}
	if in.Minutes != nil && *in.Minutes < 0 {
		s.writeError(w, http.StatusBadRequest, "minutes must be non-negative")
		return
	}

	review := store.DailyReview{
		Date:    date,
		Learned: optionalText(in.Learned),
		Issue:   optionalText(in.Issue),
		Next:    optionalText(in.Next),
		Minutes: in.Minutes,
	}
	if review.Learned == nil && review.Issue == nil && review.Next == nil {
		s.writeError(w, http.StatusBadRequest, "at least one review field is required")
		return
	}

	out, err := s.store.UpsertReview(r.Context(), review)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	game, err := s.gameEnvelope(r.Context(), func(ctx context.Context) (store.GameAward, error) {
		return s.store.AwardReviewXP(ctx, out.Date)
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, reviewWriteResponse{DailyReview: out, Game: game})
}

func (s *Server) listMetrics(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r, "name", "from", "to", "limit"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	from, to, err := dateBounds(r.URL.Query().Get("from"), r.URL.Query().Get("to"), s.location)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := parsePositiveLimit(r.URL.Query().Get("limit"), 100, 0)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	metrics, err := s.store.ListMetrics(r.Context(), store.MetricFilter{
		Name:  r.URL.Query().Get("name"),
		From:  from,
		To:    to,
		Limit: limit,
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if metrics == nil {
		metrics = []store.Metric{}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{"metrics": metrics})
}

func (s *Server) createMetric(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name       string   `json:"name"`
		Value      *float64 `json:"value"`
		Unit       *string  `json:"unit"`
		Note       *string  `json:"note"`
		RecordedAt *string  `json:"recorded_at"`
	}

	if err := decodeJSON(r, &in); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(in.Name)
	if err := requireNonBlank(name, "name"); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Value == nil {
		s.writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	recordedAt := ""
	if in.RecordedAt != nil {
		if _, err := time.Parse(time.RFC3339, *in.RecordedAt); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid recorded_at")
			return
		}
		recordedAt = *in.RecordedAt
	}

	metric, err := s.store.CreateMetric(r.Context(), store.NewMetric{
		Name:       name,
		Value:      *in.Value,
		Unit:       optionalText(in.Unit),
		Note:       optionalText(in.Note),
		RecordedAt: recordedAt,
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	game, err := s.gameEnvelope(r.Context(), func(ctx context.Context) (store.GameAward, error) {
		return s.store.AwardMetricXP(ctx, metric.ID)
	})
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/metrics/%d", metric.ID))
	s.writeJSON(w, http.StatusCreated, metricWriteResponse{Metric: metric, Game: game})
}

func (s *Server) metricDefs(w http.ResponseWriter, r *http.Request) {
	defs, err := s.store.ListMetricDefs(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if defs == nil {
		defs = []store.MetricDef{}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{"metric_defs": defs})
}

func (s *Server) getCheckpoint(w http.ResponseWriter, r *http.Request) {
	week := r.PathValue("week")
	checkpoint, err := s.store.GetCheckpoint(r.Context(), week)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	game, err := s.gameEnvelope(r.Context(), nil)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, checkpointWriteResponse{Checkpoint: checkpoint, Game: game})
}

func (s *Server) putCheckpoint(w http.ResponseWriter, r *http.Request) {
	week := r.PathValue("week")

	var in struct {
		Answers []string `json:"answers"`
	}
	if err := decodeJSON(r, &in); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.GetCheckpoint(r.Context(), week)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if len(in.Answers) != len(existing.Questions) {
		s.writeError(w, http.StatusBadRequest, "answers length must match questions length")
		return
	}

	checkpoint, err := s.store.SaveCheckpointAnswers(r.Context(), week, in.Answers)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, checkpoint)
}

func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Export(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	filename := "dashboard-export-" + s.now().In(s.location).Format("20060102") + ".json"
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	s.writeJSON(w, http.StatusOK, data)
}

func decodeTaskPatch(r *http.Request) (store.TaskPatch, error) {
	raw, err := decodeRawObject(r)
	if err != nil {
		return store.TaskPatch{}, err
	}

	allowed := map[string]struct{}{
		"title": {}, "project": {}, "goal_id": {}, "status": {}, "done_means": {},
		"skill_ids": {},
	}

	var patch store.TaskPatch
	for key, value := range raw {
		if _, ok := allowed[key]; !ok {
			return store.TaskPatch{}, fmt.Errorf("json: unknown field %q", key)
		}

		switch key {
		case "title":
			var title string
			if err := json.Unmarshal(value, &title); err != nil {
				return store.TaskPatch{}, fmt.Errorf("title must be a string")
			}
			title = strings.TrimSpace(title)
			if err := requireNonBlank(title, "title"); err != nil {
				return store.TaskPatch{}, err
			}
			patch.Title = &title
		case "project":
			var project string
			if err := json.Unmarshal(value, &project); err != nil {
				return store.TaskPatch{}, fmt.Errorf("project must be a string")
			}
			project = strings.TrimSpace(project)
			if err := requireNonBlank(project, "project"); err != nil {
				return store.TaskPatch{}, err
			}
			patch.Project = &project
		case "goal_id":
			if string(value) == "null" {
				var cleared *int64
				patch.GoalID = &cleared
				continue
			}
			var goalID int64
			if err := json.Unmarshal(value, &goalID); err != nil {
				return store.TaskPatch{}, fmt.Errorf("goal_id must be an integer or null")
			}
			if goalID <= 0 {
				return store.TaskPatch{}, fmt.Errorf("goal_id must be positive")
			}
			patch.GoalID = int64PointerPointer(goalID)
		case "status":
			var status string
			if err := json.Unmarshal(value, &status); err != nil {
				return store.TaskPatch{}, fmt.Errorf("status must be a string")
			}
			if err := validateOneOf(status, "status", allowedTaskStatuses); err != nil {
				return store.TaskPatch{}, err
			}
			patch.Status = &status
		case "done_means":
			if string(value) == "null" {
				continue
			}
			var doneMeans string
			if err := json.Unmarshal(value, &doneMeans); err != nil {
				return store.TaskPatch{}, fmt.Errorf("done_means must be a string")
			}
			patch.DoneMeans = stringPointer(strings.TrimSpace(doneMeans))
		case "skill_ids":
			// Absent means "leave the links alone". Present-but-empty is a deliberate attempt to
			// strip every skill, which the invariant forbids - the handler turns it into a 422.
			var skillIDs []int64
			if err := json.Unmarshal(value, &skillIDs); err != nil {
				return store.TaskPatch{}, fmt.Errorf("skill_ids must be an array of integers")
			}
			patch.SkillIDs = &skillIDs
		}
	}

	return patch, nil
}

func decodeRawObject(r *http.Request) (map[string]json.RawMessage, error) {
	defer func() { _ = r.Body.Close() }()

	var raw map[string]json.RawMessage
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("request body must be a JSON object")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("request body must contain a single JSON object")
	}

	return raw, nil
}

func isoWeekday(day time.Time) int {
	weekday := int(day.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func validateHTTPURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must use http or https")
	}

	return nil
}

func int64PointerPointer(value int64) **int64 {
	ptr := &value
	return &ptr
}

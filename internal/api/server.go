// Package api contains HTTP handlers for the dashboard server.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/store"
)

const jsonContentType = "application/json; charset=utf-8"

// Config wires the API server.
type Config struct {
	Store     *store.Store
	Version   string
	Token     string
	PublicURL string
	Location  *time.Location
	Now       func() time.Time
	Logger    *slog.Logger
}

// Server owns the HTTP boundary: routing, middleware, validation, and JSON encoding.
type Server struct {
	store     *store.Store
	version   string
	token     string
	publicURL string
	location  *time.Location
	now       func() time.Time
	logger    *slog.Logger

	previewMu     sync.Mutex
	previewedPlan map[string]struct{}
	pairLimiter   *pairRateLimiter
}

// HealthResponse is returned by GET /api/health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// NewMux wires every API route behind the required middleware chain.
func NewMux(cfg Config) http.Handler {
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.PublicURL == "" {
		cfg.PublicURL = "http://localhost:8484"
	}

	s := &Server{
		store:     cfg.Store,
		version:   cfg.Version,
		token:     cfg.Token,
		publicURL: strings.TrimRight(cfg.PublicURL, "/"),
		location:  cfg.Location,
		now:       cfg.Now,
		logger:    cfg.Logger,

		previewedPlan: map[string]struct{}{},
		pairLimiter:   newPairRateLimiter(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/pair", s.createPair)
	mux.HandleFunc("GET /api/pair/{code}", s.pairSVG)
	mux.HandleFunc("GET /api/dashboard", s.dashboard)
	mux.HandleFunc("GET /api/goals", s.listGoals)
	mux.HandleFunc("POST /api/goals", s.createGoal)
	mux.HandleFunc("GET /api/goals/{id}", s.getGoal)
	mux.HandleFunc("PATCH /api/goals/{id}", s.patchGoal)
	mux.HandleFunc("DELETE /api/goals/{id}", s.deleteGoal)
	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("POST /api/tasks", s.createTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.getTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", s.patchTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.deleteTask)
	mux.HandleFunc("GET /api/logs", s.listLogs)
	mux.HandleFunc("POST /api/logs", s.createLog)
	mux.HandleFunc("GET /api/logs/summary", s.logSummary)
	mux.HandleFunc("DELETE /api/logs/{id}", s.deleteLog)
	mux.HandleFunc("GET /api/game/profile", s.gameProfile)
	mux.HandleFunc("GET /api/game/skills", s.gameSkills)
	mux.HandleFunc("GET /api/game/events", s.gameEvents)
	mux.HandleFunc("GET /api/skills", s.listSkills)
	mux.HandleFunc("GET /api/skills/{id}", s.getSkill)
	mux.HandleFunc("GET /api/projects", s.projects)
	mux.HandleFunc("GET /api/categories", s.categories)
	mux.HandleFunc("GET /api/plan", s.getPlan)
	mux.HandleFunc("POST /api/plan/preview", s.previewPlan)
	mux.HandleFunc("POST /api/plan/apply", s.applyPlan)
	mux.HandleFunc("GET /api/reviews", s.listReviews)
	mux.HandleFunc("POST /api/reviews", s.upsertReview)
	mux.HandleFunc("GET /api/metrics", s.listMetrics)
	mux.HandleFunc("POST /api/metrics", s.createMetric)
	mux.HandleFunc("GET /api/metric-defs", s.metricDefs)
	mux.HandleFunc("GET /api/checkpoints/{week}", s.getCheckpoint)
	mux.HandleFunc("PUT /api/checkpoints/{week}", s.putCheckpoint)
	mux.HandleFunc("GET /api/export", s.export)
	mux.HandleFunc("GET /pair/{code}", s.redeemPair)

	return s.recover(s.requestLog(s.tokenAuth(mux)))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, HealthResponse{Status: "ok", Version: s.version})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				s.logger.Error("panic in api handler",
					"method", r.Method, "path", r.URL.Path, "panic", p, "stack", string(debug.Stack()))
				s.writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(lw, r)

		s.logger.Info("request",
			"method", r.Method,
			"path", redactedRequestPath(r.URL.Path),
			"status", lw.status,
			"duration", time.Since(start),
			"bytes", lw.bytes,
		)
	})
}

func redactedRequestPath(path string) string {
	if strings.HasPrefix(path, "/pair/") {
		return "/pair/<redacted>"
	}
	if strings.HasPrefix(path, "/api/pair/") {
		return "/api/pair/<redacted>"
	}
	return path
}

func (s *Server) tokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.Method == http.MethodGet && r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("X-Token") != s.token {
			s.writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		s.logger.Error("encode response", "error", err)
	}
}

func (s *Server) writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) writeConflictCurrent(w http.ResponseWriter, message string, current any) {
	s.writeJSON(w, http.StatusPreconditionFailed, map[string]any{
		"error":   message,
		"current": current,
	})
}

func (s *Server) handleStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrConstraint):
		s.writeError(w, http.StatusBadRequest, "constraint violation")
	case errors.Is(err, store.ErrConflict):
		s.writeError(w, http.StatusConflict, "conflict")
	default:
		s.logger.Error("api handler failed", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func decodeJSON(r *http.Request, dst any) error {
	defer func() { _ = r.Body.Close() }()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func requireKnownQuery(r *http.Request, allowed ...string) error {
	seen := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		seen[name] = true
	}

	for name := range r.URL.Query() {
		if !seen[name] {
			return fmt.Errorf("unknown query parameter %q", name)
		}
	}

	return nil
}

func pathID(r *http.Request) (int64, error) {
	raw := r.PathValue("id")
	if raw == "" {
		return 0, errors.New("missing id")
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id %q", raw)
	}

	return id, nil
}

func parsePositiveLimit(raw string, defaultValue int, maxValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid limit %q", raw)
	}
	if maxValue > 0 && n > maxValue {
		return maxValue, nil
	}

	return n, nil
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			out = append(out, value)
		}
	}

	return out
}

func stringPointer(value string) *string {
	return &value
}

func optionalText(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func requireNonBlank(value string, field string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func validateOneOf(value string, field string, allowed map[string]struct{}) error {
	if _, ok := allowed[value]; !ok {
		return fmt.Errorf("invalid %s %q", field, value)
	}
	return nil
}

func (s *Server) validateProjectID(ctx context.Context, project string) error {
	ok, err := s.store.ProjectExists(ctx, project)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid project %q", project)
	}
	return nil
}

func (s *Server) validateProjects(ctx context.Context, projects []string) error {
	for _, project := range projects {
		if err := s.validateProjectID(ctx, project); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) validatePhaseID(ctx context.Context, phase string) error {
	ok, err := s.store.PhaseExists(ctx, phase)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid phase %q", phase)
	}
	return nil
}

func (s *Server) validateCategoryID(ctx context.Context, category string) error {
	ok, err := s.store.CategoryExists(ctx, category)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid category %q", category)
	}
	return nil
}

var (
	allowedGoalStatuses = map[string]struct{}{"backlog": {}, "active": {}, "done": {}}
	allowedTaskStatuses = map[string]struct{}{"todo": {}, "done": {}}
)

package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FernasFragas/dashboard-app/internal/plan"
	"github.com/FernasFragas/dashboard-app/internal/seed"
	"github.com/FernasFragas/dashboard-app/internal/store"
)

const (
	maxPlanSourceBytes  = 1 << 20
	maxPlanApplyBytes   = maxPlanSourceBytes + 64*1024
	planParseTimeout    = 2 * time.Second
	planMultipartMemory = 256 << 10
)

var planLineErrorRe = regexp.MustCompile(`(?:^|: )line (\d+):\s*(.*)$`)

type planIdentity struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LoadedAt string `json:"loaded_at,omitempty"`
}

type planStatusResponse struct {
	Plan   *planIdentity     `json:"plan"`
	Source *store.PlanSource `json:"source"`
	Counts store.PlanCounts  `json:"counts"`
}

type planPreviewResponse struct {
	SourceSHA256 string             `json:"source_sha256,omitempty"`
	Plan         *planIdentity      `json:"plan,omitempty"`
	CurrentPlan  *planIdentity      `json:"current_plan,omitempty"`
	Mode         string             `json:"mode,omitempty"`
	Parsed       store.PlanCounts   `json:"parsed,omitempty"`
	Changes      store.PlanDiff     `json:"changes,omitempty"`
	AtRisk       store.PlanAtRisk   `json:"at_risk,omitempty"`
	Errors       []planPreviewError `json:"errors"`
}

type planPreviewError struct {
	Line    *int    `json:"line,omitempty"`
	Message string  `json:"message"`
	Excerpt *string `json:"excerpt,omitempty"`
}

type applyPlanRequest struct {
	Source          string `json:"source"`
	SourceSHA256    string `json:"source_sha256"`
	ConfirmPlanName string `json:"confirm_plan_name"`
}

type applyPlanResponse struct {
	Plan       planIdentity     `json:"plan"`
	Mode       string           `json:"mode"`
	Counts     store.PlanCounts `json:"counts"`
	Seed       seed.Result      `json:"seed"`
	Source     store.PlanSource `json:"source"`
	BackupPath *string          `json:"backup_path"`
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	current, err := s.store.CurrentPlan(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	counts, err := s.store.PlanCounts(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	var source *store.PlanSource
	if current != nil {
		source, err = s.store.LatestPlanSource(r.Context(), current.ID)
		if err != nil {
			s.handleStoreError(w, err)
			return
		}
	}

	s.writeJSON(w, http.StatusOK, planStatusResponse{
		Plan:   planIdentityFromMeta(current),
		Source: source,
		Counts: counts,
	})
}

func (s *Server) previewPlan(w http.ResponseWriter, r *http.Request) {
	source, ok := s.readUploadedPlan(w, r)
	if !ok {
		return
	}

	doc, parseErr := parsePlanWithTimeout(r.Context(), source)
	if parseErr != nil {
		s.writeJSON(w, http.StatusOK, planPreviewResponse{
			Errors: []planPreviewError{planError(source, parseErr)},
		})
		return
	}

	response, err := s.planPreviewForDocument(r.Context(), source, doc)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.rememberPlanPreview(response.SourceSHA256)
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) applyPlan(w http.ResponseWriter, r *http.Request) {
	req, ok := s.decodeApplyPlan(w, r)
	if !ok {
		return
	}

	if req.Source == "" {
		s.writeError(w, http.StatusBadRequest, "plan source is required")
		return
	}
	if !validPlanSource(req.Source) {
		s.writeError(w, http.StatusBadRequest, "plan must be UTF-8 markdown without NUL bytes")
		return
	}

	sha := sourceSHA256(req.Source)
	if sha != req.SourceSHA256 {
		s.writeError(w, http.StatusConflict, "source_sha256 does not match plan source")
		return
	}
	if !s.planWasPreviewed(sha) {
		s.writeError(w, http.StatusConflict, "plan source must be previewed before apply")
		return
	}

	doc, parseErr := parsePlanWithTimeout(r.Context(), req.Source)
	if parseErr != nil {
		s.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors": []planPreviewError{planError(req.Source, parseErr)},
		})
		return
	}

	current, err := s.store.CurrentPlan(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	mode := planMode(current, doc.Plan)
	if mode == "replace" && req.ConfirmPlanName != doc.Plan.Name {
		s.writeError(w, http.StatusUnprocessableEntity, "confirm_plan_name must match the new plan name")
		return
	}

	var backupPath *string
	if mode == "replace" {
		path, err := s.store.SnapshotBeforePlanReplace(r.Context(), doc.Plan.ID)
		if err != nil {
			s.handleStoreError(w, err)
			return
		}
		backupPath = &path
	}

	var result seed.Result
	switch mode {
	case "replace":
		result, err = seed.Replace(r.Context(), s.store.DB(), doc)
	default:
		result, err = seed.Apply(r.Context(), s.store.DB(), doc)
	}
	if err != nil {
		if errors.Is(err, seed.ErrPlanMismatch) {
			s.writeError(w, http.StatusConflict, err.Error())
			return
		}
		s.handleStoreError(w, err)
		return
	}

	source, err := s.store.SavePlanSource(r.Context(), doc.Plan.ID, sha, req.Source)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	counts, err := s.store.PlanCounts(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.forgetPlanPreview(sha)
	s.writeJSON(w, http.StatusOK, applyPlanResponse{
		Plan: planIdentity{
			ID:       doc.Plan.ID,
			Name:     doc.Plan.Name,
			LoadedAt: source.LoadedAt,
		},
		Mode:       mode,
		Counts:     counts,
		Seed:       result,
		Source:     source,
		BackupPath: backupPath,
	})
}

func (s *Server) planPreviewForDocument(
	ctx context.Context,
	source string,
	doc seed.Document,
) (planPreviewResponse, error) {
	current, err := s.store.CurrentPlan(ctx)
	if err != nil {
		return planPreviewResponse{}, err
	}
	changes, err := s.store.DiffPlan(ctx, planFingerprints(doc))
	if err != nil {
		return planPreviewResponse{}, err
	}

	mode := planMode(current, doc.Plan)
	var atRisk store.PlanAtRisk
	if mode == "replace" {
		atRisk, err = s.store.PlanAtRiskForReplace(ctx, planCategoryIDs(doc))
		if err != nil {
			return planPreviewResponse{}, err
		}
	}

	return planPreviewResponse{
		SourceSHA256: sourceSHA256(source),
		Plan:         &planIdentity{ID: doc.Plan.ID, Name: doc.Plan.Name},
		CurrentPlan:  planIdentityFromMeta(current),
		Mode:         mode,
		Parsed: store.PlanCounts{
			Weeks:    len(doc.Weeks),
			Goals:    len(doc.Goals),
			Tasks:    len(doc.Tasks),
			Skills:   len(doc.Skills),
			Projects: len(doc.Projects),
		},
		Changes: changes,
		AtRisk:  atRisk,
		Errors:  []planPreviewError{},
	}, nil
}

func (s *Server) readUploadedPlan(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPlanSourceBytes)
	defer func() { _ = r.Body.Close() }()

	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var (
		body []byte
		err  error
	)

	if mediaType == "multipart/form-data" {
		body, err = readMultipartPlan(r)
	} else {
		body, err = io.ReadAll(r.Body)
	}

	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "plan is larger than 1 MB", http.StatusRequestEntityTooLarge)
			return "", false
		}
		s.writeError(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	if len(body) == 0 {
		s.writeError(w, http.StatusBadRequest, "plan source is required")
		return "", false
	}
	if bytes.Contains(body, []byte{0}) || !utf8.Valid(body) {
		s.writeError(w, http.StatusBadRequest, "plan must be UTF-8 markdown without NUL bytes")
		return "", false
	}

	return string(body), true
}

func readMultipartPlan(r *http.Request) ([]byte, error) {
	if err := r.ParseMultipartForm(planMultipartMemory); err != nil {
		return nil, err
	}

	for _, field := range []string{"file", "plan"} {
		file, _, err := r.FormFile(field)
		if err == nil {
			defer func() { _ = file.Close() }()
			return io.ReadAll(file)
		}
	}

	if values := r.MultipartForm.Value["source"]; len(values) > 0 {
		return []byte(values[0]), nil
	}

	return nil, errors.New("multipart upload needs a file, plan or source part")
}

func (s *Server) decodeApplyPlan(w http.ResponseWriter, r *http.Request) (applyPlanRequest, bool) {
	defer func() { _ = r.Body.Close() }()

	r.Body = http.MaxBytesReader(w, r.Body, maxPlanApplyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req applyPlanRequest
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "plan apply request is larger than 1 MB", http.StatusRequestEntityTooLarge)
			return applyPlanRequest{}, false
		}
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return applyPlanRequest{}, false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		s.writeError(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return applyPlanRequest{}, false
	}

	req.SourceSHA256 = strings.ToLower(strings.TrimSpace(req.SourceSHA256))
	req.ConfirmPlanName = strings.TrimSpace(req.ConfirmPlanName)
	return req, true
}

func parsePlanWithTimeout(ctx context.Context, source string) (seed.Document, error) {
	type result struct {
		doc seed.Document
		err error
	}

	ctx, cancel := context.WithTimeout(ctx, planParseTimeout)
	defer cancel()

	done := make(chan result, 1)
	go func() {
		doc, err := plan.ParseWithProfile(source, plan.DefaultProfile())
		done <- result{doc: doc, err: err}
	}()

	select {
	case res := <-done:
		return res.doc, res.err
	case <-ctx.Done():
		return seed.Document{}, fmt.Errorf("plan parse timed out")
	}
}

func planError(source string, err error) planPreviewError {
	out := planPreviewError{Message: err.Error()}
	m := planLineErrorRe.FindStringSubmatch(err.Error())
	if m == nil {
		return out
	}

	line, convErr := strconv.Atoi(m[1])
	if convErr != nil || line < 1 {
		return out
	}

	out.Line = &line
	lines := strings.Split(source, "\n")
	if line <= len(lines) {
		excerpt := lines[line-1]
		out.Excerpt = &excerpt
	}

	return out
}

func validPlanSource(source string) bool {
	return !strings.ContainsRune(source, '\x00') && utf8.ValidString(source)
}

func sourceSHA256(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func planMode(current *store.PlanMeta, next seed.Plan) string {
	if current == nil {
		return "initial"
	}
	if current.ID == next.ID {
		return "additive"
	}
	return "replace"
}

func planIdentityFromMeta(meta *store.PlanMeta) *planIdentity {
	if meta == nil {
		return nil
	}
	return &planIdentity{ID: meta.ID, Name: meta.Name, LoadedAt: meta.LoadedAt}
}

func planFingerprints(doc seed.Document) store.PlanFingerprints {
	return store.PlanFingerprints{
		Weeks: weekFingerprints(doc.Weeks),
		Tasks: taskFingerprints(doc.Tasks),
		Goals: goalFingerprints(doc.Goals),
	}
}

func weekFingerprints(weeks []seed.Week) map[string]string {
	out := map[string]string{}
	for _, week := range weeks {
		focus := ""
		if week.Focus != nil {
			focus = *week.Focus
		}
		out[week.Code] = fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d",
			week.Phase, week.StartDate, week.EndDate, focus, week.SortOrder)
	}
	return out
}

func taskFingerprints(tasks []seed.Task) map[string]string {
	out := map[string]string{}
	for _, task := range tasks {
		out[task.SeedKey] = task.Title
	}
	return out
}

func goalFingerprints(goals []seed.Goal) map[string]string {
	out := map[string]string{}
	for _, goal := range goals {
		doneMeans := ""
		if goal.DoneMeans != nil {
			doneMeans = *goal.DoneMeans
		}
		phase := ""
		if goal.Phase != nil {
			phase = *goal.Phase
		}
		target := ""
		if goal.Target != nil {
			target = *goal.Target
		}
		out[goal.Code] = fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%d",
			goal.Title, doneMeans, goal.Project, phase, target, goal.SortOrder)
	}
	return out
}

func planCategoryIDs(doc seed.Document) []string {
	out := make([]string, 0, len(doc.Categories))
	for _, category := range doc.Categories {
		out = append(out, category.ID)
	}
	return out
}

func (s *Server) rememberPlanPreview(hash string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()

	s.previewedPlan[hash] = struct{}{}
}

func (s *Server) planWasPreviewed(hash string) bool {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()

	_, ok := s.previewedPlan[hash]
	return ok
}

func (s *Server) forgetPlanPreview(hash string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()

	delete(s.previewedPlan, hash)
}

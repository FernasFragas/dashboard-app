package api

import (
	"net/http"

	"github.com/FernasFragas/dashboard-app/internal/store"
)

// GET /api/skills - the profile grid. Every statistic is derived from the associations at read
// time (ADR-007), so these numbers cannot disagree with the task list they summarise.
func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	if err := requireKnownQuery(r); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	skills, err := s.store.ListSkills(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	if skills == nil {
		skills = []store.Skill{}
	}

	// Tasks that predate M8 have no skill at all. Reporting them lets the UI show a
	// "needs skill" badge instead of quietly implying the invariant already holds.
	unlinked, err := s.store.UnlinkedTaskIDs(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	if unlinked == nil {
		unlinked = []int64{}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"skills":         skills,
		"unlinked_tasks": unlinked,
	})
}

// GET /api/skills/{id} - one skill, the tasks that build it, and the evidence citing it.
func (s *Server) getSkill(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	detail, err := s.store.GetSkillDetail(r.Context(), id)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	if detail.Tasks == nil {
		detail.Tasks = []store.Task{}
	}
	if detail.Evidence == nil {
		detail.Evidence = []store.LogEntry{}
	}

	s.writeJSON(w, http.StatusOK, detail)
}

// validateSkillIDs checks a submitted skill list against the catalogue.
//
// required=true is the task rule: every task builds at least one skill, and an empty list is a
// 422 rather than a 400 because the request is well-formed - it just asks for something the
// model forbids.
func (s *Server) validateSkillIDs(r *http.Request, ids []int64, required bool) (int, string) {
	if len(ids) == 0 {
		if required {
			return http.StatusUnprocessableEntity,
				"every task builds at least one skill - pick what this work trains"
		}
		return 0, ""
	}

	known, err := s.store.SkillIDsByCode(r.Context())
	if err != nil {
		return http.StatusInternalServerError, "internal server error"
	}

	valid := make(map[int64]bool, len(known))
	for _, id := range known {
		valid[id] = true
	}

	for _, id := range ids {
		if !valid[id] {
			return http.StatusBadRequest, "unknown skill id"
		}
	}

	return 0, ""
}

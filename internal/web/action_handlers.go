package web

import (
	"net/http"
	"strings"

	"subtitle-review/internal/domain"
	"subtitle-review/internal/workflow"
)

func (s *Server) ReviewHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	var body struct {
		ExpectedVersion       int64               `json:"expectedVersion"`
		Reviewer              string              `json:"reviewer"`
		LanguageApproved      bool                `json:"languageApproved"`
		AccessibilityApproved bool                `json:"accessibilityApproved"`
		Decision              domain.DecisionType `json:"decision"`
		Reason                string              `json:"reason"`
		IdempotencyKey        string              `json:"idempotencyKey"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	agg, err := s.workflow.Review(workflow.ReviewCommand{ProjectID: projectID, ExpectedVersion: body.ExpectedVersion, Reviewer: body.Reviewer, LanguageApproved: body.LanguageApproved, AccessibilityApproved: body.AccessibilityApproved, Decision: body.Decision, Reason: body.Reason, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, agg)
}

func (s *Server) FreezeHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	var body struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		IssuedBy        string `json:"issuedBy"`
		IdempotencyKey  string `json:"idempotencyKey"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	agg, err := s.workflow.Freeze(workflow.FreezeCommand{ProjectID: projectID, ExpectedVersion: body.ExpectedVersion, IssuedBy: body.IssuedBy, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, agg)
}

func (s *Server) VerifyCredentialHandler(w http.ResponseWriter, _ *http.Request, code string) {
	verification, err := s.workflow.VerifyCredential(strings.TrimSpace(code))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, verification)
}

func (s *Server) ReviewContextHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	context, err := s.workflow.ReviewContext(projectID, r.URL.Query().Get("reviewer"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, context)
}

func (s *Server) FreezeReadinessHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	expectedVersion, ok := queryInt(w, r, "expectedVersion", 0)
	if !ok {
		return
	}
	result, err := s.workflow.FreezeReadiness(projectID, int64(expectedVersion))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

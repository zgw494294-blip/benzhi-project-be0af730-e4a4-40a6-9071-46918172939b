package web

import (
	"net/http"
	"strconv"
	"strings"

	"subtitle-review/internal/domain"
	"subtitle-review/internal/workflow"
)

func (s *Server) ListProjectsHandler(w http.ResponseWriter, r *http.Request) {
	page, ok := queryInt(w, r, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := queryInt(w, r, "pageSize", 50)
	if !ok {
		return
	}
	keyword := r.URL.Query().Get("q")
	if keyword == "" {
		keyword = r.URL.Query().Get("keyword")
	}
	result, err := s.workflow.QueryProjects(domain.ProjectQueueQuery{Status: domain.ProjectStatus(strings.TrimSpace(r.URL.Query().Get("status"))), Language: r.URL.Query().Get("language"), Keyword: keyword, Page: page, PageSize: pageSize})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func queryInt(w http.ResponseWriter, r *http.Request, field string, fallback int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(field))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeDomainError(w, domain.FieldError(field, field+" 必须是整数"))
		return 0, false
	}
	return value, true
}

func (s *Server) CreateProjectHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.CreateProjectCommand
	if !decodeJSON(w, r, &cmd) {
		return
	}
	agg, err := s.workflow.CreateProject(cmd)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, agg)
}

func (s *Server) GetProjectHandler(w http.ResponseWriter, _ *http.Request, projectID string) {
	agg, err := s.workflow.GetProject(projectID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agg)
}

func (s *Server) SubmitRevisionHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	var body struct {
		ExpectedVersion int64               `json:"expectedVersion"`
		SubmittedBy     string              `json:"submittedBy"`
		Cues            []domain.CaptionCue `json:"cues"`
		IdempotencyKey  string              `json:"idempotencyKey"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	agg, err := s.workflow.SubmitRevision(workflow.SubmitRevisionCommand{ProjectID: projectID, ExpectedVersion: body.ExpectedVersion, SubmittedBy: body.SubmittedBy, Cues: body.Cues, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, agg)
}

func (s *Server) UpdateProjectRulesHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	var body struct {
		ExpectedVersion   int64   `json:"expectedVersion"`
		Title             string  `json:"title"`
		DurationMillis    int64   `json:"durationMillis"`
		Language          string  `json:"language"`
		FrameRate         float64 `json:"frameRate"`
		MaxCharsPerSecond float64 `json:"maxCharsPerSecond"`
		DeliveryStandard  string  `json:"deliveryStandard"`
		Actor             string  `json:"actor"`
		IdempotencyKey    string  `json:"idempotencyKey"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	agg, err := s.workflow.UpdateProjectRules(workflow.UpdateProjectRulesCommand{ProjectID: projectID, ExpectedVersion: body.ExpectedVersion, Title: body.Title, DurationMillis: body.DurationMillis, Language: body.Language, FrameRate: body.FrameRate, MaxCharsPerSecond: body.MaxCharsPerSecond, DeliveryStandard: body.DeliveryStandard, Actor: body.Actor, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agg)
}

func (s *Server) PreflightRevisionHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	var body struct {
		ExpectedVersion int64               `json:"expectedVersion"`
		SubmittedBy     string              `json:"submittedBy"`
		Cues            []domain.CaptionCue `json:"cues"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.workflow.PreflightRevision(workflow.PreflightRevisionCommand{ProjectID: projectID, ExpectedVersion: body.ExpectedVersion, SubmittedBy: body.SubmittedBy, Cues: body.Cues})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) RevisionDiffHandler(w http.ResponseWriter, r *http.Request, projectID, revisionID string) {
	result, err := s.workflow.RevisionDiff(projectID, revisionID, strings.TrimSpace(r.URL.Query().Get("parentRevisionID")))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) DispositionIssueHandler(w http.ResponseWriter, r *http.Request, projectID, issueID string) {
	var body struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		Disposition     string `json:"disposition"`
		Actor           string `json:"actor"`
		IdempotencyKey  string `json:"idempotencyKey"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	agg, err := s.workflow.DispositionIssue(workflow.DispositionIssueCommand{ProjectID: projectID, ExpectedVersion: body.ExpectedVersion, IssueID: issueID, Disposition: body.Disposition, Actor: body.Actor, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agg)
}

func (s *Server) BatchDispositionHandler(w http.ResponseWriter, r *http.Request, projectID string) {
	var body struct {
		ExpectedVersion int64                            `json:"expectedVersion"`
		Items           []workflow.IssueDispositionInput `json:"items"`
		Actor           string                           `json:"actor"`
		IdempotencyKey  string                           `json:"idempotencyKey"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.workflow.BatchDisposition(workflow.BatchDispositionCommand{ProjectID: projectID, ExpectedVersion: body.ExpectedVersion, Items: body.Items, Actor: body.Actor, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

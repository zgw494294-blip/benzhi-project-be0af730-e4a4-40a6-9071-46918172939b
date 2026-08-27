package workflow

import (
	"sort"
	"strings"

	"subtitle-review/internal/caption"
	"subtitle-review/internal/domain"
)

func (s *Service) ReviewContext(projectID, candidateReviewer string) (*ReviewContext, error) {
	agg, err := s.store.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	revisionNumbers := make(map[string]int, len(agg.Revisions))
	for _, revision := range agg.Revisions {
		revisionNumbers[revision.ID] = revision.RevisionNumber
	}
	context := &ReviewContext{ProjectID: projectID, History: make([]ReviewHistoryItem, 0, len(agg.Decisions)), ReturnBasis: ReturnBasis{Changes: []caption.CueChange{}, IssueCoverage: []IssueCoverage{}}, Readiness: ReviewReadiness{ProjectAwaitingReview: agg.Project.Status == domain.StatusReview}}
	for _, decision := range agg.Decisions {
		context.History = append(context.History, ReviewHistoryItem{Decision: decision, RevisionNumber: revisionNumbers[decision.RevisionID]})
	}
	sort.SliceStable(context.History, func(i, j int) bool {
		if context.History[i].RevisionNumber == context.History[j].RevisionNumber {
			return context.History[i].Decision.DecidedAt.Before(context.History[j].Decision.DecidedAt)
		}
		return context.History[i].RevisionNumber < context.History[j].RevisionNumber
	})
	active := agg.ActiveRevision()
	if active != nil {
		context.Readiness.AutomaticValidationPassed = active.ValidationStatus == domain.ValidationPassed
		candidateReviewer = strings.TrimSpace(candidateReviewer)
		context.Readiness.CandidateIsSubmitter = candidateReviewer != "" && candidateReviewer == active.SubmittedBy
	}
	var returned *domain.ReviewDecision
	for i := len(agg.Decisions) - 1; i >= 0; i-- {
		if agg.Decisions[i].Decision == domain.DecisionReturn {
			copy := agg.Decisions[i]
			returned = &copy
			break
		}
	}
	if returned == nil || active == nil {
		return context, nil
	}
	var returnedRevision *domain.CaptionRevision
	for i := range agg.Revisions {
		if agg.Revisions[i].ID == returned.RevisionID {
			copy := agg.Revisions[i]
			returnedRevision = &copy
			break
		}
	}
	if returnedRevision == nil {
		return nil, domain.NewError(domain.CodeIntegrity, "退回复核绑定的修订不存在")
	}
	context.ReturnBasis.Available = true
	context.ReturnBasis.Decision = returned
	context.ReturnBasis.Revision = returnedRevision
	context.ReturnBasis.Changes = caption.Compare(returnedRevision.Cues, active.Cues)
	newByOld := make(map[string]string)
	for _, change := range context.ReturnBasis.Changes {
		if change.OldCueID != "" {
			newByOld[change.OldCueID] = change.NewCueID
		}
	}
	for _, issue := range agg.Issues {
		if issue.RevisionID != returnedRevision.ID {
			continue
		}
		newID, changed := newByOld[issue.CueID]
		if changed || issue.Resolved {
			context.ReturnBasis.IssueCoverage = append(context.ReturnBasis.IssueCoverage, IssueCoverage{IssueID: issue.ID, OldCueID: issue.CueID, NewCueID: newID, CoveredByRevisionID: issue.CoveredByRevisionID, Resolved: issue.Resolved})
		}
	}
	return context, nil
}

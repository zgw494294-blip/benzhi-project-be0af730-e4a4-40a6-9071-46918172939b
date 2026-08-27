package workflow

import (
	"fmt"
	"sort"
	"strings"

	"subtitle-review/internal/caption"
	"subtitle-review/internal/domain"
)

func (s *Service) PreflightRevision(cmd PreflightRevisionCommand) (*PreflightResult, error) {
	if _, err := actor(cmd.SubmittedBy, "submittedBy"); err != nil {
		return nil, err
	}
	if len(cmd.Cues) == 0 {
		return nil, domain.FieldError("cues", "至少需要一个字幕片段")
	}
	agg, err := s.store.GetProject(cmd.ProjectID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersion(agg, cmd.ExpectedVersion); err != nil {
		return nil, err
	}
	if agg.Project.Status != domain.StatusDraft && agg.Project.Status != domain.StatusFixing {
		return nil, domain.NewError(domain.CodeConflict, "仅草稿或待整改项目可预检字幕修订")
	}
	cues := caption.NormalizeCues(cmd.Cues)
	digest, err := caption.RevisionDigest(cues)
	if err != nil {
		return nil, err
	}
	issues := s.validator.Validate("preflight", cues, agg.Project.Rules)
	sequences := make(map[string]int, len(cues))
	for _, cue := range cues {
		sequences[cue.CueID] = cue.Sequence
	}
	issueCounts := make(map[domain.IssueKind]int, 6)
	for _, kind := range []domain.IssueKind{domain.IssueOutOfBounds, domain.IssueOverlap, domain.IssueReadingSpeed, domain.IssueEmptyText, domain.IssueLineBreak, domain.IssueSpeaker} {
		issueCounts[kind] = 0
	}
	result := &PreflightResult{ProjectID: agg.Project.ID, Version: agg.Project.Version, Cues: cues, ContentDigest: digest, IssueCount: len(issues), IssueCounts: issueCounts, Issues: make([]PreflightIssue, 0, len(issues)), Passed: len(issues) == 0}
	for _, issue := range issues {
		result.IssueCounts[issue.Kind]++
		result.Issues = append(result.Issues, PreflightIssue{ValidationIssue: issue, CueSequence: sequences[issue.CueID]})
	}
	return result, nil
}

func (s *Service) RevisionDiff(projectID, childRevisionID, parentRevisionID string) (*RevisionDiff, error) {
	agg, err := s.store.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	var child *domain.CaptionRevision
	for i := range agg.Revisions {
		if agg.Revisions[i].ID == childRevisionID {
			child = &agg.Revisions[i]
			break
		}
	}
	if child == nil {
		return nil, domain.NewError(domain.CodeNotFound, "所选修订不属于该项目或不存在")
	}
	if parentRevisionID == "" {
		parentRevisionID = child.ParentRevisionID
	}
	var parent *domain.CaptionRevision
	if parentRevisionID != "" {
		for i := range agg.Revisions {
			if agg.Revisions[i].ID == parentRevisionID {
				parent = &agg.Revisions[i]
				break
			}
		}
		if parent == nil {
			return nil, domain.NewError(domain.CodeNotFound, "父修订不属于该项目或不存在")
		}
		if child.ParentRevisionID != parent.ID {
			return nil, domain.NewError(domain.CodeConflict, "所选修订不构成直接父子关系")
		}
	} else if child.ParentRevisionID != "" {
		return nil, domain.NewError(domain.CodeConflict, "非首个修订必须与其父修订比较")
	}
	parentCues := []domain.CaptionCue(nil)
	if parent != nil {
		parentCues = parent.Cues
	}
	changes := caption.Compare(parentCues, child.Cues)
	result := &RevisionDiff{ProjectID: projectID, ParentRevision: parent, ChildRevision: *child, Changes: changes, Counts: map[string]int{"added": 0, "removed": 0, "changed": 0}, IssueCoverage: []IssueCoverage{}}
	newByOld := make(map[string]string)
	for _, change := range changes {
		result.Counts[change.Type]++
		if change.OldCueID != "" {
			newByOld[change.OldCueID] = change.NewCueID
		}
	}
	if parent != nil {
		for _, issue := range agg.Issues {
			if issue.RevisionID != parent.ID {
				continue
			}
			if newID, changed := newByOld[issue.CueID]; changed || issue.Resolved {
				result.IssueCoverage = append(result.IssueCoverage, IssueCoverage{IssueID: issue.ID, OldCueID: issue.CueID, NewCueID: newID, CoveredByRevisionID: issue.CoveredByRevisionID, Resolved: issue.Resolved})
			}
		}
	}
	return result, nil
}

func issueSummary(agg *domain.ProjectAggregate) domain.IssueSummary {
	summary := domain.IssueSummary{ByKind: make(map[domain.IssueKind]int)}
	for _, issue := range agg.Issues {
		summary.Total++
		summary.ByKind[issue.Kind]++
		if strings.TrimSpace(issue.Disposition) != "" {
			summary.Dispositioned++
		}
		if issue.Resolved {
			summary.Covered++
		} else {
			summary.Unresolved++
		}
	}
	return summary
}

func (s *Service) BatchDisposition(cmd BatchDispositionCommand) (*BatchDispositionResult, error) {
	who, err := actor(cmd.Actor, "actor")
	if err != nil {
		return nil, err
	}
	if len(cmd.Items) == 0 {
		return nil, domain.FieldError("items", "至少需要一条问题处置说明")
	}
	seen := make(map[string]bool, len(cmd.Items))
	for _, item := range cmd.Items {
		if strings.TrimSpace(item.IssueID) == "" {
			return nil, domain.FieldError("issueID", "问题编号不能为空")
		}
		if seen[item.IssueID] {
			return nil, domain.FieldError("issueID", "批量请求中不能包含重复问题编号")
		}
		seen[item.IssueID] = true
		if strings.TrimSpace(item.Disposition) == "" {
			return nil, domain.FieldError("disposition", "处置说明不能为空")
		}
	}
	digest, err := requestDigest(cmd)
	if err != nil {
		return nil, err
	}
	result, err := s.store.Mutate(cmd.ProjectID, cmd.ExpectedVersion, who, "batch_disposition", cmd.IdempotencyKey, digest, s.now(), func(agg *domain.ProjectAggregate) (domain.CommandResult, string, map[string]string, error) {
		if agg.Project.Status != domain.StatusFixing {
			return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeConflict, "仅待整改项目可批量记录问题处置说明")
		}
		indices := make(map[string]int, len(agg.Issues))
		for i, issue := range agg.Issues {
			indices[issue.ID] = i
		}
		ids := make([]string, 0, len(cmd.Items))
		for _, item := range cmd.Items {
			index, ok := indices[item.IssueID]
			if !ok {
				return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeNotFound, "批量请求包含不存在或属于其他项目的问题")
			}
			if agg.Issues[index].Resolved {
				return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeConflict, "批量请求包含已覆盖问题")
			}
			ids = append(ids, item.IssueID)
		}
		for _, item := range cmd.Items {
			index := indices[item.IssueID]
			agg.Issues[index].Disposition = strings.TrimSpace(item.Disposition)
		}
		sort.Strings(ids)
		return domain.CommandResult{EntityID: strings.Join(ids, ",")}, "issues.batch_disposition_recorded", map[string]string{"count": fmt.Sprint(len(ids)), "issues": strings.Join(ids, ",")}, nil
	})
	if err != nil {
		return nil, err
	}
	agg, err := s.store.GetProject(result.ProjectID)
	if err != nil {
		return nil, err
	}
	return &BatchDispositionResult{Aggregate: agg, Summary: issueSummary(agg)}, nil
}

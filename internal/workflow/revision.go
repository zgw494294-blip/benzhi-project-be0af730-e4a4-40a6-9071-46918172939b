package workflow

import (
	"fmt"

	"subtitle-review/internal/caption"
	"subtitle-review/internal/domain"
)

func (s *Service) SubmitRevision(cmd SubmitRevisionCommand) (*domain.ProjectAggregate, error) {
	who, err := actor(cmd.SubmittedBy, "submittedBy")
	if err != nil {
		return nil, err
	}
	if len(cmd.Cues) == 0 {
		return nil, domain.FieldError("cues", "至少需要一个字幕片段")
	}
	digest, err := requestDigest(cmd)
	if err != nil {
		return nil, err
	}
	now := s.now()
	result, err := s.store.Mutate(cmd.ProjectID, cmd.ExpectedVersion, who, "submit_revision", cmd.IdempotencyKey, digest, now, func(agg *domain.ProjectAggregate) (domain.CommandResult, string, map[string]string, error) {
		if err := domain.EnsureMutable(agg.Project.Status); err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		if agg.Project.Status != domain.StatusDraft && agg.Project.Status != domain.StatusFixing {
			return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeConflict, "仅草稿或待整改项目可提交修订")
		}
		cues := caption.NormalizeCues(cmd.Cues)
		revisionNumber := len(agg.Revisions) + 1
		revisionID := randomID("rev_")
		contentDigest, err := caption.RevisionDigest(cues)
		if err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		parentID := agg.Project.ActiveRevisionID
		revision := domain.CaptionRevision{ID: revisionID, ProjectID: agg.Project.ID, ParentRevisionID: parentID, RevisionNumber: revisionNumber, SubmittedBy: who, SubmittedAt: now.UTC(), ContentDigest: contentDigest, Cues: cues, ValidationStatus: domain.ValidationPending}
		if err := revision.Validate(); err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		issues := s.validator.Validate(revisionID, cues, agg.Project.Rules)
		if len(issues) == 0 {
			revision.ValidationStatus = domain.ValidationPassed
		} else {
			revision.ValidationStatus = domain.ValidationFailed
		}
		if parent := agg.ActiveRevision(); parent != nil {
			agg.Issues = caption.CoverIssues(agg.Issues, parent.Cues, cues, revisionID)
		}
		agg.Revisions = append(agg.Revisions, revision)
		agg.Issues = append(agg.Issues, issues...)
		agg.Project.ActiveRevisionID = revisionID
		if len(issues) == 0 {
			agg.Project.Status = domain.StatusReview
		} else {
			agg.Project.Status = domain.StatusFixing
		}
		return domain.CommandResult{EntityID: revisionID}, "revision.validated", map[string]string{"revision": revisionID, "number": fmt.Sprint(revisionNumber), "issues": fmt.Sprint(len(issues))}, nil
	})
	if err != nil {
		return nil, err
	}
	return s.store.GetProject(result.ProjectID)
}

func (s *Service) DispositionIssue(cmd DispositionIssueCommand) (*domain.ProjectAggregate, error) {
	who, err := actor(cmd.Actor, "actor")
	if err != nil {
		return nil, err
	}
	if cmd.Disposition == "" {
		return nil, domain.FieldError("disposition", "处置说明不能为空")
	}
	digest, err := requestDigest(cmd)
	if err != nil {
		return nil, err
	}
	result, err := s.store.Mutate(cmd.ProjectID, cmd.ExpectedVersion, who, "disposition_issue", cmd.IdempotencyKey, digest, s.now(), func(agg *domain.ProjectAggregate) (domain.CommandResult, string, map[string]string, error) {
		if err := domain.EnsureMutable(agg.Project.Status); err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		if agg.Project.Status != domain.StatusFixing {
			return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeConflict, "仅待整改项目可记录问题处置说明")
		}
		for i := range agg.Issues {
			if agg.Issues[i].ID == cmd.IssueID {
				if agg.Issues[i].Resolved {
					return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeConflict, "问题已由替代修订覆盖")
				}
				agg.Issues[i].Disposition = cmd.Disposition
				return domain.CommandResult{EntityID: cmd.IssueID}, "issue.disposition_recorded", map[string]string{"issue": cmd.IssueID}, nil
			}
		}
		return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeNotFound, "问题不存在")
	})
	if err != nil {
		return nil, err
	}
	return s.store.GetProject(result.ProjectID)
}

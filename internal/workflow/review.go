package workflow

import (
	"subtitle-review/internal/domain"
)

func (s *Service) Review(cmd ReviewCommand) (*domain.ProjectAggregate, error) {
	who, err := actor(cmd.Reviewer, "reviewer")
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(cmd)
	if err != nil {
		return nil, err
	}
	now := s.now()
	result, err := s.store.Mutate(cmd.ProjectID, cmd.ExpectedVersion, who, "review", cmd.IdempotencyKey, digest, now, func(agg *domain.ProjectAggregate) (domain.CommandResult, string, map[string]string, error) {
		if err := domain.EnsureMutable(agg.Project.Status); err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		if agg.Project.Status != domain.StatusReview {
			return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeConflict, "项目当前不在待复核状态")
		}
		revision := agg.ActiveRevision()
		if revision == nil || revision.ValidationStatus != domain.ValidationPassed {
			return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeNotReady, "活动修订未通过自动校验")
		}
		decision := domain.ReviewDecision{ID: randomID("review_"), ProjectID: agg.Project.ID, RevisionID: revision.ID, Reviewer: who, LanguageApproved: cmd.LanguageApproved, AccessibilityApproved: cmd.AccessibilityApproved, Decision: cmd.Decision, Reason: cmd.Reason, DecidedAt: now.UTC()}
		if err := decision.Validate(revision.SubmittedBy); err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		agg.Decisions = append(agg.Decisions, decision)
		if decision.Decision == domain.DecisionApprove {
			agg.Project.Status = domain.StatusApproved
		} else {
			agg.Project.Status = domain.StatusFixing
		}
		return domain.CommandResult{EntityID: decision.ID}, "review." + string(decision.Decision), map[string]string{"review": decision.ID, "revision": revision.ID, "reason": decision.Reason}, nil
	})
	if err != nil {
		return nil, err
	}
	return s.store.GetProject(result.ProjectID)
}

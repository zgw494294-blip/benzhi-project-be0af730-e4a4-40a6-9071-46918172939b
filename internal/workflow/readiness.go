package workflow

import (
	"subtitle-review/internal/caption"
	"subtitle-review/internal/domain"
)

func (s *Service) FreezeReadiness(projectID string, expectedVersion int64) (*FreezeReadiness, error) {
	s.readinessMu.RLock()
	cached := s.readinessCache[projectID]
	s.readinessMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	agg, err := s.store.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersion(agg, expectedVersion); err != nil {
		return nil, err
	}
	result, err := evaluateFreezeReadiness(agg)
	if err != nil {
		return nil, err
	}
	if result.Ready {
		s.readinessMu.Lock()
		s.readinessCache[projectID] = result
		s.readinessMu.Unlock()
	}
	return result, nil
}

func evaluateFreezeReadiness(agg *domain.ProjectAggregate) (*FreezeReadiness, error) {
	result := &FreezeReadiness{ProjectID: agg.Project.ID, Version: agg.Project.Version, Checks: []ReadinessCheck{}}
	if agg.Project.Status == domain.StatusFrozen {
		result.Ready, result.AlreadyFrozen = true, true
		result.Credential = agg.Credential
		result.Checks = append(result.Checks, ReadinessCheck{Name: "冻结状态", Passed: true, Reason: "项目已完成冻结并签发凭据"})
		if agg.Manifest != nil {
			result.RevisionDigest = agg.Manifest.RevisionDigest
		}
		if agg.Credential != nil {
			result.ManifestMaterialDigest = agg.Credential.ManifestDigest
		}
		return result, nil
	}
	add := func(name string, passed bool, reason, entityID string) {
		result.Checks = append(result.Checks, ReadinessCheck{Name: name, Passed: passed, Reason: reason, EntityID: entityID})
		if !passed {
			result.BlockerCount++
		}
	}
	add("项目状态", agg.Project.Status == domain.StatusApproved, statusReadinessReason(agg.Project.Status), agg.Project.ID)
	revision := agg.ActiveRevision()
	add("活动修订", revision != nil, chooseReason(revision != nil, "活动修订存在", "项目没有活动修订"), agg.Project.ActiveRevisionID)
	validationPassed := revision != nil && revision.ValidationStatus == domain.ValidationPassed
	add("自动校验", validationPassed, chooseReason(validationPassed, "活动修订已通过自动校验", "活动修订未通过自动校验"), agg.Project.ActiveRevisionID)
	unresolvedID := ""
	unresolved := 0
	for _, issue := range agg.Issues {
		if revision != nil && issue.RevisionID == revision.ID && !issue.Resolved {
			unresolved++
			if unresolvedID == "" {
				unresolvedID = issue.ID
			}
		}
	}
	add("活动修订问题", unresolved == 0, chooseReason(unresolved == 0, "活动修订没有未解决问题", "活动修订仍有未解决问题"), unresolvedID)
	decision := agg.LatestDecision()
	approved := decision != nil && decision.Decision == domain.DecisionApprove
	decisionID := ""
	if decision != nil {
		decisionID = decision.ID
	}
	add("人工批准", approved, chooseReason(approved, "活动修订已获人工批准", "活动修订尚未获人工批准"), decisionID)
	result.Ready = result.BlockerCount == 0
	if !result.Ready {
		return result, nil
	}
	revisionDigest, err := caption.RevisionDigest(revision.Cues)
	if err != nil {
		return nil, err
	}
	preview := ManifestPreview{ProjectID: agg.Project.ID, ProjectVersion: agg.Project.Version + 1, Rules: agg.Project.Rules, RevisionID: revision.ID, RevisionDigest: revisionDigest, Cues: append([]domain.CaptionCue(nil), revision.Cues...), Decision: *decision}
	digest, err := caption.Digest(preview)
	if err != nil {
		return nil, err
	}
	result.RevisionDigest = revisionDigest
	result.ManifestMaterialDigest = digest
	result.ManifestPreview = &preview
	return result, nil
}

func chooseReason(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func statusReadinessReason(status domain.ProjectStatus) string {
	if status == domain.StatusApproved {
		return "项目已批准，可进入冻结"
	}
	return "当前状态为" + status.Label() + "，尚不能冻结"
}

package workflow

import (
	"regexp"
	"sort"
	"strings"

	"subtitle-review/internal/caption"
	"subtitle-review/internal/domain"
)

var credentialInputPattern = regexp.MustCompile(`^(cred_[0-9a-f]{12,40}|[0-9a-f]{16})$`)

func (s *Service) Freeze(cmd FreezeCommand) (*domain.ProjectAggregate, error) {
	who, err := actor(cmd.IssuedBy, "issuedBy")
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(cmd)
	if err != nil {
		return nil, err
	}
	now := s.now()
	result, err := s.store.Mutate(cmd.ProjectID, cmd.ExpectedVersion, who, "freeze", cmd.IdempotencyKey, digest, now, func(agg *domain.ProjectAggregate) (domain.CommandResult, string, map[string]string, error) {
		readiness, checkErr := evaluateFreezeReadiness(agg)
		if checkErr != nil {
			return domain.CommandResult{}, "", nil, checkErr
		}
		if !readiness.Ready || readiness.AlreadyFrozen {
			reason := "项目尚未满足冻结条件"
			for _, check := range readiness.Checks {
				if !check.Passed {
					reason = check.Reason
					break
				}
			}
			return domain.CommandResult{}, "", nil, domain.NewError(domain.CodeNotReady, reason)
		}
		revision := agg.ActiveRevision()
		manifest, manifestDigest, err := caption.BuildManifest(agg, who, now)
		if err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		credentialID := randomID("cred_")
		credential := domain.DeliveryCredential{CredentialID: credentialID, ProjectID: agg.Project.ID, RevisionID: revision.ID, ManifestDigest: manifestDigest, IssuedBy: who, IssuedAt: now.UTC(), VerificationCode: caption.VerificationCode(credentialID, manifestDigest)}
		agg.Manifest, agg.Credential = &manifest, &credential
		agg.Project.Status = domain.StatusFrozen
		return domain.CommandResult{EntityID: credentialID}, "project.frozen", map[string]string{"credential": credentialID, "manifestDigest": manifestDigest}, nil
	})
	if err != nil {
		return nil, err
	}
	return s.store.GetProject(result.ProjectID)
}

func (s *Service) VerifyCredential(code string) (*Verification, error) {
	code = strings.TrimSpace(code)
	if code == "" || !credentialInputPattern.MatchString(code) {
		return nil, domain.FieldError("code", "凭据编号或验证码格式无效")
	}
	if cached, ok := s.cachedVerification(code); ok {
		return cached, nil
	}
	agg, err := s.store.FindCredential(code)
	if err != nil {
		return nil, err
	}
	if agg.Credential == nil || agg.Manifest == nil {
		return nil, domain.NewError(domain.CodeIntegrity, "冻结项目缺少清单或凭据")
	}
	checks := make([]VerificationCheck, 0, 6)
	add := func(name string, passed bool, reason string) {
		checks = append(checks, VerificationCheck{Name: name, Passed: passed, Reason: chooseReason(passed, "", reason)})
	}
	revision := agg.ActiveRevision()
	latestDecision := agg.LatestDecision()
	rulesDigest, rulesErr := caption.Digest(agg.Project.Rules)
	manifestRulesDigest, manifestRulesErr := caption.Digest(agg.Manifest.Rules)
	bindingOK := agg.Project.Status == domain.StatusFrozen &&
		agg.Credential.ProjectID == agg.Project.ID && agg.Manifest.ProjectID == agg.Project.ID &&
		agg.Manifest.ProjectVersion == agg.Project.Version && revision != nil &&
		agg.Credential.RevisionID == agg.Project.ActiveRevisionID && agg.Manifest.RevisionID == agg.Project.ActiveRevisionID &&
		latestDecision != nil && agg.Manifest.Decision.ID == latestDecision.ID && agg.Manifest.Decision.RevisionID == agg.Project.ActiveRevisionID &&
		rulesErr == nil && manifestRulesErr == nil && rulesDigest == manifestRulesDigest
	add("项目与修订绑定", bindingOK, "凭据、清单、冻结状态或活动修订绑定不一致")
	revisionOK := false
	if revision != nil {
		digest, digestErr := caption.RevisionDigest(revision.Cues)
		manifestCuesDigest, manifestCuesErr := caption.RevisionDigest(agg.Manifest.Cues)
		revisionOK = digestErr == nil && manifestCuesErr == nil && digest == revision.ContentDigest && digest == agg.Manifest.RevisionDigest && digest == manifestCuesDigest
	}
	add("修订内容摘要", revisionOK, "当前修订内容摘要与修订或清单记录不一致")
	manifestDigest, manifestErr := caption.ManifestDigest(*agg.Manifest)
	manifestOK := manifestErr == nil && manifestDigest == agg.Credential.ManifestDigest
	add("冻结清单摘要", manifestOK, "冻结清单摘要与凭据记录不一致")
	codeOK := manifestErr == nil && caption.VerificationCode(agg.Credential.CredentialID, manifestDigest) == agg.Credential.VerificationCode
	add("凭据验证码", codeOK, "凭据验证码重算不一致")
	auditErr := s.store.CheckAuditIntegrity()
	add("审计摘要链", auditErr == nil, errorMessage(auditErr, "审计摘要链不一致"))
	sort.Slice(agg.Timeline, func(i, j int) bool {
		if agg.Timeline[i].At.Equal(agg.Timeline[j].At) {
			return agg.Timeline[i].Sequence < agg.Timeline[j].Sequence
		}
		return agg.Timeline[i].At.Before(agg.Timeline[j].At)
	})
	valid := true
	for _, check := range checks {
		valid = valid && check.Passed
	}
	message := "凭据、冻结清单、修订内容与审计摘要链一致"
	if !valid {
		message = "凭据核验存在不一致项"
	}
	result := &Verification{Valid: valid, Message: message, Checks: checks, Credential: *agg.Credential, Manifest: *agg.Manifest, Timeline: agg.Timeline}
	s.rememberVerification(code, result)
	return result, nil
}

func errorMessage(err error, fallback string) string {
	if err != nil {
		_, message, _ := domain.ErrorInfo(err)
		return message
	}
	return fallback
}

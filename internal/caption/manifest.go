package caption

import (
	"fmt"
	"time"

	"subtitle-review/internal/domain"
)

func BuildManifest(agg *domain.ProjectAggregate, actor string, at time.Time) (domain.Manifest, string, error) {
	rev := agg.ActiveRevision()
	if rev == nil {
		return domain.Manifest{}, "", domain.NewError(domain.CodeNotReady, "没有可冻结的活动修订")
	}
	decision := agg.LatestDecision()
	if decision == nil || decision.Decision != domain.DecisionApprove {
		return domain.Manifest{}, "", domain.NewError(domain.CodeNotReady, "活动修订尚未获批")
	}
	m := domain.Manifest{
		ProjectID: agg.Project.ID, ProjectVersion: agg.Project.Version + 1, Rules: agg.Project.Rules,
		RevisionID: rev.ID, RevisionDigest: rev.ContentDigest, Cues: append([]domain.CaptionCue(nil), rev.Cues...),
		Decision: *decision, FrozenAt: at.UTC(), FrozenBy: actor,
	}
	digest, err := ManifestDigest(m)
	if err != nil {
		return domain.Manifest{}, "", fmt.Errorf("计算冻结清单摘要: %w", err)
	}
	return m, digest, nil
}

func VerifyManifest(agg *domain.ProjectAggregate) (string, error) {
	if agg.Manifest == nil || agg.Credential == nil {
		return "", domain.NewError(domain.CodeNotReady, "项目没有冻结清单或交付凭据")
	}
	rev := agg.ActiveRevision()
	if rev == nil {
		return "", domain.NewError(domain.CodeIntegrity, "冻结修订不存在")
	}
	revDigest, err := RevisionDigest(rev.Cues)
	if err != nil || revDigest != agg.Manifest.RevisionDigest || revDigest != rev.ContentDigest {
		return "", domain.NewError(domain.CodeIntegrity, "字幕内容摘要不一致")
	}
	digest, err := ManifestDigest(*agg.Manifest)
	if err != nil {
		return "", err
	}
	if digest != agg.Credential.ManifestDigest {
		return "", domain.NewError(domain.CodeIntegrity, "冻结清单摘要与凭据不一致")
	}
	expected := VerificationCode(agg.Credential.CredentialID, digest)
	if expected != agg.Credential.VerificationCode {
		return "", domain.NewError(domain.CodeIntegrity, "凭据验证码不一致")
	}
	return digest, nil
}

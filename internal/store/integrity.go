package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"subtitle-review/internal/domain"
)

func stableDigest(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func validateDatabase(db database) error {
	if db.SchemaVersion != 1 {
		return domain.NewError(domain.CodeIntegrity, "快照版本无效")
	}
	if db.AuditSequence < 0 {
		return domain.NewError(domain.CodeIntegrity, "审计序号不能为负数")
	}
	sequences := make(map[int64]bool)
	var highest int64
	for id, agg := range db.Projects {
		if agg == nil {
			return domain.NewError(domain.CodeIntegrity, "项目聚合不能为空")
		}
		if id != agg.Project.ID {
			return domain.NewError(domain.CodeIntegrity, "项目索引与聚合编号不一致")
		}
		if err := validateAggregate(agg); err != nil {
			return fmt.Errorf("项目 %s: %w", id, err)
		}
		for _, event := range agg.Timeline {
			if sequences[event.Sequence] {
				return domain.NewError(domain.CodeIntegrity, "审计事件序号重复")
			}
			sequences[event.Sequence] = true
			if event.Sequence > highest {
				highest = event.Sequence
			}
		}
	}
	if highest != db.AuditSequence {
		return domain.NewError(domain.CodeIntegrity, "快照审计末端序号不一致")
	}
	for key, record := range db.Idempotency {
		if key == "" || key != record.Key || record.Operation == "" || record.RequestDigest == "" {
			return domain.NewError(domain.CodeIntegrity, "幂等记录内容无效")
		}
		if record.Result.ProjectID == "" {
			return domain.NewError(domain.CodeIntegrity, "幂等记录缺少项目编号")
		}
	}
	return nil
}

func validateAggregate(agg *domain.ProjectAggregate) error {
	p := agg.Project
	if p.ID == "" || p.Title == "" || !p.Status.Valid() || p.Version < 1 {
		return domain.NewError(domain.CodeIntegrity, "项目基础字段无效")
	}
	if err := p.Rules.Validate(); err != nil {
		return domain.NewError(domain.CodeIntegrity, "项目规则不满足约束")
	}
	revisions := make(map[string]*domain.CaptionRevision)
	for i := range agg.Revisions {
		revision := &agg.Revisions[i]
		if revision.ProjectID != p.ID || revision.ID == "" {
			return domain.NewError(domain.CodeIntegrity, "修订归属或编号无效")
		}
		if _, exists := revisions[revision.ID]; exists {
			return domain.NewError(domain.CodeIntegrity, "修订编号重复")
		}
		if revision.RevisionNumber != i+1 {
			return domain.NewError(domain.CodeIntegrity, "修订序号不连续")
		}
		if i == 0 && revision.ParentRevisionID != "" {
			return domain.NewError(domain.CodeIntegrity, "首个修订不能有父修订")
		}
		if i > 0 && revision.ParentRevisionID != agg.Revisions[i-1].ID {
			return domain.NewError(domain.CodeIntegrity, "修订谱系不连续")
		}
		digest, err := stableDigest(revision.Cues)
		if err != nil || digest != revision.ContentDigest {
			return domain.NewError(domain.CodeIntegrity, "修订内容摘要无效")
		}
		revisions[revision.ID] = revision
	}
	if p.ActiveRevisionID != "" {
		if _, ok := revisions[p.ActiveRevisionID]; !ok {
			return domain.NewError(domain.CodeIntegrity, "活动修订不存在")
		}
	}
	for _, issue := range agg.Issues {
		if _, ok := revisions[issue.RevisionID]; !ok {
			return domain.NewError(domain.CodeIntegrity, "问题引用了不存在的修订")
		}
		if issue.Resolved && issue.CoveredByRevisionID == "" {
			return domain.NewError(domain.CodeIntegrity, "已解决问题缺少覆盖修订")
		}
		if issue.CoveredByRevisionID != "" {
			if _, ok := revisions[issue.CoveredByRevisionID]; !ok {
				return domain.NewError(domain.CodeIntegrity, "问题覆盖修订不存在")
			}
		}
	}
	for _, decision := range agg.Decisions {
		revision, ok := revisions[decision.RevisionID]
		if !ok || decision.ProjectID != p.ID {
			return domain.NewError(domain.CodeIntegrity, "复核决定引用无效")
		}
		if err := decision.Validate(revision.SubmittedBy); err != nil {
			return domain.NewError(domain.CodeIntegrity, "复核决定不满足领域约束")
		}
	}
	if p.Status == domain.StatusFrozen {
		if agg.Manifest == nil || agg.Credential == nil {
			return domain.NewError(domain.CodeIntegrity, "冻结项目缺少清单或凭据")
		}
		if agg.Manifest.ProjectID != p.ID || agg.Credential.ProjectID != p.ID {
			return domain.NewError(domain.CodeIntegrity, "冻结材料项目归属无效")
		}
	} else if agg.Manifest != nil || agg.Credential != nil {
		return domain.NewError(domain.CodeIntegrity, "未冻结项目不应包含冻结材料")
	}
	return validateTimeline(agg.Timeline, p.ID, p.Version)
}

func validateTimeline(events []domain.AuditEvent, projectID string, version int64) error {
	if len(events) == 0 {
		return domain.NewError(domain.CodeIntegrity, "项目缺少审计时间线")
	}
	ordered := append([]domain.AuditEvent(nil), events...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ProjectVersion < ordered[j].ProjectVersion })
	for i, event := range ordered {
		if event.ProjectID != projectID || event.Actor == "" || event.EventType == "" {
			return domain.NewError(domain.CodeIntegrity, "审计事件字段无效")
		}
		if i > 0 && event.ProjectVersion <= ordered[i-1].ProjectVersion {
			return domain.NewError(domain.CodeIntegrity, "项目审计版本未递增")
		}
	}
	if ordered[len(ordered)-1].ProjectVersion != version {
		return domain.NewError(domain.CodeIntegrity, "项目版本与时间线末端不一致")
	}
	return nil
}

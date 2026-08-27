package workflow

import (
	"fmt"
	"strings"

	"subtitle-review/internal/domain"
)

func (s *Service) UpdateProjectRules(cmd UpdateProjectRulesCommand) (*domain.ProjectAggregate, error) {
	who, err := actor(cmd.Actor, "actor")
	if err != nil {
		return nil, err
	}
	rules := domain.ProjectRules{
		DurationMillis: cmd.DurationMillis, Language: strings.TrimSpace(cmd.Language),
		FrameRate: cmd.FrameRate, MaxCharsPerSecond: cmd.MaxCharsPerSecond,
		DeliveryStandard: strings.TrimSpace(cmd.DeliveryStandard),
	}
	if strings.TrimSpace(cmd.Title) == "" {
		return nil, domain.FieldError("title", "节目名称不能为空")
	}
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	digest, err := requestDigest(cmd)
	if err != nil {
		return nil, err
	}
	result, err := s.store.Mutate(cmd.ProjectID, cmd.ExpectedVersion, who, "update_project_rules", cmd.IdempotencyKey, digest, s.now(), func(agg *domain.ProjectAggregate) (domain.CommandResult, string, map[string]string, error) {
		changed, err := agg.Project.UpdateDraftRules(cmd.Title, rules, len(agg.Revisions))
		if err != nil {
			return domain.CommandResult{}, "", nil, err
		}
		if len(changed) == 0 {
			changed = append(changed, "无字段变化")
		}
		return domain.CommandResult{EntityID: agg.Project.ID}, "project.rules_updated", map[string]string{"changedFields": strings.Join(changed, "; ")}, nil
	})
	if err != nil {
		return nil, err
	}
	return s.store.GetProject(result.ProjectID)
}

func (s *Service) QueryProjects(query domain.ProjectQueueQuery) (domain.ProjectQueueResult, error) {
	query.Keyword = strings.TrimSpace(query.Keyword)
	query.Language = strings.TrimSpace(query.Language)
	if query.Status != "" && !query.Status.Valid() {
		return domain.ProjectQueueResult{}, domain.FieldError("status", "未知的项目状态")
	}
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 50
	}
	if query.Page < 1 {
		return domain.ProjectQueueResult{}, domain.FieldError("page", "页码必须大于零")
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		return domain.ProjectQueueResult{}, domain.FieldError("pageSize", "每页数量必须在 1 到 100 之间")
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page-1 > maxInt/query.PageSize {
		return domain.ProjectQueueResult{}, domain.FieldError("page", "页码超出支持范围")
	}
	return s.store.QueryProjects(query)
}

func ensureVersion(agg *domain.ProjectAggregate, expected int64) error {
	if expected <= 0 {
		return domain.FieldError("expectedVersion", "expectedVersion 必须大于零")
	}
	if agg.Project.Version != expected {
		return domain.NewError(domain.CodeStaleVersion, fmt.Sprintf("版本已更新，当前版本为 %d", agg.Project.Version))
	}
	return nil
}

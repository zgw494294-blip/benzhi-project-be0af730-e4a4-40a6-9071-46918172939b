package store

import (
	"sort"
	"strings"

	"subtitle-review/internal/domain"
)

func (s *FileStore) GetProject(id string) (*domain.ProjectAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agg, ok := s.db.Projects[id]
	if !ok {
		return nil, domain.NewError(domain.CodeNotFound, "字幕项目不存在")
	}
	return cloneAggregate(agg)
}

func (s *FileStore) ListProjects() ([]domain.CaptionProject, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.CaptionProject, 0, len(s.db.Projects))
	for _, agg := range s.db.Projects {
		items = append(items, agg.Project)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *FileStore) QueryProjects(query domain.ProjectQueueQuery) (domain.ProjectQueueResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := domain.ProjectQueueResult{Page: query.Page, PageSize: query.PageSize, Summary: domain.QueueSummary{Scope: "all_projects", StatusCounts: make(map[domain.ProjectStatus]int)}}
	for _, status := range domain.ProjectStatuses() {
		result.Summary.StatusCounts[status] = 0
	}
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	language := strings.ToLower(strings.TrimSpace(query.Language))
	matching := make([]domain.CaptionProject, 0)
	for _, agg := range s.db.Projects {
		project := agg.Project
		result.Summary.TotalProjects++
		result.Summary.StatusCounts[project.Status]++
		if project.Status == domain.StatusReview {
			result.Summary.ReviewProjects++
		}
		if project.Status == domain.StatusFixing {
			for _, issue := range agg.Issues {
				if !issue.Resolved {
					result.Summary.FixingUnresolvedIssues++
				}
			}
		}
		if query.Status != "" && project.Status != query.Status {
			continue
		}
		if language != "" && strings.ToLower(project.Rules.Language) != language {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(project.Title), keyword) && !strings.Contains(strings.ToLower(project.ID), keyword) {
			continue
		}
		matching = append(matching, project)
	}
	sort.Slice(matching, func(i, j int) bool {
		if matching[i].UpdatedAt.Equal(matching[j].UpdatedAt) {
			return matching[i].ID < matching[j].ID
		}
		return matching[i].UpdatedAt.After(matching[j].UpdatedAt)
	})
	result.Total = len(matching)
	start := (query.Page - 1) * query.PageSize
	if start >= len(matching) {
		result.Projects = []domain.CaptionProject{}
		return result, nil
	}
	end := start + query.PageSize
	if end > len(matching) {
		end = len(matching)
	}
	result.Projects = append([]domain.CaptionProject(nil), matching[start:end]...)
	return result, nil
}

func (s *FileStore) FindCredential(code string) (*domain.ProjectAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found *domain.ProjectAggregate
	for _, agg := range s.db.Projects {
		if agg.Credential != nil && (agg.Credential.VerificationCode == code || agg.Credential.CredentialID == code) {
			if found != nil {
				return nil, domain.NewError(domain.CodeConflict, "凭据编号或验证码存在非唯一匹配")
			}
			found = agg
		}
	}
	if found != nil {
		return cloneAggregate(found)
	}
	return nil, domain.NewError(domain.CodeNotFound, "未找到交付凭据")
}

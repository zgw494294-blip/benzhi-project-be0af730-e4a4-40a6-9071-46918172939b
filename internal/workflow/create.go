package workflow

import (
	"strings"

	"subtitle-review/internal/domain"
)

func (s *Service) CreateProject(cmd CreateProjectCommand) (*domain.ProjectAggregate, error) {
	who, err := actor(cmd.Actor, "actor")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.ID) == "" {
		cmd.ID = randomID("prj_")
	}
	rules := domain.ProjectRules{DurationMillis: cmd.DurationMillis, Language: strings.TrimSpace(cmd.Language), FrameRate: cmd.FrameRate, MaxCharsPerSecond: cmd.MaxCharsPerSecond, DeliveryStandard: strings.TrimSpace(cmd.DeliveryStandard)}
	project, err := domain.NewProject(cmd.ID, cmd.Title, rules, s.now())
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(cmd)
	if err != nil {
		return nil, err
	}
	result, err := s.store.CreateProject(project, who, cmd.IdempotencyKey, digest)
	if err != nil {
		return nil, err
	}
	return s.store.GetProject(result.ProjectID)
}

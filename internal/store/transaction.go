package store

import (
	"encoding/json"
	"fmt"
	"time"

	"subtitle-review/internal/domain"
)

type Mutation func(*domain.ProjectAggregate) (domain.CommandResult, string, map[string]string, error)

func (s *FileStore) CreateProject(project *domain.CaptionProject, actor, idempotencyKey, requestDigest string) (domain.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if result, found, err := s.checkIdempotency(idempotencyKey, "create_project", requestDigest); found || err != nil {
		return result, err
	}
	if _, exists := s.db.Projects[project.ID]; exists {
		return domain.CommandResult{}, domain.NewError(domain.CodeConflict, "项目编号已存在")
	}
	next, err := cloneDatabase(s.db)
	if err != nil {
		return domain.CommandResult{}, err
	}
	agg := &domain.ProjectAggregate{Project: *project}
	result := domain.CommandResult{ProjectID: project.ID, Version: project.Version, EntityID: project.ID, Status: project.Status}
	event := s.makeEvent(&next, project.ID, "project.created", actor, project.CreatedAt, project.Version, map[string]string{"title": project.Title})
	agg.Timeline = append(agg.Timeline, event)
	next.Projects[project.ID] = agg
	addIdempotency(&next, idempotencyKey, "create_project", requestDigest, result, project.CreatedAt)
	if err := s.commit(next, event); err != nil {
		return domain.CommandResult{}, err
	}
	return result, nil
}

func (s *FileStore) Mutate(projectID string, expectedVersion int64, actor, operation, idempotencyKey, requestDigest string, now time.Time, fn Mutation) (domain.CommandResult, error) {
	s.mu.Lock()
	if result, found, err := s.checkIdempotency(idempotencyKey, operation, requestDigest); found || err != nil {
		s.mu.Unlock()
		return result, err
	}
	current, ok := s.db.Projects[projectID]
	if !ok {
		s.mu.Unlock()
		return domain.CommandResult{}, domain.NewError(domain.CodeNotFound, "字幕项目不存在")
	}
	if current.Project.Version != expectedVersion {
		s.mu.Unlock()
		return domain.CommandResult{}, domain.NewError(domain.CodeStaleVersion, fmt.Sprintf("版本已更新，当前版本为 %d", current.Project.Version))
	}
	next, err := cloneDatabase(s.db)
	if err != nil {
		s.mu.Unlock()
		return domain.CommandResult{}, err
	}
	s.mu.Unlock()

	agg := next.Projects[projectID]
	result, eventType, details, err := fn(agg)
	if err != nil {
		return domain.CommandResult{}, err
	}
	agg.Project.Version++
	agg.Project.UpdatedAt = now.UTC()
	result.ProjectID, result.Version, result.Status = projectID, agg.Project.Version, agg.Project.Status
	event := s.makeEvent(&next, projectID, eventType, actor, now, agg.Project.Version, details)
	agg.Timeline = append(agg.Timeline, event)
	addIdempotency(&next, idempotencyKey, operation, requestDigest, result, now)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.commit(next, event); err != nil {
		return domain.CommandResult{}, err
	}
	return result, nil
}

func (s *FileStore) checkIdempotency(key, operation, digest string) (domain.CommandResult, bool, error) {
	if key == "" {
		return domain.CommandResult{}, false, domain.FieldError("idempotencyKey", "幂等键不能为空")
	}
	record, ok := s.db.Idempotency[key]
	if !ok {
		return domain.CommandResult{}, false, nil
	}
	if record.Operation != operation || record.RequestDigest != digest {
		return domain.CommandResult{}, true, domain.NewError(domain.CodeIdempotency, "幂等键已用于不同请求")
	}
	return record.Result, true, nil
}

func addIdempotency(db *database, key, operation, digest string, result domain.CommandResult, at time.Time) {
	db.Idempotency[key] = domain.IdempotencyRecord{Key: key, Operation: operation, RequestDigest: digest, Result: result, CreatedAt: at.UTC()}
}

func (s *FileStore) makeEvent(db *database, projectID, eventType, actor string, at time.Time, version int64, details map[string]string) domain.AuditEvent {
	db.AuditSequence++
	e := domain.AuditEvent{Sequence: db.AuditSequence, ProjectID: projectID, EventType: eventType, Actor: actor, At: at.UTC(), ProjectVersion: version, Details: details, PreviousDigest: db.AuditDigest}
	e.Digest = auditEventDigest(e)
	db.AuditDigest = e.Digest
	return e
}

func auditEventDigest(e domain.AuditEvent) string {
	b, _ := json.Marshal(struct {
		Sequence                                  int64
		ProjectID, EventType, Actor, At, Previous string
		Version                                   int64
		Details                                   map[string]string
	}{e.Sequence, e.ProjectID, e.EventType, e.Actor, e.At.Format(time.RFC3339Nano), e.PreviousDigest, e.ProjectVersion, e.Details})
	digest, _ := stableDigest(json.RawMessage(b))
	return digest
}

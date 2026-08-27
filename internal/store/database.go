package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"subtitle-review/internal/domain"
)

type database struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	Projects      map[string]*domain.ProjectAggregate `json:"projects"`
	Idempotency   map[string]domain.IdempotencyRecord `json:"idempotency"`
	AuditSequence int64                               `json:"auditSequence"`
	AuditDigest   string                              `json:"auditDigest,omitempty"`
}

type FileStore struct {
	mu           sync.RWMutex
	dir          string
	snapshotPath string
	auditPath    string
	db           database
}

func Open(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, domain.FieldError("dataDir", "数据目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	s := &FileStore{dir: dir, snapshotPath: filepath.Join(dir, "snapshot.json"), auditPath: filepath.Join(dir, "audit.jsonl")}
	s.db = database{SchemaVersion: 1, Projects: map[string]*domain.ProjectAggregate{}, Idempotency: map[string]domain.IdempotencyRecord{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	if err := s.reconcileAudit(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) load() error {
	b, err := os.ReadFile(s.snapshotPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &s.db); err != nil {
		return domain.NewError(domain.CodeIntegrity, "快照文件无法解析")
	}
	if s.db.SchemaVersion != 1 {
		return domain.NewError(domain.CodeIntegrity, "不支持的快照版本")
	}
	if s.db.Projects == nil {
		s.db.Projects = map[string]*domain.ProjectAggregate{}
	}
	if s.db.Idempotency == nil {
		s.db.Idempotency = map[string]domain.IdempotencyRecord{}
	}
	return validateDatabase(s.db)
}

func (s *FileStore) Close() error { return nil }

func cloneAggregate(in *domain.ProjectAggregate) (*domain.ProjectAggregate, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out domain.ProjectAggregate
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func cloneDatabase(in database) (database, error) {
	out := database{
		SchemaVersion: in.SchemaVersion,
		Projects:      make(map[string]*domain.ProjectAggregate, len(in.Projects)),
		Idempotency:   make(map[string]domain.IdempotencyRecord, len(in.Idempotency)),
		AuditSequence: in.AuditSequence,
		AuditDigest:   in.AuditDigest,
	}
	for id, aggregate := range in.Projects {
		out.Projects[id] = aggregate
	}
	for key, record := range in.Idempotency {
		out.Idempotency[key] = record
	}
	return out, nil
}

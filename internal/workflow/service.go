package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"subtitle-review/internal/caption"
	"subtitle-review/internal/domain"
	"subtitle-review/internal/store"
)

type Clock func() time.Time

type Service struct {
	store     *store.FileStore
	validator *caption.Validator
	now       Clock
}

func NewService(repo *store.FileStore) *Service {
	return &Service{store: repo, validator: caption.NewValidator(), now: time.Now}
}

func NewServiceWithClock(repo *store.FileStore, clock Clock) *Service {
	return &Service{store: repo, validator: caption.NewValidator(), now: clock}
}

func (s *Service) Store() *store.FileStore { return s.store }

func randomID(prefix string) string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return prefix + hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000000")))
	}
	return prefix + hex.EncodeToString(b)
}

func actor(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", domain.FieldError(field, "操作人不能为空")
	}
	return value, nil
}

func requestDigest(value any) (string, error) { return caption.Digest(value) }

func (s *Service) GetProject(id string) (*domain.ProjectAggregate, error) {
	return s.store.GetProject(id)
}
func (s *Service) ListProjects() ([]domain.CaptionProject, error) { return s.store.ListProjects() }

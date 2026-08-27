package store

import (
	"testing"
	"time"

	"subtitle-review/internal/domain"
)

func TestSnapshotAndAuditRecover(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := domain.NewProject("p", "节目", domain.ProjectRules{DurationMillis: 1000, Language: "zh-CN", FrameRate: 25, MaxCharsPerSecond: 12, DeliveryStandard: "规范"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(p, "编辑", "key", "digest"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := reopened.GetProject("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Timeline) != 1 || a.Timeline[0].EventType != "project.created" {
		t.Fatalf("timeline=%+v", a.Timeline)
	}
}

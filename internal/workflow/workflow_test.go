package workflow

import (
	"errors"
	"testing"
	"time"

	"subtitle-review/internal/domain"
	"subtitle-review/internal/store"
)

func testService(t *testing.T) *Service {
	t.Helper()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	return NewServiceWithClock(repo, func() time.Time { now = now.Add(time.Second); return now })
}

func createTestProject(t *testing.T, s *Service) *domain.ProjectAggregate {
	t.Helper()
	a, err := s.CreateProject(CreateProjectCommand{ID: "p1", Title: "节目", DurationMillis: 10000, Language: "zh-CN", FrameRate: 25, MaxCharsPerSecond: 20, DeliveryStandard: "规范", Actor: "编辑", IdempotencyKey: "create"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func cleanCues() []domain.CaptionCue {
	return []domain.CaptionCue{{Sequence: 1, StartMillis: 0, EndMillis: 3000, Text: "欢迎收看", Speaker: "主持人"}}
}

func TestFullWorkflowAndFrozenGuard(t *testing.T) {
	s := testService(t)
	a := createTestProject(t, s)
	a, err := s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: cleanCues(), IdempotencyKey: "rev"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Project.Status != domain.StatusReview {
		t.Fatalf("status=%s", a.Project.Status)
	}
	a, err = s.Review(ReviewCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Reviewer: "审核员", LanguageApproved: true, AccessibilityApproved: true, Decision: domain.DecisionApprove, IdempotencyKey: "review"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Freeze(FreezeCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, IssuedBy: "负责人", IdempotencyKey: "freeze"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.VerifyCredential(a.Credential.VerificationCode)
	if err != nil || !v.Valid {
		t.Fatalf("verify=%+v err=%v", v, err)
	}
	_, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: cleanCues(), IdempotencyKey: "after-freeze"})
	assertCode(t, err, domain.CodeFrozen)
}

func TestReviewerSeparationAndStaleVersion(t *testing.T) {
	s := testService(t)
	a := createTestProject(t, s)
	_, err := s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: 99, SubmittedBy: "编辑", Cues: cleanCues(), IdempotencyKey: "stale"})
	assertCode(t, err, domain.CodeStaleVersion)
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: cleanCues(), IdempotencyKey: "rev"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Review(ReviewCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Reviewer: "编辑", LanguageApproved: true, AccessibilityApproved: true, Decision: domain.DecisionApprove, IdempotencyKey: "own-review"})
	assertCode(t, err, domain.CodeForbidden)
}

func TestIdempotencyReusesFirstResult(t *testing.T) {
	s := testService(t)
	cmd := CreateProjectCommand{ID: "p1", Title: "节目", DurationMillis: 10000, Language: "zh-CN", FrameRate: 25, MaxCharsPerSecond: 20, DeliveryStandard: "规范", Actor: "编辑", IdempotencyKey: "same"}
	first, err := s.CreateProject(cmd)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateProject(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if first.Project.Version != second.Project.Version {
		t.Fatal("idempotent result changed")
	}
	cmd.Title = "不同节目"
	_, err = s.CreateProject(cmd)
	assertCode(t, err, domain.CodeIdempotency)
}

func assertCode(t *testing.T, err error, want domain.ErrorCode) {
	t.Helper()
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != want {
		t.Fatalf("error=%v want=%s", err, want)
	}
}

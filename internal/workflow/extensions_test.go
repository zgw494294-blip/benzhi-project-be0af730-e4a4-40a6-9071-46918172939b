package workflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"subtitle-review/internal/domain"
	"subtitle-review/internal/store"
)

func TestDraftRulesAndQueueQuery(t *testing.T) {
	s := testService(t)
	a := createTestProject(t, s)
	cmd := UpdateProjectRulesCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Title: "修订节目", DurationMillis: 12000, Language: " zh-CN ", FrameRate: 30, MaxCharsPerSecond: 18, DeliveryStandard: " 修订规范 ", Actor: "编辑", IdempotencyKey: "rules"}
	a, err := s.UpdateProjectRules(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if a.Project.Version != 2 || a.Project.Title != "修订节目" || a.Project.Rules.DurationMillis != 12000 || len(a.Timeline) != 2 || a.Timeline[1].EventType != "project.rules_updated" {
		t.Fatalf("updated aggregate=%+v timeline=%+v", a.Project, a.Timeline)
	}
	replayed, err := s.UpdateProjectRules(cmd)
	if err != nil || replayed.Project.Version != 2 || len(replayed.Timeline) != 2 {
		t.Fatalf("idempotent rules update=%+v err=%v", replayed, err)
	}
	invalid := cmd
	invalid.ExpectedVersion, invalid.FrameRate, invalid.IdempotencyKey = 2, 0, "invalid-rules"
	_, err = s.UpdateProjectRules(invalid)
	assertCode(t, err, domain.CodeInvalid)
	unchanged, _ := s.GetProject("p1")
	if unchanged.Project.Version != 2 || unchanged.Project.Rules.FrameRate != 30 {
		t.Fatal("invalid update changed project")
	}
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: 2, SubmittedBy: "编辑", Cues: cleanCues(), IdempotencyKey: "rules-test-revision"})
	if err != nil {
		t.Fatal(err)
	}
	cmd.ExpectedVersion, cmd.IdempotencyKey = a.Project.Version, "rules-after-revision"
	_, err = s.UpdateProjectRules(cmd)
	assertCode(t, err, domain.CodeConflict)
	_, err = s.CreateProject(CreateProjectCommand{ID: "p2", Title: "其他节目", DurationMillis: 5000, Language: "en-US", FrameRate: 25, MaxCharsPerSecond: 15, DeliveryStandard: "规范", Actor: "编辑", IdempotencyKey: "create-p2"})
	if err != nil {
		t.Fatal(err)
	}
	queue, err := s.QueryProjects(domain.ProjectQueueQuery{Status: domain.StatusReview, Language: "zh-CN", Keyword: "  修订  ", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if queue.Total != 1 || len(queue.Projects) != 1 || queue.Projects[0].ID != "p1" || queue.Summary.StatusCounts[domain.StatusDraft] != 1 || queue.Summary.StatusCounts[domain.StatusReview] != 1 {
		t.Fatalf("queue=%+v", queue)
	}
	empty, err := s.QueryProjects(domain.ProjectQueueQuery{Page: 9, PageSize: 1})
	if err != nil || len(empty.Projects) != 0 || empty.Total != 2 {
		t.Fatalf("out-of-range queue=%+v err=%v", empty, err)
	}
	_, err = s.QueryProjects(domain.ProjectQueueQuery{Status: "unknown", Page: 1, PageSize: 10})
	assertCode(t, err, domain.CodeInvalid)
}

func TestPreflightBatchDiffReviewReadinessAndVerification(t *testing.T) {
	s := testService(t)
	a := createTestProject(t, s)
	flawed := []domain.CaptionCue{
		{Sequence: 1, StartMillis: 0, EndMillis: 500, Text: "Speaker:快速字幕"},
		{Sequence: 2, StartMillis: 400, EndMillis: 2000, Text: "第二条"},
	}
	beforeVersion, beforeEvents := a.Project.Version, len(a.Timeline)
	preflight, err := s.PreflightRevision(PreflightRevisionCommand{ProjectID: "p1", ExpectedVersion: beforeVersion, SubmittedBy: "编辑", Cues: flawed})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.Passed || preflight.IssueCounts[domain.IssueOverlap] == 0 || preflight.IssueCounts[domain.IssueSpeaker] == 0 || preflight.ContentDigest == "" || preflight.Cues[0].CueID == "" {
		t.Fatalf("preflight=%+v", preflight)
	}
	afterPreflight, _ := s.GetProject("p1")
	if afterPreflight.Project.Version != beforeVersion || len(afterPreflight.Timeline) != beforeEvents || len(afterPreflight.Revisions) != 0 {
		t.Fatal("preflight changed aggregate")
	}
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: flawed, IdempotencyKey: "flawed"})
	if err != nil {
		t.Fatal(err)
	}
	items := make([]IssueDispositionInput, 0, len(a.Issues))
	for _, issue := range a.Issues {
		items = append(items, IssueDispositionInput{IssueID: issue.ID, Disposition: "将在替代修订中调整"})
	}
	batchVersion, batchEvents := a.Project.Version, len(a.Timeline)
	batchCmd := BatchDispositionCommand{ProjectID: "p1", ExpectedVersion: batchVersion, Items: items, Actor: "编辑", IdempotencyKey: "batch"}
	batch, err := s.BatchDisposition(batchCmd)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Aggregate.Project.Version != batchVersion+1 || len(batch.Aggregate.Timeline) != batchEvents+1 || batch.Summary.Dispositioned != len(items) {
		t.Fatalf("batch=%+v", batch)
	}
	replayed, err := s.BatchDisposition(batchCmd)
	if err != nil || replayed.Aggregate.Project.Version != batch.Aggregate.Project.Version || len(replayed.Aggregate.Timeline) != len(batch.Aggregate.Timeline) {
		t.Fatalf("batch replay=%+v err=%v", replayed, err)
	}
	clean := []domain.CaptionCue{
		{Sequence: 1, StartMillis: 0, EndMillis: 3000, Text: "快速字幕"},
		{Sequence: 2, StartMillis: 4000, EndMillis: 7000, Text: "新增字幕"},
	}
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: batch.Aggregate.Project.Version, SubmittedBy: "编辑", Cues: clean, IdempotencyKey: "clean"})
	if err != nil {
		t.Fatal(err)
	}
	diff, err := s.RevisionDiff("p1", a.Revisions[1].ID, a.Revisions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Counts["changed"] != 1 || diff.Counts["removed"] != 1 || diff.Counts["added"] != 1 || len(diff.IssueCoverage) == 0 {
		t.Fatalf("diff=%+v", diff)
	}
	a, err = s.Review(ReviewCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Reviewer: "审核员", Decision: domain.DecisionReturn, Reason: "调整首条措辞", IdempotencyKey: "return"})
	if err != nil {
		t.Fatal(err)
	}
	corrected := append([]domain.CaptionCue(nil), clean...)
	corrected[0].Text = "优化后的快速字幕"
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: corrected, IdempotencyKey: "corrected"})
	if err != nil {
		t.Fatal(err)
	}
	context, err := s.ReviewContext("p1", "编辑")
	if err != nil {
		t.Fatal(err)
	}
	if len(context.History) != 1 || !context.ReturnBasis.Available || len(context.ReturnBasis.Changes) == 0 || !context.Readiness.CandidateIsSubmitter {
		t.Fatalf("review context=%+v", context)
	}
	blocked, err := s.FreezeReadiness("p1", a.Project.Version)
	if err != nil || blocked.Ready || blocked.BlockerCount == 0 {
		t.Fatalf("blocked readiness=%+v err=%v", blocked, err)
	}
	a, err = s.Review(ReviewCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Reviewer: "独立审核员", LanguageApproved: true, AccessibilityApproved: true, Decision: domain.DecisionApprove, IdempotencyKey: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := s.FreezeReadiness("p1", a.Project.Version)
	if err != nil || !ready.Ready || ready.ManifestMaterialDigest == "" || ready.RevisionDigest == "" {
		t.Fatalf("ready=%+v err=%v", ready, err)
	}
	version, events := a.Project.Version, len(a.Timeline)
	unchanged, _ := s.GetProject("p1")
	if unchanged.Project.Version != version || len(unchanged.Timeline) != events {
		t.Fatal("readiness changed aggregate")
	}
	a, err = s.Freeze(FreezeCommand{ProjectID: "p1", ExpectedVersion: version, IssuedBy: "发布负责人", IdempotencyKey: "freeze-extended"})
	if err != nil {
		t.Fatal(err)
	}
	report, err := s.VerifyCredential("  " + a.Credential.CredentialID + "  ")
	if err != nil || !report.Valid || len(report.Checks) != 5 {
		t.Fatalf("verification=%+v err=%v", report, err)
	}
	_, err = s.VerifyCredential("bad code")
	assertCode(t, err, domain.CodeInvalid)
}

func TestPreflightMatchesFormalSubmissionWithoutWriting(t *testing.T) {
	s := testService(t)
	a := createTestProject(t, s)
	input := []domain.CaptionCue{
		{Sequence: 2, StartMillis: 3000, EndMillis: 6000, Text: "  第二条   字幕  ", Speaker: "  嘉宾  "},
		{Sequence: 1, StartMillis: 0, EndMillis: 2500, Text: "欢迎收看", Speaker: "主持人"},
	}
	version, events := a.Project.Version, len(a.Timeline)
	preflight, err := s.PreflightRevision(PreflightRevisionCommand{ProjectID: "p1", ExpectedVersion: version, SubmittedBy: "编辑", Cues: input})
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.Passed || preflight.IssueCount != 0 || preflight.ContentDigest == "" || preflight.Cues[0].Sequence != 1 || preflight.Cues[1].Text != "第二条 字幕" || preflight.Cues[1].Speaker != "嘉宾" {
		t.Fatalf("preflight=%+v", preflight)
	}
	unchanged, err := s.GetProject("p1")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Project.Version != version || len(unchanged.Revisions) != 0 || len(unchanged.Timeline) != events {
		t.Fatal("预检改变了项目聚合")
	}
	submitted, err := s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: version, SubmittedBy: "编辑", Cues: input, IdempotencyKey: "matching-submit"})
	if err != nil {
		t.Fatal(err)
	}
	revision := submitted.ActiveRevision()
	if revision == nil || revision.ValidationStatus != domain.ValidationPassed || revision.ContentDigest != preflight.ContentDigest || len(revision.Cues) != len(preflight.Cues) {
		t.Fatalf("revision=%+v preflight=%+v", revision, preflight)
	}
	for i := range revision.Cues {
		if revision.Cues[i] != preflight.Cues[i] {
			t.Fatalf("cue %d mismatch: revision=%+v preflight=%+v", i, revision.Cues[i], preflight.Cues[i])
		}
	}
	_, err = s.PreflightRevision(PreflightRevisionCommand{ProjectID: "p1", ExpectedVersion: version, SubmittedBy: "编辑", Cues: input})
	assertCode(t, err, domain.CodeStaleVersion)
}

func TestRevisionDiffReadOnlyFirstAndUnchanged(t *testing.T) {
	s := testService(t)
	a := createTestProject(t, s)
	flawed := []domain.CaptionCue{{Sequence: 1, StartMillis: 0, EndMillis: 200, Text: "阅读速度超过阈值的字幕"}}
	a, err := s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: flawed, IdempotencyKey: "first-diff"})
	if err != nil {
		t.Fatal(err)
	}
	version, events := a.Project.Version, len(a.Timeline)
	first, err := s.RevisionDiff("p1", a.Revisions[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ParentRevision != nil || first.Counts["added"] != 1 || first.Counts["removed"] != 0 || first.Counts["changed"] != 0 || first.Changes[0].After == nil {
		t.Fatalf("first diff=%+v", first)
	}
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: version, SubmittedBy: "编辑", Cues: flawed, IdempotencyKey: "same-diff"})
	if err != nil {
		t.Fatal(err)
	}
	same, err := s.RevisionDiff("p1", a.Revisions[1].ID, a.Revisions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(same.Changes) != 0 || same.Counts["added"] != 0 || same.Counts["removed"] != 0 || same.Counts["changed"] != 0 {
		t.Fatalf("same diff=%+v", same)
	}
	if versionAfter, eventCount := a.Project.Version, len(a.Timeline); versionAfter != version+1 || eventCount != events+1 {
		t.Fatal("测试修订状态异常")
	}
	beforeQuery, _ := s.GetProject("p1")
	_, err = s.RevisionDiff("p1", a.Revisions[1].ID, "missing")
	assertCode(t, err, domain.CodeNotFound)
	afterQuery, _ := s.GetProject("p1")
	if afterQuery.Project.Version != beforeQuery.Project.Version || len(afterQuery.Timeline) != len(beforeQuery.Timeline) {
		t.Fatal("差异查询改变了项目聚合")
	}
}

func TestFreezeReadinessUsesActiveRevisionIssuesOnly(t *testing.T) {
	s := testService(t)
	a := createTestProject(t, s)
	withOverlap := []domain.CaptionCue{
		{Sequence: 1, StartMillis: 0, EndMillis: 2000, Text: "第一条"},
		{Sequence: 2, StartMillis: 1500, EndMillis: 3000, Text: "第二条"},
	}
	a, err := s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: withOverlap, IdempotencyKey: "overlap"})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Issues) != 1 || a.Issues[0].Kind != domain.IssueOverlap {
		t.Fatalf("issues=%+v", a.Issues)
	}
	clean := []domain.CaptionCue{
		{Sequence: 1, StartMillis: 0, EndMillis: 1400, Text: "第一条"},
		{Sequence: 2, StartMillis: 1500, EndMillis: 3000, Text: "第二条"},
	}
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: clean, IdempotencyKey: "resolve-overlap"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Project.Status != domain.StatusReview || a.Issues[0].Resolved || a.Issues[0].Disposition != "" {
		t.Fatalf("historical issue=%+v status=%s", a.Issues[0], a.Project.Status)
	}
	a, err = s.Review(ReviewCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Reviewer: "审核员", LanguageApproved: true, AccessibilityApproved: true, Decision: domain.DecisionApprove, IdempotencyKey: "approve-active"})
	if err != nil {
		t.Fatal(err)
	}
	version, events := a.Project.Version, len(a.Timeline)
	readiness, err := s.FreezeReadiness("p1", version)
	if err != nil || !readiness.Ready || readiness.BlockerCount != 0 || readiness.ManifestMaterialDigest == "" {
		t.Fatalf("readiness=%+v err=%v", readiness, err)
	}
	unchanged, _ := s.GetProject("p1")
	if unchanged.Project.Version != version || len(unchanged.Timeline) != events || unchanged.Manifest != nil || unchanged.Credential != nil {
		t.Fatal("就绪检查产生了写入")
	}
	frozen, err := s.Freeze(FreezeCommand{ProjectID: "p1", ExpectedVersion: version, IssuedBy: "发布负责人", IdempotencyKey: "freeze-active"})
	if err != nil || frozen.Project.Status != domain.StatusFrozen || frozen.Manifest == nil || frozen.Credential == nil {
		t.Fatalf("frozen=%+v err=%v", frozen, err)
	}
	_, err = s.VerifyCredential("0000000000000000")
	assertCode(t, err, domain.CodeNotFound)
}

func TestCredentialVerificationReportsAuditFailureWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	repo, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	s := NewServiceWithClock(repo, func() time.Time {
		now = now.Add(time.Second)
		return now
	})
	a := createTestProject(t, s)
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: cleanCues(), IdempotencyKey: "audit-revision"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Review(ReviewCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Reviewer: "审核员", LanguageApproved: true, AccessibilityApproved: true, Decision: domain.DecisionApprove, IdempotencyKey: "audit-review"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Freeze(FreezeCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, IssuedBy: "发布负责人", IdempotencyKey: "audit-freeze"})
	if err != nil {
		t.Fatal(err)
	}
	byID, err := s.VerifyCredential(a.Credential.CredentialID)
	if err != nil || !byID.Valid {
		t.Fatalf("credential ID verification=%+v err=%v", byID, err)
	}
	byCode, err := s.VerifyCredential(a.Credential.VerificationCode)
	if err != nil || !byCode.Valid || byCode.Credential.ProjectID != byID.Credential.ProjectID || byCode.Credential.ManifestDigest != byID.Credential.ManifestDigest {
		t.Fatalf("verification code report=%+v err=%v", byCode, err)
	}
	version, eventCount := a.Project.Version, len(a.Timeline)
	if err := os.WriteFile(filepath.Join(dir, "audit.jsonl"), []byte("{}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	report, err := s.VerifyCredential(a.Credential.VerificationCode)
	if err != nil {
		t.Fatal(err)
	}
	if report.Valid {
		t.Fatalf("corrupt audit report=%+v", report)
	}
	failed := 0
	for _, check := range report.Checks {
		if !check.Passed {
			failed++
			if check.Name != "审计摘要链" || check.Reason == "" {
				t.Fatalf("unexpected failed check=%+v", check)
			}
		}
	}
	if failed != 1 {
		t.Fatalf("checks=%+v", report.Checks)
	}
	unchanged, err := s.GetProject("p1")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Project.Version != version || len(unchanged.Timeline) != eventCount {
		t.Fatal("凭据核验改变了项目聚合")
	}
}

func TestCredentialTimelineIsSortedByDecisionTime(t *testing.T) {
	dir := t.TempDir()
	repo, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	times := []time.Time{
		time.Date(2026, 8, 27, 12, 0, 4, 0, time.UTC),
		time.Date(2026, 8, 27, 12, 0, 3, 0, time.UTC),
		time.Date(2026, 8, 27, 12, 0, 2, 0, time.UTC),
		time.Date(2026, 8, 27, 12, 0, 1, 0, time.UTC),
	}
	index := 0
	s := NewServiceWithClock(repo, func() time.Time {
		at := times[index]
		index++
		return at
	})
	a := createTestProject(t, s)
	a, err = s.SubmitRevision(SubmitRevisionCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, SubmittedBy: "编辑", Cues: cleanCues(), IdempotencyKey: "timeline-revision"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Review(ReviewCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, Reviewer: "审核员", LanguageApproved: true, AccessibilityApproved: true, Decision: domain.DecisionApprove, IdempotencyKey: "timeline-review"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Freeze(FreezeCommand{ProjectID: "p1", ExpectedVersion: a.Project.Version, IssuedBy: "发布负责人", IdempotencyKey: "timeline-freeze"})
	if err != nil {
		t.Fatal(err)
	}
	report, err := s.VerifyCredential(a.Credential.VerificationCode)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(report.Timeline); i++ {
		if report.Timeline[i].At.Before(report.Timeline[i-1].At) {
			t.Fatalf("timeline is not ordered: %+v", report.Timeline)
		}
	}
}

package caption

import (
	"testing"

	"subtitle-review/internal/domain"
)

func TestValidatorFindsAllIssueKinds(t *testing.T) {
	rules := domain.ProjectRules{DurationMillis: 5000, Language: "zh-CN", FrameRate: 25, MaxCharsPerSecond: 3, DeliveryStandard: "test"}
	cues := NormalizeCues([]domain.CaptionCue{
		{Sequence: 1, StartMillis: 0, EndMillis: 1000, Text: "[说话人甲]这是一条非常非常长并且没有合理断行的字幕文本"},
		{Sequence: 2, StartMillis: 900, EndMillis: 2000, Text: ""},
		{Sequence: 3, StartMillis: 4500, EndMillis: 6000, Text: "甲\n乙\n丙"},
	})
	issues := NewValidator().Validate("rev", cues, rules)
	kinds := map[domain.IssueKind]bool{}
	for _, issue := range issues {
		kinds[issue.Kind] = true
	}
	for _, want := range []domain.IssueKind{domain.IssueOutOfBounds, domain.IssueOverlap, domain.IssueReadingSpeed, domain.IssueEmptyText, domain.IssueLineBreak, domain.IssueSpeaker} {
		if !kinds[want] {
			t.Errorf("missing issue kind %s", want)
		}
	}
}

func TestNormalizeAndCoverIssues(t *testing.T) {
	parent := NormalizeCues([]domain.CaptionCue{{Sequence: 3, StartMillis: 1000, EndMillis: 2000, Text: "  原文  "}})
	child := NormalizeCues([]domain.CaptionCue{{Sequence: 1, StartMillis: 1000, EndMillis: 3000, Text: "修订文本"}})
	issues := []domain.ValidationIssue{{ID: "i1", CueID: parent[0].CueID, RevisionID: "r1"}}
	covered := CoverIssues(issues, parent, child, "r2")
	if !covered[0].Resolved || covered[0].CoveredByRevisionID != "r2" {
		t.Fatalf("issue not covered: %+v", covered[0])
	}
	if parent[0].Sequence != 1 || parent[0].LineCount != 1 || parent[0].Text != "原文" {
		t.Fatalf("unexpected normalization: %+v", parent[0])
	}
}

func TestCompareUsesStableCueAnchors(t *testing.T) {
	parent := NormalizeCues([]domain.CaptionCue{
		{Sequence: 1, StartMillis: 0, EndMillis: 1000, Text: "保留"},
		{Sequence: 2, StartMillis: 2000, EndMillis: 3000, Text: "后移"},
	})
	inserted := NormalizeCues([]domain.CaptionCue{
		{Sequence: 1, StartMillis: 0, EndMillis: 1000, Text: "保留"},
		{Sequence: 2, StartMillis: 1200, EndMillis: 1800, Text: "新增"},
		{Sequence: 3, StartMillis: 2000, EndMillis: 3000, Text: "后移"},
	})
	changes := Compare(parent, inserted)
	if len(changes) != 1 || changes[0].Type != "added" || changes[0].After.Text != "新增" {
		t.Fatalf("insert diff=%+v", changes)
	}
}

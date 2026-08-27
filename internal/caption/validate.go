package caption

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"subtitle-review/internal/domain"
)

type Validator struct {
	issues []domain.ValidationIssue
}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(revisionID string, cues []domain.CaptionCue, rules domain.ProjectRules) []domain.ValidationIssue {
	issues := v.issues[:0]
	for i, cue := range cues {
		if cue.StartMillis < 0 || cue.EndMillis <= cue.StartMillis || cue.EndMillis > rules.DurationMillis {
			issues = append(issues, issue(revisionID, cue.CueID, domain.IssueOutOfBounds, "字幕时间码超出节目范围或区间无效"))
		}
		if i > 0 && cue.StartMillis < cues[i-1].EndMillis {
			issues = append(issues, issue(revisionID, cue.CueID, domain.IssueOverlap, fmt.Sprintf("与片段 %s 时间重叠", cues[i-1].CueID)))
		}
		text := strings.TrimSpace(cue.Text)
		if text == "" {
			issues = append(issues, issue(revisionID, cue.CueID, domain.IssueEmptyText, "字幕文本不能为空"))
		}
		if cue.LineCount > 2 || hasLongLine(cue.Text, 24) {
			issues = append(issues, issue(revisionID, cue.CueID, domain.IssueLineBreak, "字幕应不超过两行且每行不超过 24 个字符"))
		}
		if invalidSpeaker(cue) {
			issues = append(issues, issue(revisionID, cue.CueID, domain.IssueSpeaker, "说话人标记需填写 speaker 字段，不应混入字幕正文"))
		}
		if speed := charsPerSecond(cue); speed > rules.MaxCharsPerSecond {
			issues = append(issues, issue(revisionID, cue.CueID, domain.IssueReadingSpeed, fmt.Sprintf("阅读速度 %.1f 字/秒超过阈值 %.1f", speed, rules.MaxCharsPerSecond)))
		}
	}
	v.issues = issues
	return issues
}

func issue(revisionID, cueID string, kind domain.IssueKind, message string) domain.ValidationIssue {
	id := "iss_" + ShortDigest([]byte(revisionID+"|"+cueID+"|"+string(kind)), 14)
	return domain.ValidationIssue{ID: id, RevisionID: revisionID, CueID: cueID, Kind: kind, Severity: "error", Message: message}
}

func charsPerSecond(cue domain.CaptionCue) float64 {
	seconds := float64(cue.EndMillis-cue.StartMillis) / 1000
	if seconds <= 0 {
		return 0
	}
	text := strings.ReplaceAll(cue.Text, "\n", "")
	text = strings.ReplaceAll(text, " ", "")
	return float64(utf8.RuneCountInString(text)) / seconds
}

func hasLongLine(text string, limit int) bool {
	for _, line := range strings.Split(text, "\n") {
		if utf8.RuneCountInString(line) > limit {
			return true
		}
	}
	return false
}

func invalidSpeaker(cue domain.CaptionCue) bool {
	t := strings.TrimSpace(cue.Text)
	return strings.HasPrefix(t, "[说话人") || strings.HasPrefix(t, "【说话人") || strings.HasPrefix(t, "Speaker:")
}

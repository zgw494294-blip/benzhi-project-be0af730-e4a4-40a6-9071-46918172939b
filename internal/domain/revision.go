package domain

import (
	"strings"
	"time"
)

type CaptionCue struct {
	CueID       string `json:"cueID"`
	Sequence    int    `json:"sequence"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Text        string `json:"text"`
	Speaker     string `json:"speaker,omitempty"`
	LineCount   int    `json:"lineCount"`
}

func (c CaptionCue) ValidateShape() error {
	if c.Sequence <= 0 {
		return FieldError("sequence", "字幕序号必须大于零")
	}
	if c.StartMillis < 0 {
		return FieldError("startMillis", "开始时间不能小于零")
	}
	if c.EndMillis <= c.StartMillis {
		return FieldError("endMillis", "结束时间必须晚于开始时间")
	}
	return nil
}

type ValidationStatus string

const (
	ValidationPending ValidationStatus = "pending"
	ValidationFailed  ValidationStatus = "failed"
	ValidationPassed  ValidationStatus = "passed"
)

type CaptionRevision struct {
	ID               string           `json:"id"`
	ProjectID        string           `json:"projectID"`
	ParentRevisionID string           `json:"parentRevisionID,omitempty"`
	RevisionNumber   int              `json:"revisionNumber"`
	SubmittedBy      string           `json:"submittedBy"`
	SubmittedAt      time.Time        `json:"submittedAt"`
	ContentDigest    string           `json:"contentDigest"`
	Cues             []CaptionCue     `json:"cues"`
	ValidationStatus ValidationStatus `json:"validationStatus"`
}

func (r CaptionRevision) Validate() error {
	if r.ID == "" || r.ProjectID == "" {
		return FieldError("revision", "修订编号和项目编号不能为空")
	}
	if r.RevisionNumber <= 0 {
		return FieldError("revisionNumber", "修订序号必须大于零")
	}
	if strings.TrimSpace(r.SubmittedBy) == "" {
		return FieldError("submittedBy", "提交人不能为空")
	}
	if len(r.Cues) == 0 {
		return FieldError("cues", "至少需要一个字幕片段")
	}
	return nil
}

type IssueKind string

const (
	IssueOutOfBounds  IssueKind = "out_of_bounds"
	IssueOverlap      IssueKind = "overlap"
	IssueReadingSpeed IssueKind = "reading_speed"
	IssueEmptyText    IssueKind = "empty_text"
	IssueLineBreak    IssueKind = "line_break"
	IssueSpeaker      IssueKind = "speaker_marker"
)

type ValidationIssue struct {
	ID                  string    `json:"id"`
	RevisionID          string    `json:"revisionID"`
	CueID               string    `json:"cueID"`
	Kind                IssueKind `json:"kind"`
	Severity            string    `json:"severity"`
	Message             string    `json:"message"`
	Disposition         string    `json:"disposition,omitempty"`
	CoveredByRevisionID string    `json:"coveredByRevisionID,omitempty"`
	Resolved            bool      `json:"resolved"`
}

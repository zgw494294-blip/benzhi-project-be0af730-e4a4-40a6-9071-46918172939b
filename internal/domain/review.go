package domain

import (
	"strings"
	"time"
)

type DecisionType string

const (
	DecisionApprove DecisionType = "approve"
	DecisionReturn  DecisionType = "return"
)

type ReviewDecision struct {
	ID                    string       `json:"id"`
	ProjectID             string       `json:"projectID"`
	RevisionID            string       `json:"revisionID"`
	Reviewer              string       `json:"reviewer"`
	LanguageApproved      bool         `json:"languageApproved"`
	AccessibilityApproved bool         `json:"accessibilityApproved"`
	Decision              DecisionType `json:"decision"`
	Reason                string       `json:"reason,omitempty"`
	DecidedAt             time.Time    `json:"decidedAt"`
}

func (d ReviewDecision) Validate(submitter string) error {
	if strings.TrimSpace(d.Reviewer) == "" {
		return FieldError("reviewer", "审核员不能为空")
	}
	if d.Reviewer == submitter {
		return NewError(CodeForbidden, "提交者不能审核自己的修订")
	}
	if d.Decision != DecisionApprove && d.Decision != DecisionReturn {
		return FieldError("decision", "复核决定必须为 approve 或 return")
	}
	if d.Decision == DecisionApprove && (!d.LanguageApproved || !d.AccessibilityApproved) {
		return FieldError("decision", "批准必须同时通过语言准确性和无障碍适配")
	}
	if d.Decision == DecisionReturn && strings.TrimSpace(d.Reason) == "" {
		return FieldError("reason", "退回时必须填写理由")
	}
	return nil
}

type Manifest struct {
	ProjectID      string         `json:"projectID"`
	ProjectVersion int64          `json:"projectVersion"`
	Rules          ProjectRules   `json:"rules"`
	RevisionID     string         `json:"revisionID"`
	RevisionDigest string         `json:"revisionDigest"`
	Cues           []CaptionCue   `json:"cues"`
	Decision       ReviewDecision `json:"decision"`
	FrozenAt       time.Time      `json:"frozenAt"`
	FrozenBy       string         `json:"frozenBy"`
}

type DeliveryCredential struct {
	CredentialID     string    `json:"credentialID"`
	ProjectID        string    `json:"projectID"`
	RevisionID       string    `json:"revisionID"`
	ManifestDigest   string    `json:"manifestDigest"`
	IssuedBy         string    `json:"issuedBy"`
	IssuedAt         time.Time `json:"issuedAt"`
	VerificationCode string    `json:"verificationCode"`
}

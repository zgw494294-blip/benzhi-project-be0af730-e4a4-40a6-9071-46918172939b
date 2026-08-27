package workflow

import (
	"subtitle-review/internal/caption"
	"subtitle-review/internal/domain"
)

type CreateProjectCommand struct {
	ID                string  `json:"id"`
	Title             string  `json:"title"`
	DurationMillis    int64   `json:"durationMillis"`
	Language          string  `json:"language"`
	FrameRate         float64 `json:"frameRate"`
	MaxCharsPerSecond float64 `json:"maxCharsPerSecond"`
	DeliveryStandard  string  `json:"deliveryStandard"`
	Actor             string  `json:"actor"`
	IdempotencyKey    string  `json:"idempotencyKey"`
}

type SubmitRevisionCommand struct {
	ProjectID       string              `json:"projectID"`
	ExpectedVersion int64               `json:"expectedVersion"`
	SubmittedBy     string              `json:"submittedBy"`
	Cues            []domain.CaptionCue `json:"cues"`
	IdempotencyKey  string              `json:"idempotencyKey"`
}

type UpdateProjectRulesCommand struct {
	ProjectID         string  `json:"projectID"`
	ExpectedVersion   int64   `json:"expectedVersion"`
	Title             string  `json:"title"`
	DurationMillis    int64   `json:"durationMillis"`
	Language          string  `json:"language"`
	FrameRate         float64 `json:"frameRate"`
	MaxCharsPerSecond float64 `json:"maxCharsPerSecond"`
	DeliveryStandard  string  `json:"deliveryStandard"`
	Actor             string  `json:"actor"`
	IdempotencyKey    string  `json:"idempotencyKey"`
}

type PreflightRevisionCommand struct {
	ProjectID       string              `json:"projectID"`
	ExpectedVersion int64               `json:"expectedVersion"`
	SubmittedBy     string              `json:"submittedBy"`
	Cues            []domain.CaptionCue `json:"cues"`
}

type DispositionIssueCommand struct {
	ProjectID       string `json:"projectID"`
	ExpectedVersion int64  `json:"expectedVersion"`
	IssueID         string `json:"issueID"`
	Disposition     string `json:"disposition"`
	Actor           string `json:"actor"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type IssueDispositionInput struct {
	IssueID     string `json:"issueID"`
	Disposition string `json:"disposition"`
}

type BatchDispositionCommand struct {
	ProjectID       string                  `json:"projectID"`
	ExpectedVersion int64                   `json:"expectedVersion"`
	Items           []IssueDispositionInput `json:"items"`
	Actor           string                  `json:"actor"`
	IdempotencyKey  string                  `json:"idempotencyKey"`
}

type ReviewCommand struct {
	ProjectID             string              `json:"projectID"`
	ExpectedVersion       int64               `json:"expectedVersion"`
	Reviewer              string              `json:"reviewer"`
	LanguageApproved      bool                `json:"languageApproved"`
	AccessibilityApproved bool                `json:"accessibilityApproved"`
	Decision              domain.DecisionType `json:"decision"`
	Reason                string              `json:"reason"`
	IdempotencyKey        string              `json:"idempotencyKey"`
}

type FreezeCommand struct {
	ProjectID       string `json:"projectID"`
	ExpectedVersion int64  `json:"expectedVersion"`
	IssuedBy        string `json:"issuedBy"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type Verification struct {
	Valid      bool                      `json:"valid"`
	Message    string                    `json:"message"`
	Checks     []VerificationCheck       `json:"checks"`
	Credential domain.DeliveryCredential `json:"credential"`
	Manifest   domain.Manifest           `json:"manifest"`
	Timeline   []domain.AuditEvent       `json:"timeline"`
}

type VerificationCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

type PreflightIssue struct {
	domain.ValidationIssue
	CueSequence int `json:"cueSequence"`
}

type PreflightResult struct {
	ProjectID     string                   `json:"projectID"`
	Version       int64                    `json:"version"`
	Cues          []domain.CaptionCue      `json:"cues"`
	ContentDigest string                   `json:"contentDigest"`
	IssueCount    int                      `json:"issueCount"`
	IssueCounts   map[domain.IssueKind]int `json:"issueCounts"`
	Issues        []PreflightIssue         `json:"issues"`
	Passed        bool                     `json:"passed"`
}

type IssueCoverage struct {
	IssueID             string `json:"issueID"`
	OldCueID            string `json:"oldCueID"`
	NewCueID            string `json:"newCueID,omitempty"`
	CoveredByRevisionID string `json:"coveredByRevisionID,omitempty"`
	Resolved            bool   `json:"resolved"`
}

type RevisionDiff struct {
	ProjectID      string                  `json:"projectID"`
	ParentRevision *domain.CaptionRevision `json:"parentRevision,omitempty"`
	ChildRevision  domain.CaptionRevision  `json:"childRevision"`
	Changes        []caption.CueChange     `json:"changes"`
	Counts         map[string]int          `json:"counts"`
	IssueCoverage  []IssueCoverage         `json:"issueCoverage"`
}

type BatchDispositionResult struct {
	Aggregate *domain.ProjectAggregate `json:"aggregate"`
	Summary   domain.IssueSummary      `json:"summary"`
}

type ReviewHistoryItem struct {
	Decision       domain.ReviewDecision `json:"decision"`
	RevisionNumber int                   `json:"revisionNumber"`
}

type ReviewReadiness struct {
	AutomaticValidationPassed bool `json:"automaticValidationPassed"`
	CandidateIsSubmitter      bool `json:"candidateIsSubmitter"`
	ProjectAwaitingReview     bool `json:"projectAwaitingReview"`
}

type ReturnBasis struct {
	Available     bool                    `json:"available"`
	Decision      *domain.ReviewDecision  `json:"decision,omitempty"`
	Revision      *domain.CaptionRevision `json:"revision,omitempty"`
	Changes       []caption.CueChange     `json:"changes"`
	IssueCoverage []IssueCoverage         `json:"issueCoverage"`
}

type ReviewContext struct {
	ProjectID   string              `json:"projectID"`
	History     []ReviewHistoryItem `json:"history"`
	ReturnBasis ReturnBasis         `json:"returnBasis"`
	Readiness   ReviewReadiness     `json:"readiness"`
}

type ReadinessCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason,omitempty"`
	EntityID string `json:"entityID,omitempty"`
}

type ManifestPreview struct {
	ProjectID      string                `json:"projectID"`
	ProjectVersion int64                 `json:"projectVersion"`
	Rules          domain.ProjectRules   `json:"rules"`
	RevisionID     string                `json:"revisionID"`
	RevisionDigest string                `json:"revisionDigest"`
	Cues           []domain.CaptionCue   `json:"cues"`
	Decision       domain.ReviewDecision `json:"decision"`
}

type FreezeReadiness struct {
	ProjectID              string                     `json:"projectID"`
	Version                int64                      `json:"version"`
	Ready                  bool                       `json:"ready"`
	AlreadyFrozen          bool                       `json:"alreadyFrozen"`
	BlockerCount           int                        `json:"blockerCount"`
	Checks                 []ReadinessCheck           `json:"checks"`
	RevisionDigest         string                     `json:"revisionDigest,omitempty"`
	ManifestMaterialDigest string                     `json:"manifestMaterialDigest,omitempty"`
	ManifestPreview        *ManifestPreview           `json:"manifestPreview,omitempty"`
	Credential             *domain.DeliveryCredential `json:"credential,omitempty"`
}

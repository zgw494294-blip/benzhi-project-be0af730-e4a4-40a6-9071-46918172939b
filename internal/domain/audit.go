package domain

import "time"

type AuditEvent struct {
	Sequence       int64             `json:"sequence"`
	ProjectID      string            `json:"projectID"`
	EventType      string            `json:"eventType"`
	Actor          string            `json:"actor"`
	At             time.Time         `json:"at"`
	ProjectVersion int64             `json:"projectVersion"`
	Details        map[string]string `json:"details,omitempty"`
	PreviousDigest string            `json:"previousDigest,omitempty"`
	Digest         string            `json:"digest"`
}

type CommandResult struct {
	ProjectID string        `json:"projectID"`
	Version   int64         `json:"version"`
	EntityID  string        `json:"entityID,omitempty"`
	Status    ProjectStatus `json:"status"`
}

type IdempotencyRecord struct {
	Key           string        `json:"key"`
	Operation     string        `json:"operation"`
	RequestDigest string        `json:"requestDigest"`
	Result        CommandResult `json:"result"`
	CreatedAt     time.Time     `json:"createdAt"`
}

type ProjectAggregate struct {
	Project    CaptionProject      `json:"project"`
	Revisions  []CaptionRevision   `json:"revisions"`
	Issues     []ValidationIssue   `json:"issues"`
	Decisions  []ReviewDecision    `json:"decisions"`
	Manifest   *Manifest           `json:"manifest,omitempty"`
	Credential *DeliveryCredential `json:"credential,omitempty"`
	Timeline   []AuditEvent        `json:"timeline"`
}

func (a *ProjectAggregate) ActiveRevision() *CaptionRevision {
	for i := range a.Revisions {
		if a.Revisions[i].ID == a.Project.ActiveRevisionID {
			return &a.Revisions[i]
		}
	}
	return nil
}

func (a *ProjectAggregate) LatestDecision() *ReviewDecision {
	for i := len(a.Decisions) - 1; i >= 0; i-- {
		if a.Decisions[i].RevisionID == a.Project.ActiveRevisionID {
			return &a.Decisions[i]
		}
	}
	return nil
}

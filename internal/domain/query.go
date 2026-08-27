package domain

type ProjectQueueQuery struct {
	Status   ProjectStatus
	Language string
	Keyword  string
	Page     int
	PageSize int
}

type QueueSummary struct {
	Scope                  string                `json:"scope"`
	TotalProjects          int                   `json:"totalProjects"`
	StatusCounts           map[ProjectStatus]int `json:"statusCounts"`
	FixingUnresolvedIssues int                   `json:"fixingUnresolvedIssues"`
	ReviewProjects         int                   `json:"reviewProjects"`
}

type ProjectQueueResult struct {
	Projects []CaptionProject `json:"projects"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Summary  QueueSummary     `json:"summary"`
}

type IssueSummary struct {
	ByKind        map[IssueKind]int `json:"byKind"`
	Total         int               `json:"total"`
	Dispositioned int               `json:"dispositioned"`
	Unresolved    int               `json:"unresolved"`
	Covered       int               `json:"covered"`
}

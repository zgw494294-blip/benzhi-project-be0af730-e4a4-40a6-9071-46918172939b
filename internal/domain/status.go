package domain

import "fmt"

type ProjectStatus string

const (
	StatusDraft      ProjectStatus = "draft"
	StatusValidating ProjectStatus = "validating"
	StatusFixing     ProjectStatus = "fixing"
	StatusReview     ProjectStatus = "review"
	StatusApproved   ProjectStatus = "approved"
	StatusFrozen     ProjectStatus = "frozen"
)

var statusLabels = map[ProjectStatus]string{
	StatusDraft: "草稿", StatusValidating: "待校验", StatusFixing: "待整改",
	StatusReview: "待复核", StatusApproved: "已批准", StatusFrozen: "已冻结",
}

func (s ProjectStatus) Label() string { return statusLabels[s] }

func (s ProjectStatus) Valid() bool { _, ok := statusLabels[s]; return ok }

func ProjectStatuses() []ProjectStatus {
	return []ProjectStatus{StatusDraft, StatusValidating, StatusFixing, StatusReview, StatusApproved, StatusFrozen}
}

func EnsureMutable(s ProjectStatus) error {
	if s == StatusFrozen {
		return NewError(CodeFrozen, "项目已冻结，不能再进行业务变更")
	}
	return nil
}

func CanSubmitRevision(s ProjectStatus) bool {
	return s == StatusDraft || s == StatusFixing || s == StatusReview
}

func ValidateTransition(from, to ProjectStatus) error {
	allowed := map[ProjectStatus]map[ProjectStatus]bool{
		StatusDraft:      {StatusValidating: true},
		StatusValidating: {StatusFixing: true, StatusReview: true},
		StatusFixing:     {StatusValidating: true},
		StatusReview:     {StatusFixing: true, StatusApproved: true},
		StatusApproved:   {StatusFrozen: true},
	}
	if !allowed[from][to] {
		return NewError(CodeConflict, fmt.Sprintf("不允许从%s转换到%s", from.Label(), to.Label()))
	}
	return nil
}

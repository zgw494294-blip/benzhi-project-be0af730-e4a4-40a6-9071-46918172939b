package domain

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeInvalid      ErrorCode = "invalid_input"
	CodeNotFound     ErrorCode = "not_found"
	CodeConflict     ErrorCode = "conflict"
	CodeForbidden    ErrorCode = "forbidden"
	CodeFrozen       ErrorCode = "project_frozen"
	CodeStaleVersion ErrorCode = "stale_version"
	CodeIdempotency  ErrorCode = "idempotency_conflict"
	CodeIntegrity    ErrorCode = "integrity_error"
	CodeNotReady     ErrorCode = "not_ready"
)

type DomainError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

func (e *DomainError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Field)
}

func NewError(code ErrorCode, message string) error {
	return &DomainError{Code: code, Message: message}
}
func FieldError(field, message string) error {
	return &DomainError{Code: CodeInvalid, Message: message, Field: field}
}

func ErrorInfo(err error) (ErrorCode, string, string) {
	var de *DomainError
	if errors.As(err, &de) {
		return de.Code, de.Message, de.Field
	}
	return CodeIntegrity, "内部数据处理失败", ""
}

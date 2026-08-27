package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"subtitle-review/internal/domain"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求 JSON 无法解析", "")
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求只能包含一个 JSON 对象", "")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeDomainError(w http.ResponseWriter, err error) {
	code, message, field := domain.ErrorInfo(err)
	status := http.StatusUnprocessableEntity
	switch code {
	case domain.CodeNotFound:
		status = http.StatusNotFound
	case domain.CodeForbidden:
		status = http.StatusForbidden
	case domain.CodeConflict, domain.CodeFrozen, domain.CodeStaleVersion, domain.CodeIdempotency, domain.CodeNotReady:
		status = http.StatusConflict
	case domain.CodeIntegrity:
		status = http.StatusInternalServerError
	}
	writeError(w, status, string(code), message, field)
}

func writeError(w http.ResponseWriter, status int, code, message, field string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message, Field: field}})
}

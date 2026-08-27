package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"subtitle-review/internal/store"
	"subtitle-review/internal/workflow"
)

func TestIndexAndAPIError(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(workflow.NewService(repo)).Handler()
	index := httptest.NewRecorder()
	h.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), "<body>") {
		t.Fatalf("index status=%d", index.Code)
	}
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"unknown":true}`)))
	if bad.Code != http.StatusBadRequest || !strings.Contains(bad.Body.String(), "invalid_json") {
		t.Fatalf("bad response=%d %s", bad.Code, bad.Body.String())
	}
}

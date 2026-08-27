package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"subtitle-review/internal/workflow"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	workflow *workflow.Service
	assets   http.Handler
}

func NewServer(service *workflow.Service) *Server {
	sub, _ := fs.Sub(staticFiles, "static")
	return &Server{workflow: service, assets: http.FileServer(http.FS(sub))}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.ServeHTTP)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	if r.URL.Path == "/" && r.Method == http.MethodGet {
		s.assets.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/static/") && r.Method == http.MethodGet {
		http.StripPrefix("/static/", s.assets).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		HealthHandler(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	s.routeAPI(w, r)
}

func (s *Server) routeAPI(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(strings.TrimPrefix(r.URL.Path, "/api/"))
	if len(parts) == 1 && parts[0] == "projects" {
		if r.Method == http.MethodGet {
			s.ListProjectsHandler(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.CreateProjectHandler(w, r)
			return
		}
	}
	if len(parts) >= 2 && parts[0] == "projects" {
		projectID := parts[1]
		if len(parts) == 2 && r.Method == http.MethodGet {
			s.GetProjectHandler(w, r, projectID)
			return
		}
		if len(parts) == 3 && parts[2] == "revisions" && r.Method == http.MethodPost {
			s.SubmitRevisionHandler(w, r, projectID)
			return
		}
		if len(parts) == 3 && parts[2] == "rules" && (r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			s.UpdateProjectRulesHandler(w, r, projectID)
			return
		}
		if len(parts) == 4 && parts[2] == "revisions" && parts[3] == "preflight" && r.Method == http.MethodPost {
			s.PreflightRevisionHandler(w, r, projectID)
			return
		}
		if len(parts) == 5 && parts[2] == "revisions" && parts[4] == "diff" && r.Method == http.MethodGet {
			s.RevisionDiffHandler(w, r, projectID, parts[3])
			return
		}
		if len(parts) == 5 && parts[2] == "issues" && parts[4] == "disposition" && r.Method == http.MethodPost {
			s.DispositionIssueHandler(w, r, projectID, parts[3])
			return
		}
		if len(parts) == 4 && parts[2] == "issues" && parts[3] == "batch-disposition" && r.Method == http.MethodPost {
			s.BatchDispositionHandler(w, r, projectID)
			return
		}
		if len(parts) == 3 && parts[2] == "reviews" && r.Method == http.MethodPost {
			s.ReviewHandler(w, r, projectID)
			return
		}
		if len(parts) == 3 && parts[2] == "freeze" && r.Method == http.MethodPost {
			s.FreezeHandler(w, r, projectID)
			return
		}
		if len(parts) == 3 && parts[2] == "review-context" && r.Method == http.MethodGet {
			s.ReviewContextHandler(w, r, projectID)
			return
		}
		if len(parts) == 3 && parts[2] == "freeze-readiness" && r.Method == http.MethodGet {
			s.FreezeReadinessHandler(w, r, projectID)
			return
		}
	}
	if len(parts) == 2 && parts[0] == "verify" && r.Method == http.MethodGet {
		s.VerifyCredentialHandler(w, r, parts[1])
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "API 路由不存在", "")
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

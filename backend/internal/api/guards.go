package api

import (
	"net/http"
	"strings"
	"time"

	"dockerpilot/internal/audit"
	"dockerpilot/internal/envx"
	"dockerpilot/internal/rbac"

	"github.com/go-chi/chi/v5/middleware"
)

func hostHeader(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get("X-Dockerpilot-Host")); h != "" {
		return h
	}
	return strings.TrimSpace(r.Header.Get("X-Dockpanel-Host"))
}

func (s *Server) readonlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !envx.ReadOnly() || !isMutating(r.Method) || isReadonlyExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"modo somente leitura (DOCKERPILOT_READONLY)"}`))
	})
}

func (s *Server) rbacMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := ""
		if u := userFromContext(r); u != nil {
			role = u.Role
		}
		if rbac.Allow(role, r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"permissão insuficiente para este papel"}`))
	})
}

func (s *Server) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if s.Audit == nil || !isMutating(r.Method) || strings.Contains(r.URL.Path, "/api/auth/") {
			return
		}
		ev := audit.Event{
			At:     time.Now().UTC(),
			Action: r.Method + " " + r.URL.Path,
			Status: ww.Status(),
			Host:   hostHeader(r),
		}
		if ev.Status == 0 {
			ev.Status = http.StatusOK
		}
		if u := userFromContext(r); u != nil {
			ev.Actor = u.Email
			ev.OrgID = u.OrgID
			if u.KeyID != "" {
				ev.Detail = "key:" + u.KeyID
			}
		}
		_ = s.Audit.Append(ev)
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isReadonlyExempt(path string) bool {
	switch path {
	case "/api/auth/login", "/api/auth/logout", "/api/auth/sso/exchange", "/api/client-errors":
		return true
	default:
		return false
	}
}

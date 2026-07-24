package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"dockpanel/internal/observability"

	"github.com/google/uuid"
)

type reqIDKey struct{}

func requestIDFromContext(ctx context.Context) string {
	s, _ := ctx.Value(reqIDKey{}).(string)
	return s
}

func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), reqIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			attrs := map[string]string{
				"request_id": requestIDFromContext(r.Context()),
				"method":     r.Method,
				"path":       r.URL.Path,
				"stack":      string(debug.Stack()),
			}
			if u := userFromContext(r); u != nil {
				attrs["user_id"] = u.ID
			}
			observability.CapturePanic(r.Context(), rec, attrs)
			slog.Error("panic",
				"request_id", attrs["request_id"],
				"method", r.Method,
				"path", r.URL.Path,
				"panic", fmt.Sprint(rec),
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"erro interno"}`))
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ok := true
	services := map[string]string{"api": "up"}
	if s.Hosts != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		hostsUp := 0
		for _, h := range s.Hosts.List() {
			cli, err := s.Hosts.Get(h.ID)
			if err != nil {
				continue
			}
			if err := cli.Ping(ctx); err == nil {
				hostsUp++
			}
		}
		if hostsUp == 0 && len(s.Hosts.List()) > 0 {
			services["docker"] = "down"
			ok = false
		} else if len(s.Hosts.List()) == 0 {
			services["docker"] = "unconfigured"
		} else {
			services["docker"] = "up"
		}
	}
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": ok, "services": services})
}

func (s *Server) healthDetailed(w http.ResponseWriter, r *http.Request) {
	s.health(w, r)
}

func (s *Server) clientErrorLog(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	slog.Warn("client_error",
		"request_id", requestIDFromContext(r.Context()),
		"body", body,
	)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) openAPISpec(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]string{"title": "Dockpanel API", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/health":      map[string]interface{}{"get": map[string]string{"summary": "Health"}},
			"/auth/login":  map[string]interface{}{"post": map[string]string{"summary": "Login"}},
			"/containers/": map[string]interface{}{"get": map[string]string{"summary": "List containers"}},
		},
	})
}

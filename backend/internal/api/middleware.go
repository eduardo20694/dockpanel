package api

import (
	"context"
	"net/http"
	"strings"

	"dockpanel/internal/auth"
)

type ctxKey int

const userCtxKey ctxKey = 1

func userFromContext(r *http.Request) *auth.User {
	u, _ := r.Context().Value(userCtxKey).(*auth.User)
	return u
}

func tokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(auth.CookieName()); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Auth == nil || !s.Auth.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		if isPublicRoute(path) {
			// Still attach user when cookie present (me/logout).
			if tok := tokenFromRequest(r); tok != "" {
				if claims, err := s.Auth.ParseToken(tok); err == nil {
					u := s.Auth.UserFromClaims(claims)
					r = r.WithContext(context.WithValue(r.Context(), userCtxKey, &u))
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		claims, err := s.Auth.ParseToken(tokenFromRequest(r))
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"não autenticado"}`))
			return
		}
		u := s.Auth.UserFromClaims(claims)
		ctx := context.WithValue(r.Context(), userCtxKey, &u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isPublicRoute(path string) bool {
	switch path {
	case "/api/health", "/api/health/detailed", "/api/openapi.json",
		"/api/auth/login", "/api/auth/config",
		"/api/auth/me", "/api/auth/logout",
		"/api/client-errors":
		return true
	default:
		return false
	}
}

package api

import (
	"context"
	"net/http"
	"strings"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/auth"
)

type ctxKey int

const userCtxKey ctxKey = 1

func userFromContext(r *http.Request) *auth.User {
	u, _ := r.Context().Value(userCtxKey).(*auth.User)
	return u
}

func tokenFromRequest(r *http.Request) string {
	for _, name := range []string{auth.CookieName(), auth.LegacyCookieName()} {
		if c, err := r.Cookie(name); err == nil && c.Value != "" {
			return c.Value
		}
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

		token := tokenFromRequest(r)
		if claims, err := s.Auth.ParseToken(token); err == nil {
			u := s.Auth.UserFromClaims(claims)
			ctx := context.WithValue(r.Context(), userCtxKey, &u)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if s.Keys != nil {
			if p, err := s.Keys.Authenticate(token); err == nil {
				s.Keys.Touch(p.KeyID)
				u := userFromPrincipal(p)
				ctx := context.WithValue(r.Context(), userCtxKey, &u)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"não autenticado"}`))
	})
}

func userFromPrincipal(p apikey.Principal) auth.User {
	return auth.User{
		ID: p.UserID, Email: p.Name, Name: p.Name, Role: p.Role, OrgID: p.OrgID,
		KeyID: p.KeyID, Stacks: p.Stacks, Hosts: p.Hosts, Guest: p.Guest,
	}
}

func isPublicRoute(path string) bool {
	switch path {
	case "/api/health", "/api/health/detailed", "/api/openapi.json",
		"/api/auth/login", "/api/auth/config", "/api/auth/sso", "/api/auth/sso/exchange",
		"/api/auth/me", "/api/auth/logout",
		"/api/gitops/webhook",
		"/api/client-errors",
		"/metrics":
		return true
	default:
		return false
	}
}

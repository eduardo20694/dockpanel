package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"dockpanel/internal/auth"
)

func (s *Server) authConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"enabled": s.Auth != nil && s.Auth.Enabled})
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || !s.Auth.Enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "autenticação desabilitada — configure DATABASE_URL"})
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "corpo inválido", http.StatusBadRequest)
		return
	}
	token, user, err := s.Auth.Login(body.Email, body.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email ou senha incorretos"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  auth.TokenExpiry(),
	})
	writeJSON(w, map[string]interface{}{"user": user})
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || !s.Auth.Enabled {
		writeJSON(w, map[string]interface{}{"authEnabled": false, "user": nil})
		return
	}
	if u := userFromContext(r); u != nil {
		writeJSON(w, map[string]interface{}{"authEnabled": true, "user": u})
		return
	}
	token := tokenFromRequest(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"authEnabled": true, "user": nil})
		return
	}
	claims, err := s.Auth.ParseToken(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"authEnabled": true, "user": nil})
		return
	}
	writeJSON(w, map[string]interface{}{
		"authEnabled": true,
		"user": auth.User{
			ID: claims.UserID, Email: claims.Email, Name: claims.Name, Role: claims.Role,
		},
	})
}

func tokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(auth.CookieName()); err == nil && c.Value != "" {
		return c.Value
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

type ctxKey int

const userCtxKey ctxKey = 1

func userFromContext(r *http.Request) *auth.User {
	u, _ := r.Context().Value(userCtxKey).(*auth.User)
	return u
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Auth == nil || !s.Auth.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicRoute(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		token := tokenFromRequest(r)
		claims, err := s.Auth.ParseToken(token)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "não autenticado"})
			return
		}
		u := &auth.User{ID: claims.UserID, Email: claims.Email, Name: claims.Name, Role: claims.Role}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, u)))
	})
}

func isPublicRoute(path string) bool {
	switch path {
	case "/api/health", "/api/auth/login", "/api/auth/config", "/api/auth/me", "/api/auth/logout":
		return true
	default:
		return false
	}
}

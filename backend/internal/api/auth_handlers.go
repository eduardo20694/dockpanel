package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"dockpanel/internal/auth"
)

func (s *Server) authConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"enabled": s.Auth != nil && s.Auth.Enabled})
}

func setAuthCookies(w http.ResponseWriter, r *http.Request, access string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name: auth.CookieName(), Value: access, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: secure, Expires: auth.TokenExpiry(),
	})
}

func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: auth.CookieName(), Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
}

func (s *Server) checkLoginRate(w http.ResponseWriter, r *http.Request) bool {
	key := "login:" + clientIP(r)
	n := rateLimitLocal(key)
	if n > 20 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "muitas tentativas — aguarde 1 minuto"})
		return false
	}
	return true
}

var (
	localRatesMu sync.Mutex
	localRates   = map[string]struct {
		n int64
		t time.Time
	}{}
)

func rateLimitLocal(key string) int64 {
	localRatesMu.Lock()
	defer localRatesMu.Unlock()
	e, ok := localRates[key]
	if !ok || time.Since(e.t) > time.Minute {
		localRates[key] = struct {
			n int64
			t time.Time
		}{1, time.Now()}
		return 1
	}
	e.n++
	localRates[key] = e
	return e.n
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || !s.Auth.Enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "autenticação desabilitada — configure DOCKPANEL_ADMIN_EMAIL e DOCKPANEL_ADMIN_PASSWORD"})
		return
	}
	if !s.checkLoginRate(w, r) {
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
	res, err := s.Auth.Login(body.Email, body.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Email ou senha incorretos"})
		return
	}
	setAuthCookies(w, r, res.AccessToken)
	writeJSON(w, map[string]interface{}{"user": res.User})
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookies(w)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || !s.Auth.Enabled {
		writeJSON(w, map[string]interface{}{"user": nil})
		return
	}
	u := userFromContext(r)
	if u == nil {
		writeJSON(w, map[string]interface{}{"user": nil})
		return
	}
	writeJSON(w, map[string]interface{}{"user": u})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := len(xff); i > 0 {
			parts := splitComma(xff)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	host := r.RemoteAddr
	if i := lastColon(host); i >= 0 {
		return host[:i]
	}
	return host
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, trimSpace(s[start:i]))
			start = i + 1
		}
	}
	out = append(out, trimSpace(s[start:]))
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

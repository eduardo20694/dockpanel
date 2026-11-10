package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeJSON(w, map[string]any{"keys": []any{}})
		return
	}
	orgID := orgFrom(r)
	list, err := s.Keys.List(orgID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"keys": list})
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeErr(w, simpleError("chaves indisponíveis"))
		return
	}
	var body struct {
		Name    string `json:"name"`
		UserID  string `json:"userId"`
		Role    string `json:"role"`
		TTLDays int    `json:"ttlDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	orgID := orgFrom(r)
	userID := body.UserID
	role := body.Role
	if u := userFromContext(r); u != nil {
		if userID == "" {
			userID = u.ID
		}
		if role == "" {
			role = u.Role
		}
	}
	if role == "" {
		role = "operator"
	}
	ttl := time.Duration(body.TTLDays) * 24 * time.Hour
	token, key, err := s.Keys.Issue(orgID, userID, body.Name, role, ttl)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"token": token, "key": key})
}

func (s *Server) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeErr(w, simpleError("chaves indisponíveis"))
		return
	}
	if err := s.Keys.Revoke(orgFrom(r), chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func orgFrom(r *http.Request) string {
	if u := userFromContext(r); u != nil && u.OrgID != "" {
		return u.OrgID
	}
	return "default"
}

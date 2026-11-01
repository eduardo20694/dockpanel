package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"dockerpilot/internal/billing"
	"dockerpilot/internal/seats"

	"github.com/go-chi/chi/v5"
)

func (s *Server) billingStatus(w http.ResponseWriter, r *http.Request) {
	orgID := "default"
	if u := userFromContext(r); u != nil && u.OrgID != "" {
		orgID = u.OrgID
	}
	sub := seats.Subscription{OrgID: orgID, Plan: "individual", Seats: 1, Status: "active"}
	if s.Seats != nil {
		sub = s.Seats.Get(orgID)
	}
	used := 1
	if s.Orgs != nil {
		used += len(s.Orgs.Users(orgID))
	}
	writeJSON(w, map[string]any{
		"subscription": sub,
		"used":         used,
		"provider":     providerName(s.Billing),
		"plans": []map[string]any{
			{"id": "individual", "name": "Individual", "seats": 1, "blurb": "Um operador, uma chave."},
			{"id": "organization", "name": "Organização", "seats": "por assento", "blurb": "Admin convida o time e emite as chaves."},
		},
	})
}

func (s *Server) billingCheckout(w http.ResponseWriter, r *http.Request) {
	if s.Billing == nil {
		writeErr(w, billing.ErrNotConfigured)
		return
	}
	var body struct {
		Plan  string `json:"plan"`
		Seats int    `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if body.Plan == "" {
		body.Plan = "organization"
	}
	if body.Seats < 1 {
		body.Seats = 1
	}
	orgID := "default"
	email := ""
	if u := userFromContext(r); u != nil {
		if u.OrgID != "" {
			orgID = u.OrgID
		}
		email = u.Email
	}
	url, err := s.Billing.CreateCheckout(r.Context(), billing.CheckoutRequest{
		OrgID: orgID, Email: email, Plan: body.Plan, Seats: body.Seats,
		SuccessURL: "http://localhost:5173/billing?ok=1",
		CancelURL:  "http://localhost:5173/billing?cancel=1",
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"url": url})
}

func (s *Server) billingWebhook(w http.ResponseWriter, r *http.Request) {
	if s.Billing == nil || s.Seats == nil {
		writeErr(w, billing.ErrNotConfigured)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, `{"error":"corpo inválido"}`, http.StatusBadRequest)
		return
	}
	ev, err := s.Billing.ParseWebhook(raw, r.Header.Get("Stripe-Signature"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	sub := s.Seats.Get(ev.OrgID)
	sub.OrgID = ev.OrgID
	sub.Plan = ev.Plan
	sub.Seats = ev.Seats
	sub.Status = ev.Status
	sub.Provider = providerName(s.Billing)
	sub.CustomerID = ev.CustomerID
	sub.SubscriptionID = ev.SubscriptionID
	if err := s.Seats.Save(sub); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "subscription": s.Seats.Get(ev.OrgID)})
}

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

func providerName(p billing.Provider) string {
	if p == nil {
		return ""
	}
	return p.Name()
}

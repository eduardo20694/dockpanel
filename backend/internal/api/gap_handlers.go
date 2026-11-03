package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"dockerpilot/internal/auth"
	"dockerpilot/internal/composeedit"
	"dockerpilot/internal/files"
	"dockerpilot/internal/metricsx"
	"dockerpilot/internal/notify"
	"dockerpilot/internal/observability"
	"dockerpilot/internal/scan"
)

func (s *Server) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	hosts, containers := 0, 0
	if s.Hosts != nil {
		hosts = len(s.Hosts.List())
	}
	metricsx.Handler(metricsx.Snapshot{Hosts: hosts, Containers: containers, Alerts: 0, Deploys: 0}).ServeHTTP(w, r)
}

func (s *Server) observabilityStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"metricsPath": "/metrics",
		"otel":        observability.OTLPEndpoint() != "",
		"grafana":     "docs/grafana/dockpilot.json",
		"sample":      metricsx.Text(metricsx.Snapshot{Hosts: 1, Containers: 1, Alerts: 0, Deploys: 0}),
	})
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	if s.Agents == nil {
		writeJSON(w, map[string]any{"agents": []any{}})
		return
	}
	writeJSON(w, map[string]any{"agents": s.Agents.List()})
}

func (s *Server) enrollAgent(w http.ResponseWriter, r *http.Request) {
	if s.Agents == nil {
		writeErr(w, simpleError("agentes indisponíveis"))
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	token, rec, err := s.Agents.Enroll(body.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"token": token, "agent": rec})
}

func (s *Server) revokeAgent(w http.ResponseWriter, r *http.Request) {
	if s.Agents == nil {
		writeErr(w, simpleError("agentes indisponíveis"))
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := s.Agents.Revoke(body.ID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) identityStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"providers": []map[string]string{
			{"id": "google", "name": "Google"},
			{"id": "entra", "name": "Microsoft Entra"},
			{"id": "keycloak", "name": "Keycloak"},
			{"id": "generic", "name": "OIDC genérico"},
			{"id": "ldap", "name": "LDAP / Active Directory"},
		},
		"mfa":      []string{"totp", "passkey"},
		"ldap":     ldapOn(),
		"passkeys": 0,
	})
}

func ldapOn() bool {
	_, _, _, ok := auth.LDAPConfig()
	return ok
}

func (s *Server) sarifExport(w http.ResponseWriter, r *http.Request) {
	var body scan.Report
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	raw, err := scan.ToSARIF(body)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/sarif+json")
	_, _ = w.Write(raw)
}

func (s *Server) cvePolicy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current int  `json:"current"`
		Next    int  `json:"next"`
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	blocked := scan.BlockCriticalIncrease(body.Current, body.Next, body.Enabled)
	writeJSON(w, map[string]any{"blocked": blocked})
}

func (s *Server) composePreview(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var wrap struct {
		YAML string `json:"yaml"`
	}
	text := string(raw)
	if json.Unmarshal(raw, &wrap) == nil && wrap.YAML != "" {
		text = wrap.YAML
	}
	svcs, err := composeedit.Parse(text)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"services": svcs, "yaml": composeedit.Render(svcs)})
}

func (s *Server) composeRender(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Services []composeedit.Service `json:"services"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	s.auditAccess(r, "compose edit", strings.Join(names(body.Services), ","))
	writeJSON(w, map[string]string{"yaml": composeedit.Render(body.Services)})
}

func names(svcs []composeedit.Service) []string {
	out := make([]string, 0, len(svcs))
	for _, s := range svcs {
		out = append(out, s.Name)
	}
	return out
}

func (s *Server) browseFiles(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("listing")
	if raw == "" {
		raw = "total 1\n-rw-r--r-- 1 0 0 4 Jan 1 00:00 readme.txt\n"
	}
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "/"
	}
	s.auditAccess(r, "files read", r.URL.Query().Get("container")+" "+dir)
	writeJSON(w, map[string]any{"entries": files.ParseLS(dir, raw)})
}

func (s *Server) notifyRules(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var body struct {
			Rules []notify.Rule `json:"rules"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
			return
		}
		s.rules = body.Rules
		writeJSON(w, map[string]any{"rules": s.rules})
		return
	}
	if s.rules == nil {
		s.rules = []notify.Rule{}
	}
	writeJSON(w, map[string]any{
		"rules":    s.rules,
		"channels": []string{"slack", "teams", "telegram", "discord", "email", "webhook", "pagerduty"},
	})
}

func (s *Server) notifyPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Event    string `json:"event"`
		Severity string `json:"severity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	ch, limited := notify.Channels(s.rules, body.Event, body.Severity)
	writeJSON(w, map[string]any{"channels": ch, "limited": limited})
}

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"dockerpilot/internal/audit"
	"dockerpilot/internal/deploy"
	"dockerpilot/internal/gitops"
	"dockerpilot/internal/report"
	"dockerpilot/internal/secrets"
	"dockerpilot/internal/templates"
	"dockerpilot/internal/updates"

	"github.com/go-chi/chi/v5"
)

func actorOf(r *http.Request) string {
	if u := userFromContext(r); u != nil && u.Email != "" {
		return u.Email
	}
	return "api"
}

func orgOf(r *http.Request) string {
	if u := userFromContext(r); u != nil && u.OrgID != "" {
		return u.OrgID
	}
	return "default"
}

func (s *Server) gitopsStatus(w http.ResponseWriter, r *http.Request) {
	if s.GitOps == nil {
		writeJSON(w, gitops.Status{Stacks: []gitops.StackPublic{}, History: []gitops.Event{}})
		return
	}
	writeJSON(w, s.GitOps.Status())
}

func (s *Server) gitopsGlobal(w http.ResponseWriter, r *http.Request) {
	if s.GitOps == nil {
		writeErr(w, simpleError("gitops indisponível"))
		return
	}
	var body struct {
		Enabled       *bool   `json:"enabled"`
		PollSeconds   *int    `json:"pollSeconds"`
		WebhookSecret *string `json:"webhookSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := s.GitOps.SetGlobal(gitops.GlobalInput{Enabled: body.Enabled, PollSeconds: body.PollSeconds, WebhookSecret: body.WebhookSecret}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, s.GitOps.Status())
}

func (s *Server) gitopsUpsert(w http.ResponseWriter, r *http.Request) {
	if s.GitOps == nil {
		writeErr(w, simpleError("gitops indisponível"))
		return
	}
	var body struct {
		Enabled     *bool   `json:"enabled"`
		RepoURL     string  `json:"repoUrl"`
		Branch      string  `json:"branch"`
		ComposePath string  `json:"composePath"`
		Credential  *string `json:"credential"`
		Mode        string  `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	err := s.GitOps.Upsert(gitops.LinkInput{
		Stack: chi.URLParam(r, "stack"), OrgID: orgOf(r), Enabled: body.Enabled,
		RepoURL: body.RepoURL, Branch: body.Branch, ComposePath: body.ComposePath,
		Credential: body.Credential, Mode: body.Mode,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, s.GitOps.Status())
}

func (s *Server) gitopsSync(w http.ResponseWriter, r *http.Request) {
	s.gitopsRun(w, r, "manual")
}

func (s *Server) gitopsApprove(w http.ResponseWriter, r *http.Request) {
	s.gitopsRun(w, r, "approve")
}

func (s *Server) gitopsRun(w http.ResponseWriter, r *http.Request, trigger string) {
	if s.GitOps == nil {
		writeErr(w, simpleError("gitops indisponível"))
		return
	}
	ev, err := s.GitOps.Sync(r.Context(), chi.URLParam(r, "stack"), trigger, actorOf(r), s.Audit)
	if err != nil && ev.Status == "" {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"event": ev, "error": errString(err), "status": s.GitOps.Status()})
}

func (s *Server) gitopsDiff(w http.ResponseWriter, r *http.Request) {
	if s.GitOps == nil {
		writeErr(w, simpleError("gitops indisponível"))
		return
	}
	diff, err := s.GitOps.Preview(r.Context(), chi.URLParam(r, "stack"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"diff": diff})
}

func (s *Server) gitopsWebhook(w http.ResponseWriter, r *http.Request) {
	if s.GitOps == nil {
		writeErr(w, simpleError("gitops indisponível"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, `{"error":"corpo inválido"}`, http.StatusBadRequest)
		return
	}
	n, err := s.GitOps.Webhook(r.Context(), body, r.Header.Get("X-Hub-Signature-256"), r.Header.Get("X-Gitlab-Signature-256"), r.Header.Get("X-Gitlab-Token"), "webhook", s.Audit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"matched": n})
}

func (s *Server) listSecrets(w http.ResponseWriter, r *http.Request) {
	if s.Secrets == nil {
		writeJSON(w, map[string]any{"secrets": []any{}})
		return
	}
	s.auditAccess(r, "secrets read", r.URL.Query().Get("stack"))
	writeJSON(w, map[string]any{"secrets": s.Secrets.List(orgOf(r), r.URL.Query().Get("stack"))})
}

func (s *Server) putSecret(w http.ResponseWriter, r *http.Request) {
	if s.Secrets == nil {
		writeErr(w, simpleError("cofre indisponível"))
		return
	}
	var body struct {
		Stack  string `json:"stack"`
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Target string `json:"target"`
		Value  string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	meta, err := s.Secrets.Put(orgOf(r), body.Stack, body.Name, body.Kind, body.Target, body.Value)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.auditAccess(r, "secrets write", body.Stack+" "+body.Name)
	writeJSON(w, map[string]any{"secret": meta})
}

func (s *Server) deleteSecret(w http.ResponseWriter, r *http.Request) {
	if s.Secrets == nil {
		writeErr(w, simpleError("cofre indisponível"))
		return
	}
	if err := s.Secrets.Delete(orgOf(r), chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) scanSecrets(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Compose string `json:"compose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"findings": secrets.ScanCompose(body.Compose)})
}

func (s *Server) updatesStatus(w http.ResponseWriter, r *http.Request) {
	if s.Updates == nil {
		writeJSON(w, map[string]any{"items": []any{}, "history": []any{}, "policy": updates.Policy{Mode: "approval"}})
		return
	}
	items, hist, pol := s.Updates.Snapshot()
	writeJSON(w, map[string]any{"items": items, "history": hist, "policy": pol})
}

func (s *Server) updatesPolicy(w http.ResponseWriter, r *http.Request) {
	if s.Updates == nil {
		writeErr(w, simpleError("atualizações indisponíveis"))
		return
	}
	var body updates.Policy
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := s.Updates.SetPolicy(body); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, s.Updates.Policy())
}

func (s *Server) updatesCheck(w http.ResponseWriter, r *http.Request) {
	if s.Updates == nil {
		writeErr(w, simpleError("atualizações indisponíveis"))
		return
	}
	items, err := s.Updates.Check(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"items": items})
}

func (s *Server) updatesApply(w http.ResponseWriter, r *http.Request) {
	if s.Updates == nil {
		writeErr(w, simpleError("atualizações indisponíveis"))
		return
	}
	var body struct {
		Container string `json:"container"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	res, err := s.Updates.Apply(r.Context(), body.Container, actorOf(r), s.Audit)
	writeJSON(w, map[string]any{"result": res, "error": errString(err)})
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeJSON(w, map[string]any{"sessions": []any{}})
		return
	}
	list, err := s.Keys.List(orgOf(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	var guests []any
	for _, k := range list {
		if k.Guest {
			guests = append(guests, k)
		}
	}
	if guests == nil {
		guests = []any{}
	}
	writeJSON(w, map[string]any{"sessions": guests})
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeErr(w, simpleError("chaves indisponíveis"))
		return
	}
	var body struct {
		Name   string   `json:"name"`
		Role   string   `json:"role"`
		Hours  int      `json:"hours"`
		Stacks []string `json:"stacks"`
		Hosts  []string `json:"hosts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	token, key, err := s.Keys.IssueGuest(orgOf(r), body.Name, body.Role, body.Hours, body.Stacks, body.Hosts)
	if err != nil {
		writeErr(w, err)
		return
	}
	if s.Audit != nil {
		_ = s.Audit.Append(audit.Event{
			At: time.Now().UTC(), Actor: actorOf(r), OrgID: orgOf(r),
			Action: "guest invite " + key.Name, Status: 200, Detail: "key:" + key.ID,
		})
	}
	writeJSON(w, map[string]any{"token": token, "session": key})
}

func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeErr(w, simpleError("chaves indisponíveis"))
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.Keys.Revoke(orgOf(r), id); err != nil {
		writeErr(w, err)
		return
	}
	if s.Audit != nil {
		_ = s.Audit.Append(audit.Event{
			At: time.Now().UTC(), Actor: actorOf(r), OrgID: orgOf(r),
			Action: "guest revoke", Status: 200, Detail: "key:" + id,
		})
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) sessionTimeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var events []audit.Event
	if s.Audit != nil {
		all, err := s.Audit.List(500)
		if err != nil {
			writeErr(w, err)
			return
		}
		for _, ev := range all {
			if strings.Contains(ev.Detail, "key:"+id) {
				events = append(events, ev)
			}
		}
	}
	if events == nil {
		events = []audit.Event{}
	}
	writeJSON(w, map[string]any{"events": events})
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	list := templates.Catalog()
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		out = append(out, map[string]any{"id": t.ID, "name": t.Name, "summary": t.Summary, "vars": t.Vars})
	}
	writeJSON(w, map[string]any{"templates": out})
}

func (s *Server) deployTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Stack       string            `json:"stack"`
		ProjectPath string            `json:"projectPath"`
		Vars        map[string]string `json:"vars"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if u := userFromContext(r); u != nil && len(u.Stacks) > 0 && !containsStr(u.Stacks, body.Stack) {
		http.Error(w, `{"error":"stack fora do escopo"}`, http.StatusForbidden)
		return
	}
	prep, err := templates.Prepare(templates.Request{
		ID: chi.URLParam(r, "id"), Stack: body.Stack, ProjectPath: body.ProjectPath, Vars: body.Vars,
	}, s.dataDir())
	if err != nil {
		writeErr(w, err)
		return
	}
	for name, value := range prep.Secrets {
		if s.Secrets != nil {
			if _, err := s.Secrets.Put(orgOf(r), body.Stack, name, "env", name, value); err != nil {
				writeErr(w, err)
				return
			}
		}
	}
	s.auditAccess(r, "secrets inject", body.Stack)
	res, err := deploy.RunCompose(r.Context(), deploy.ComposeRequest{
		ProjectPath: prep.Dir, Action: "up", Detach: true, ExtraEnv: prep.Env,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "result": res, "dir": prep.Dir})
		return
	}
	writeJSON(w, map[string]any{"result": res, "dir": prep.Dir})
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title      string `json:"title"`
		Session    string `json:"session"`
		Incident   string `json:"incident"`
		FinalState string `json:"finalState"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	in := report.Input{
		Title: body.Title, Session: body.Session, Incident: body.Incident, FinalState: body.FinalState,
		When: time.Now().UTC(),
	}
	if s.Audit != nil && body.Session != "" {
		all, _ := s.Audit.List(500)
		for _, ev := range all {
			if strings.Contains(ev.Detail, "key:"+body.Session) {
				in.Actions = append(in.Actions, ev.At.Format(time.RFC3339)+" "+ev.Action)
			}
		}
	}
	id, md, _, err := report.Save(s.dataDir(), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"id": id, "markdown": md})
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	raw, err := report.Load(s.dataDir(), chi.URLParam(r, "id"), "md")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"markdown": string(raw)})
}

func (s *Server) getReportPDF(w http.ResponseWriter, r *http.Request) {
	raw, err := report.Load(s.dataDir(), chi.URLParam(r, "id"), "pdf")
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="dockpilot-relatorio.pdf"`)
	_, _ = w.Write(raw)
}

func (s *Server) auditAccess(r *http.Request, action, detail string) {
	if s.Audit == nil {
		return
	}
	ev := audit.Event{At: time.Now().UTC(), Action: action, Status: 200, Detail: detail, OrgID: orgOf(r)}
	if u := userFromContext(r); u != nil {
		ev.Actor = u.Email
		if u.KeyID != "" {
			ev.Detail = strings.TrimSpace(detail + " key:" + u.KeyID)
		}
	}
	_ = s.Audit.Append(ev)
}

func (s *Server) injectSecrets(r *http.Request, stack string, req *deploy.ComposeRequest) {
	if s.Secrets == nil || stack == "" {
		return
	}
	env, files, err := s.Secrets.Materialize(orgOf(r), stack)
	if err != nil || (len(env) == 0 && len(files) == 0) {
		return
	}
	for k, v := range env {
		req.ExtraEnv = append(req.ExtraEnv, k+"="+v)
	}
	req.SecretFiles = files
	s.auditAccess(r, "secrets inject", stack)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func containsStr(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

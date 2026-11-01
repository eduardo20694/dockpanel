package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockerpilot/internal/auth"
	"dockerpilot/internal/cost"
	"dockerpilot/internal/deploy"
	"dockerpilot/internal/envx"
	"dockerpilot/internal/org"
	"dockerpilot/internal/proxy"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/seats"
	"dockerpilot/internal/triage"

	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
)

func (s *Server) authSSOConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, auth.SSOConfig())
}

func (s *Server) authSSOExchange(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil {
		writeErr(w, auth.ErrAuthDisabled)
		return
	}
	var body struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.IDToken) == "" {
		http.Error(w, `{"error":"idToken obrigatório"}`, http.StatusBadRequest)
		return
	}
	u, err := s.Auth.ExchangeSSO(body.IDToken)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	tok, err := s.Auth.Issue(u)
	if err != nil {
		writeErr(w, err)
		return
	}
	setAuthCookies(w, r, tok)
	writeJSON(w, map[string]any{"user": u})
}

func (s *Server) readonlyStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"readOnly": envx.ReadOnly()})
}

func (s *Server) listResolvedProblems(w http.ResponseWriter, r *http.Request) {
	if s.Resolved == nil {
		writeJSON(w, map[string]any{"resolved": []resolved.Item{}})
		return
	}
	writeJSON(w, map[string]any{"resolved": s.Resolved.List(s.resolveHostID(r))})
}

func (s *Server) resolveProblem(w http.ResponseWriter, r *http.Request) {
	s.changeResolved(w, r, true)
}

func (s *Server) unresolveProblem(w http.ResponseWriter, r *http.Request) {
	s.changeResolved(w, r, false)
}

func (s *Server) changeResolved(w http.ResponseWriter, r *http.Request, mark bool) {
	if s.Resolved == nil {
		writeErr(w, simpleError("arquivo de problemas resolvidos indisponível"))
		return
	}
	var body struct {
		ContainerID string `json:"containerId"`
		Name        string `json:"name"`
		Reason      string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ContainerID == "" || body.Reason == "" {
		http.Error(w, `{"error":"containerId e reason são obrigatórios"}`, http.StatusBadRequest)
		return
	}
	host := s.resolveHostID(r)
	if host == "" {
		host = "default"
	}
	var err error
	if mark {
		by := ""
		if u := userFromContext(r); u != nil {
			by = u.Email
		}
		err = s.Resolved.Mark(resolved.Item{
			HostID: host, ContainerID: body.ContainerID, Name: body.Name, Reason: body.Reason, By: by,
		})
	} else {
		err = s.Resolved.Unmark(host, body.ContainerID, body.Reason)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "resolved": s.Resolved.List(host)})
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	if s.Audit == nil {
		writeJSON(w, map[string]any{"events": []any{}})
		return
	}
	events, err := s.Audit.List(100)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"events": events})
}

func (s *Server) listOrgs(w http.ResponseWriter, r *http.Request) {
	if s.Orgs == nil {
		writeJSON(w, map[string]any{"orgs": []any{}})
		return
	}
	writeJSON(w, map[string]any{"orgs": s.Orgs.Orgs()})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users := []any{}
	if s.Auth != nil && s.Auth.Enabled {
		u := s.Auth.AdminUser()
		users = append(users, map[string]any{
			"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role, "orgId": u.OrgID, "source": "env",
		})
	}
	if s.Orgs != nil {
		for _, u := range s.Orgs.Users("") {
			users = append(users, map[string]any{
				"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role, "orgId": u.OrgID, "source": "directory",
			})
		}
	}
	writeJSON(w, map[string]any{"users": users})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if s.Orgs == nil {
		writeErr(w, simpleError("diretório de usuários indisponível"))
		return
	}
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role"`
		OrgID    string `json:"orgId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	orgID := body.OrgID
	if orgID == "" {
		orgID = "default"
	}
	if s.Seats != nil {
		members := 1 + len(s.Orgs.Users(orgID))
		if err := s.Seats.Ensure(orgID, members); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": seats.ErrSeatLimit.Error()})
			return
		}
	}
	u, err := s.Orgs.CreateUser(body.Email, body.Name, body.Password, body.Role, body.OrgID)
	if err != nil {
		status := http.StatusBadRequest
		if err == org.ErrExists {
			status = http.StatusConflict
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"user": u})
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	if s.Orgs == nil {
		writeErr(w, simpleError("diretório de usuários indisponível"))
		return
	}
	var body struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	u, err := s.Orgs.UpdateUser(chi.URLParam(r, "id"), body.Name, body.Role)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"user": u})
}

func (s *Server) listTeams(w http.ResponseWriter, r *http.Request) {
	if s.Orgs == nil {
		writeJSON(w, map[string]any{"teams": []any{}, "memberships": []any{}})
		return
	}
	writeJSON(w, map[string]any{
		"teams":       s.Orgs.Teams(""),
		"memberships": s.Orgs.Memberships(""),
	})
}

func (s *Server) createTeam(w http.ResponseWriter, r *http.Request) {
	if s.Orgs == nil {
		writeErr(w, simpleError("diretório de usuários indisponível"))
		return
	}
	var body struct {
		Name  string `json:"name"`
		OrgID string `json:"orgId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	t, err := s.Orgs.CreateTeam(body.OrgID, body.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"team": t})
}

func (s *Server) addTeamMember(w http.ResponseWriter, r *http.Request) {
	if s.Orgs == nil {
		writeErr(w, simpleError("diretório de usuários indisponível"))
		return
	}
	var body struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
		http.Error(w, `{"error":"userId obrigatório"}`, http.StatusBadRequest)
		return
	}
	m, err := s.Orgs.AddMember(chi.URLParam(r, "id"), body.UserID, body.Role)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"membership": m})
}

func (s *Server) costInsights(w http.ResponseWriter, r *http.Request) {
	rates := cost.RatesFromEnv()
	var rows []cost.Reservation
	noteExtra := ""
	dc, err := s.dockerClient(r)
	if err != nil {
		noteExtra = " Sem conexão Docker: a lista está vazia."
	} else {
		list, err := dc.CLI.ContainerList(r.Context(), container.ListOptions{All: true})
		if err != nil {
			noteExtra = " Falha ao listar containers."
		}
		for _, c := range list {
			info, err := dc.CLI.ContainerInspect(r.Context(), c.ID)
			if err != nil || info.HostConfig == nil {
				continue
			}
			name := c.ID[:12]
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			rows = append(rows, cost.Reservation{
				Name: name, NanoCPUs: info.HostConfig.NanoCPUs, MemoryBytes: info.HostConfig.Memory,
			})
		}
	}
	sum := cost.FromReservations(rows, rates)
	if noteExtra != "" {
		sum.Note += noteExtra
	}
	writeJSON(w, sum)
}

func (s *Server) listStackRevisions(w http.ResponseWriter, r *http.Request) {
	list, err := deploy.ListRevisions(s.dataDir(), r.URL.Query().Get("project"))
	if err != nil {
		writeErr(w, err)
		return
	}
	// Hide bulky compose bodies in the list.
	for i := range list {
		if len(list[i].Compose) > 240 {
			list[i].Compose = list[i].Compose[:240] + "…"
		}
	}
	writeJSON(w, map[string]any{"revisions": list})
}

func (s *Server) applyStack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Project     string `json:"project"`
		ProjectPath string `json:"projectPath"`
		Compose     string `json:"compose"`
		Action      string `json:"action"`
		Tag         string `json:"tag"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if body.ProjectPath == "" {
		body.ProjectPath = deploy.DefaultComposePath()
	}
	if body.Project == "" && body.ProjectPath != "" {
		body.Project = filepath.Base(body.ProjectPath)
	}
	if strings.TrimSpace(body.Compose) == "" {
		http.Error(w, `{"error":"compose obrigatório"}`, http.StatusBadRequest)
		return
	}
	actor := ""
	if u := userFromContext(r); u != nil {
		actor = u.Email
	}
	rev, err := deploy.SaveRevision(s.dataDir(), deploy.Revision{
		Project: body.Project, Path: body.ProjectPath, Compose: body.Compose, Actor: actor, Action: "update", Tag: body.Tag,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	written := ""
	if body.ProjectPath != "" {
		written, err = deploy.WriteComposeFile(body.ProjectPath, body.Compose)
		if err != nil {
			writeErr(w, err)
			return
		}
	}
	s.noteShip(r, body.ProjectPath, body.Project, rev.Tag, "up", actor)
	resp := map[string]any{"revision": rev, "composeFile": written, "applied": false}
	if body.Action == "up" {
		res, err := deploy.RunCompose(r.Context(), deploy.ComposeRequest{
			ProjectPath: body.ProjectPath, Action: "up", Build: true, Detach: true,
		})
		resp["result"] = res
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			resp["error"] = err.Error()
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		resp["applied"] = true
	}
	writeJSON(w, resp)
}

func (s *Server) rollbackStack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RevisionID  string `json:"revisionId"`
		ProjectPath string `json:"projectPath"`
		Apply       bool   `json:"apply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RevisionID == "" {
		http.Error(w, `{"error":"revisionId obrigatório"}`, http.StatusBadRequest)
		return
	}
	rev, err := deploy.LoadRevision(s.dataDir(), body.RevisionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	path := body.ProjectPath
	if path == "" {
		path = rev.Path
	}
	if path == "" {
		path = deploy.DefaultComposePath()
	}
	written, err := deploy.WriteComposeFile(path, rev.Compose)
	if err != nil {
		writeErr(w, err)
		return
	}
	actor := ""
	if u := userFromContext(r); u != nil {
		actor = u.Email
	}
	s.noteShip(r, path, rev.Project, rev.Tag, "rollback", actor)
	_, _ = deploy.SaveRevision(s.dataDir(), deploy.Revision{
		Project: rev.Project, Path: path, Compose: rev.Compose, Actor: actor, Action: "rollback", Tag: rev.Tag,
	})
	resp := map[string]any{"revision": rev, "composeFile": written, "applied": false}
	if body.Apply {
		res, err := deploy.RunCompose(r.Context(), deploy.ComposeRequest{ProjectPath: path, Action: "up", Detach: true})
		resp["result"] = res
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			resp["error"] = err.Error()
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		resp["applied"] = true
	}
	writeJSON(w, resp)
}

func (s *Server) noteShip(r *http.Request, projectPath, project, tag, action, actor string) {
	host := s.resolveHostID(r)
	if host == "" {
		host = "default"
	}
	prev, _ := triage.LoadShipFromHost(s.dataDir(), host)
	prevTag := prev.Tag
	if prevTag == tag {
		prevTag = prev.PreviousTag
	}
	if project == "" && projectPath != "" {
		project = filepath.Base(projectPath)
	}
	_ = triage.SaveShip(s.dataDir(), triage.ShipRecord{
		At: time.Now().UTC(), Tag: tag, PreviousTag: prevTag, Project: project,
		HostID: host, Action: action, Actor: actor,
	})
}

func (s *Server) recordShipFromCompose(r *http.Request, req deploy.ComposeRequest) {
	action := req.Action
	if action == "" {
		action = "up"
	}
	if action != "up" && action != "rollback" {
		return
	}
	path := req.ProjectPath
	if path == "" {
		path = deploy.DefaultComposePath()
	}
	compose := ""
	if path != "" {
		if name, err := findCompose(path); err == nil {
			if raw, err := os.ReadFile(name); err == nil {
				compose = string(raw)
			}
		}
	}
	_, tag := deploy.FirstImageTag(compose)
	actor := ""
	if u := userFromContext(r); u != nil {
		actor = u.Email
	}
	project := filepath.Base(path)
	if compose != "" {
		_, _ = deploy.SaveRevision(s.dataDir(), deploy.Revision{
			Project: project, Path: path, Compose: compose, Actor: actor, Action: action, Tag: tag,
		})
	}
	s.noteShip(r, path, project, tag, action, actor)
}

func findCompose(dir string) (string, error) {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

type proxyBody struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

func (s *Server) dockerProxy(w http.ResponseWriter, r *http.Request) {
	var body proxyBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := proxy.ValidatePath(body.Path); err != nil {
		http.Error(w, `{"error":"caminho rejeitado"}`, http.StatusBadRequest)
		return
	}
	if err := proxy.ValidateMethod(body.Method, envx.ReadOnly()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	dc, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	status, raw, err := proxy.DoDocker(r.Context(), dc.CLI, body.Method, body.Path, body.Body, envx.ReadOnly())
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "body": json.RawMessage(safeJSON(raw))})
}

func (s *Server) kubernetesProxy(w http.ResponseWriter, r *http.Request) {
	var body proxyBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	status, raw, err := proxy.DoKubernetes(r.Context(), body.Method, body.Path, body.Body, envx.ReadOnly())
	if err != nil {
		code := http.StatusBadGateway
		if err == proxy.ErrK8sUnconfigured {
			code = http.StatusNotImplemented
		}
		if err == proxy.ErrReadOnly || err == proxy.ErrBadPath {
			code = http.StatusForbidden
			if err == proxy.ErrBadPath {
				code = http.StatusBadRequest
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "body": json.RawMessage(safeJSON(raw))})
}

func safeJSON(raw []byte) []byte {
	raw = bytesTrim(raw)
	if len(raw) == 0 {
		return []byte("null")
	}
	if json.Valid(raw) {
		return raw
	}
	b, _ := json.Marshal(string(raw))
	return b
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

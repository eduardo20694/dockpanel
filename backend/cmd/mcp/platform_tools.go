package main

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/audit"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/gitops"
	"dockerpilot/internal/kube"
	"dockerpilot/internal/report"
	"dockerpilot/internal/secrets"
	"dockerpilot/internal/templates"
	"dockerpilot/internal/updates"

	"github.com/docker/docker/api/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Services struct {
	Git     *gitops.Store
	Secrets *secrets.Store
	Updates *updates.Service
	Keys    *apikey.Store
	Kube    *kube.Set
}

func registerPlatformTools(s *server.MCPServer, pool *dockerclient.Pool, aud *audit.Log, svc Services) {
	text := func(v any) *mcp.CallToolResult {
		raw, _ := json.MarshalIndent(v, "", "  ")
		return mcp.NewToolResultText(string(raw))
	}
	mustWrite := func() *mcp.CallToolResult {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error())
		}
		return nil
	}

	s.AddTool(mcp.NewTool("gitops_status", mcp.WithDescription("Mostra GitOps global, stacks vinculadas e o histórico de sync. Não aplica nada.")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if svc.Git == nil {
			return mcp.NewToolResultError("gitops indisponível"), nil
		}
		return text(svc.Git.Status()), nil
	})
	s.AddTool(mcp.NewTool("gitops_configure",
		mcp.WithDescription("Liga ou desliga GitOps e vincula uma stack a um repositório. Credencial e segredo de webhook não são devolvidos."),
		mcp.WithString("stack", mcp.Description("nome da stack")),
		mcp.WithString("repo_url"), mcp.WithString("branch"), mcp.WithString("compose_path"),
		mcp.WithString("mode", mcp.Description("auto ou approval")),
		mcp.WithString("credential", mcp.Description("deploy key ou token; vazio mantém o atual")),
		mcp.WithString("enabled", mcp.Description("true ou false, global se stack vazia")),
		mcp.WithString("poll_seconds"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		if svc.Git == nil {
			return mcp.NewToolResultError("gitops indisponível"), nil
		}
		if stack := strings.TrimSpace(req.GetString("stack", "")); stack != "" {
			var en *bool
			if v := req.GetString("enabled", ""); v != "" {
				b := v == "true" || v == "1"
				en = &b
			}
			var cred *string
			if v := req.GetString("credential", ""); v != "" {
				cred = &v
			}
			if err := svc.Git.Upsert(gitops.LinkInput{
				Stack: stack, Enabled: en, RepoURL: req.GetString("repo_url", ""), Branch: req.GetString("branch", ""),
				ComposePath: req.GetString("compose_path", ""), Mode: req.GetString("mode", ""), Credential: cred,
			}); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		} else {
			in := gitops.GlobalInput{}
			if v := req.GetString("enabled", ""); v != "" {
				b := v == "true" || v == "1"
				in.Enabled = &b
			}
			if v := strings.TrimSpace(req.GetString("poll_seconds", "")); v != "" {
				n := atoi(v)
				in.PollSeconds = &n
			}
			if err := svc.Git.SetGlobal(in); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		return text(svc.Git.Status()), nil
	})
	s.AddTool(mcp.NewTool("gitops_sync", mcp.WithDescription("Busca o compose e aplica ou deixa pendente, conforme o modo da stack."), mcp.WithString("stack", mcp.Required())), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		if svc.Git == nil {
			return mcp.NewToolResultError("gitops indisponível"), nil
		}
		ev, err := svc.Git.Sync(ctx, req.GetString("stack", ""), "manual", "mcp", aud)
		if err != nil && ev.Status == "" {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(ev), nil
	})
	s.AddTool(mcp.NewTool("gitops_approve", mcp.WithDescription("Aplica o diff pendente de uma stack em modo aprovação."), mcp.WithString("stack", mcp.Required())), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		if svc.Git == nil {
			return mcp.NewToolResultError("gitops indisponível"), nil
		}
		ev, err := svc.Git.Sync(ctx, req.GetString("stack", ""), "approve", "mcp", aud)
		if err != nil && ev.Status == "" {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(ev), nil
	})
	s.AddTool(mcp.NewTool("secrets_list", mcp.WithDescription("Lista metadados do cofre. O valor nunca é devolvido."), mcp.WithString("stack")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if svc.Secrets == nil {
			return mcp.NewToolResultError("cofre indisponível"), nil
		}
		if aud != nil {
			_ = aud.Append(audit.Event{At: time.Now().UTC(), Action: "secrets read", Detail: "mcp stack:" + req.GetString("stack", "")})
		}
		return text(svc.Secrets.List("", req.GetString("stack", ""))), nil
	})
	s.AddTool(mcp.NewTool("secrets_put", mcp.WithDescription("Grava ou substitui um segredo. A resposta não repete o valor."),
		mcp.WithString("stack", mcp.Required()), mcp.WithString("name", mcp.Required()), mcp.WithString("value", mcp.Required()),
		mcp.WithString("kind", mcp.Description("env ou file")), mcp.WithString("target")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		if svc.Secrets == nil {
			return mcp.NewToolResultError("cofre indisponível"), nil
		}
		meta, err := svc.Secrets.Put("default", req.GetString("stack", ""), req.GetString("name", ""), req.GetString("kind", ""), req.GetString("target", ""), req.GetString("value", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(meta), nil
	})
	s.AddTool(mcp.NewTool("secrets_scan", mcp.WithDescription("Aponta senha em texto num compose e sugere migrar para o cofre."), mcp.WithString("compose", mcp.Required())), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return text(secrets.ScanCompose(req.GetString("compose", ""))), nil
	})
	s.AddTool(mcp.NewTool("image_updates", mcp.WithDescription("Lista containers cuja tag ou digest no registry é mais novo. Não atualiza.")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if svc.Updates == nil {
			return mcp.NewToolResultError("atualizações indisponíveis"), nil
		}
		items, hist, pol := svc.Updates.Snapshot()
		return text(map[string]any{"policy": pol, "items": items, "history": hist}), nil
	})
	s.AddTool(mcp.NewTool("apply_image_update", mcp.WithDescription("Atualiza um container e faz rollback se o healthcheck falhar."), mcp.WithString("container", mcp.Required())), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		if svc.Updates == nil {
			return mcp.NewToolResultError("atualizações indisponíveis"), nil
		}
		res, err := svc.Updates.Apply(ctx, req.GetString("container", ""), "mcp", aud)
		if err != nil && res.Status == "" {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(res), nil
	})
	s.AddTool(mcp.NewTool("create_guest_access", mcp.WithDescription("Cria chave de consultor com validade em horas e escopo de stack ou host. O token aparece uma vez."),
		mcp.WithString("name", mcp.Required()), mcp.WithString("role", mcp.Required()), mcp.WithString("hours", mcp.Required()),
		mcp.WithString("stacks"), mcp.WithString("hosts")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		if svc.Keys == nil {
			return mcp.NewToolResultError("chaves indisponíveis"), nil
		}
		token, key, err := svc.Keys.IssueGuest("default", req.GetString("name", ""), req.GetString("role", ""), atoi(req.GetString("hours", "")), splitCSV(req.GetString("stacks", "")), splitCSV(req.GetString("hosts", "")))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if aud != nil {
			_ = aud.Append(audit.Event{At: time.Now().UTC(), Action: "guest invite " + key.Name, Detail: "key:" + key.ID})
		}
		return text(map[string]any{"token": token, "session": key}), nil
	})
	s.AddTool(mcp.NewTool("revoke_guest_access", mcp.WithDescription("Revoga na hora uma chave de consultor."), mcp.WithString("id", mcp.Required())), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		if svc.Keys == nil {
			return mcp.NewToolResultError("chaves indisponíveis"), nil
		}
		id := req.GetString("id", "")
		if err := svc.Keys.Revoke("default", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(map[string]bool{"ok": true}), nil
	})
	s.AddTool(mcp.NewTool("session_timeline", mcp.WithDescription("Linha do tempo auditada de uma chave de consultor."), mcp.WithString("id", mcp.Required())), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if aud == nil {
			return text([]audit.Event{}), nil
		}
		all, err := aud.List(500)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id := req.GetString("id", "")
		var out []audit.Event
		for _, ev := range all {
			if strings.Contains(ev.Detail, "key:"+id) {
				out = append(out, ev)
			}
		}
		if out == nil {
			out = []audit.Event{}
		}
		return text(out), nil
	})
	s.AddTool(mcp.NewTool("list_templates", mcp.WithDescription("Catálogo de stacks prontas (Postgres, Redis, Nginx+SSL, n8n, MySQL, MongoDB, Traefik, Grafana+Prometheus).")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		list := templates.Catalog()
		out := make([]map[string]any, 0, len(list))
		for _, t := range list {
			out = append(out, map[string]any{"id": t.ID, "name": t.Name, "summary": t.Summary, "vars": t.Vars})
		}
		return text(out), nil
	})
	s.AddTool(mcp.NewTool("deploy_template", mcp.WithDescription("Gera o compose, manda segredos para o cofre e sobe a stack."),
		mcp.WithString("template", mcp.Required()), mcp.WithString("stack", mcp.Required()), mcp.WithString("vars_json")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		vars := map[string]string{}
		if raw := req.GetString("vars_json", ""); raw != "" {
			_ = json.Unmarshal([]byte(raw), &vars)
		}
		prep, err := templates.Prepare(templates.Request{ID: req.GetString("template", ""), Stack: req.GetString("stack", ""), Vars: vars}, "")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		for name, value := range prep.Secrets {
			if svc.Secrets != nil {
				if _, err := svc.Secrets.Put("default", req.GetString("stack", ""), name, "env", name, value); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
		}
		return text(map[string]any{"dir": prep.Dir, "secrets": len(prep.Secrets), "note": "compose gravado; suba com deploy_compose se o daemon estiver disponível"}), nil
	})
	s.AddTool(mcp.NewTool("render_report", mcp.WithDescription("Gera o relatório de atendimento em Markdown com a identidade Dockpilot."),
		mcp.WithString("title"), mcp.WithString("incident"), mcp.WithString("final_state"), mcp.WithString("session")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		md, _ := report.Build(report.Input{
			Title: req.GetString("title", ""), Incident: req.GetString("incident", ""),
			FinalState: req.GetString("final_state", ""), Session: req.GetString("session", ""), When: time.Now().UTC(),
		})
		return mcp.NewToolResultText(md), nil
	})
	s.AddTool(mcp.NewTool("container_exec", mcp.WithDescription("Executa um comando não interativo no container e grava a linha na auditoria."),
		mcp.WithString("container", mcp.Required()), mcp.WithString("command", mcp.Required()), mcp.WithString("host_id"), mcp.WithString("stack")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if blocked := mustWrite(); blocked != nil {
			return blocked, nil
		}
		dc, err := hostClient(pool, req.GetString("host_id", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		cmd := strings.TrimSpace(req.GetString("command", ""))
		if cmd == "" {
			return mcp.NewToolResultError("comando vazio"), nil
		}
		if aud != nil {
			_ = aud.Append(audit.Event{At: time.Now().UTC(), Action: "terminal cmd", Detail: req.GetString("container", "") + " " + secrets.Redact(cmd)})
		}
		exec, err := dc.CLI.ContainerExecCreate(ctx, req.GetString("container", ""), types.ExecConfig{
			Cmd: []string{"/bin/sh", "-c", cmd}, AttachStdout: true, AttachStderr: true,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		attach, err := dc.CLI.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer attach.Close()
		out, _ := io.ReadAll(io.LimitReader(attach.Reader, 64<<10))
		return mcp.NewToolResultText(string(out)), nil
	})
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

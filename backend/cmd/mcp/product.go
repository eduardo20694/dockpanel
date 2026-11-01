package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dockerpilot/internal/audit"
	"dockerpilot/internal/cost"
	"dockerpilot/internal/deploy"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/envx"
	"dockerpilot/internal/org"
	"dockerpilot/internal/proxy"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/triage"

	dockerContainer "github.com/docker/docker/api/types/container"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerProductTools(s *server.MCPServer, pool *dockerclient.Pool, orgs *org.Store, aud *audit.Log, resolvedStore *resolved.Store) {
	registerWhatBroke(s, pool)
	registerResolveProblem(s, resolvedStore)
	registerListEnvironments(s, pool)
	registerUsersAndTeams(s, orgs)
	registerStackWrite(s)
	registerProxies(s, pool)
	registerAuditTool(s, aud)
	registerCostTool(s, pool)
	registerGuidance(s)
}

func registerGuidance(s *server.MCPServer) {
	tool := mcp.NewTool("dockpilot_guidance",
		mcp.WithDescription("Leia isto quando não souber qual ferramenta Dockpilot usar. Não altera nada. Explica a ordem segura: ambientes, diagnóstico, ship/rollback, segurança, logs, custo. Diferente do Portainer MCP, esta chamada não bloqueia as outras ferramentas."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(strings.TrimSpace(`
Dockpilot fala com o Docker Engine (socket ou ssh://), sem exigir Portainer.
Ordem sugerida:
1. list_environments — escolha host_id. Não invente o id.
2. what_broke — depois de um deploy, antes de reiniciar no escuro.
3. diagnose_container / scan_problems / security_audit / scan_image (Trivy) / compose_drift.
4. search_logs — Log Center, não só o tail ao vivo.
5. Escritas (container_action, create_stack, update_stack, rollback_stack, docker_proxy POST) respeitam DOCKERPILOT_READONLY e devem ser confirmadas pelo operador.
6. kubernetes_proxy só funciona com DOCKERPILOT_K8S_API. Se não estiver configurado, diga isso em vez de improvisar kubectl.
7. list_audit mostra quem mudou o quê. create_user / create_team exigem papel de admin na API; aqui o processo MCP usa o diretório local.
`)), nil
	})
}

func registerWhatBroke(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("what_broke",
		mcp.WithDescription("Responde 'o que quebrou' neste host: último ship (last-ship.json), restarts, OOM, problemas e incidentes do Log Center, e sugere rollback, restart ou nada a fazer. Use depois de um deploy. Não reinicia nem faz rollback — só recomenda. Para aplicar, use rollback_stack ou container_action."),
		mcp.WithString("host_id", mcp.Description("id do ambiente (list_environments). Vazio usa o host padrão.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		host := req.GetString("host_id", "")
		if host == "" {
			host = pool.DefaultID()
		}
		ship, _ := triage.LoadShipFromHost(envx.DataDir(), host)
		sig := triage.Signals{Now: time.Now()}
		if eng, _, err := diagnosticsFor(pool, host); err == nil {
			if problems, err := eng.ScanProblems(ctx); err == nil {
				for _, p := range problems {
					sig.Problems = append(sig.Problems, triage.ProblemBrief{
						ContainerID: p.ContainerID, Name: p.Name, Reason: p.Reason, Severity: string(p.Severity),
					})
					if string(p.Severity) == "critical" {
						sig.CriticalCount++
					} else {
						sig.WarningCount++
					}
					if p.OOMKilled {
						sig.OOMKilled = true
					}
					if p.RestartCount > sig.RestartCount {
						sig.RestartCount = p.RestartCount
					}
				}
			} else {
				sig.DockerNote = err.Error()
			}
		} else {
			sig.DockerNote = err.Error()
		}
		rep := triage.Build(ship, sig)
		raw, _ := json.MarshalIndent(rep, "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})
}

func registerResolveProblem(s *server.MCPServer, st *resolved.Store) {
	tool := mcp.NewTool("resolve_problem",
		mcp.WithDescription("Marca um problema como resolvido em resolved_problems.json para ele sair da varredura até a razão mudar. Não conserta o container. Exige container_id e reason iguais aos de scan_problems. Bloqueado em modo somente leitura."),
		mcp.WithString("container_id", mcp.Required(), mcp.Description("id do container")),
		mcp.WithString("reason", mcp.Required(), mcp.Description("razão exata devolvida por scan_problems")),
		mcp.WithString("name", mcp.Description("nome do container, só para o registro")),
		mcp.WithString("host_id", mcp.Description("ambiente")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if st == nil {
			return mcp.NewToolResultError("store de problemas resolvidos indisponível"), nil
		}
		id, err := req.RequireString("container_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		reason, err := req.RequireString("reason")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		host := req.GetString("host_id", "default")
		if err := st.Mark(resolved.Item{HostID: host, ContainerID: id, Name: req.GetString("name", ""), Reason: reason, By: "mcp"}); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("marcado como resolvido"), nil
	})
}

func registerListEnvironments(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("list_environments",
		mcp.WithDescription("Lista os ambientes Docker do Dockpilot (equivalente aos environments do Portainer, sem API do Portainer). Cada item tem id, label e dockerHost (vazio = socket local, ssh:// = host remoto). Passe o id como host_id nas outras ferramentas. Somente leitura."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, _ := json.MarshalIndent(pool.List(), "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})
}

func registerUsersAndTeams(s *server.MCPServer, orgs *org.Store) {
	listUsers := mcp.NewTool("list_users",
		mcp.WithDescription("Lista usuários do diretório Dockpilot (organização, papel: admin, operator, auditor, viewer). Não inclui hash de senha. O admin de ambiente (DOCKERPILOT_ADMIN_EMAIL) vive fora deste arquivo. Somente leitura."),
	)
	s.AddTool(listUsers, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if orgs == nil {
			return mcp.NewToolResultError("diretório indisponível"), nil
		}
		raw, _ := json.MarshalIndent(orgs.Users(""), "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})

	createUser := mcp.NewTool("create_user",
		mcp.WithDescription("Cria um usuário na organização (RBAC). Papéis: admin, operator, auditor, viewer. Senha mínima de 8 caracteres. Não altera o admin definido por variável de ambiente. Bloqueado em modo somente leitura. Não use para o break-glass admin."),
		mcp.WithString("email", mcp.Required(), mcp.Description("email único")),
		mcp.WithString("password", mcp.Required(), mcp.Description("senha com pelo menos 8 caracteres")),
		mcp.WithString("name", mcp.Description("nome de exibição")),
		mcp.WithString("role", mcp.Required(), mcp.Description("admin, operator, auditor ou viewer")),
		mcp.WithString("org_id", mcp.Description("organização; padrão default")),
	)
	s.AddTool(createUser, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if orgs == nil {
			return mcp.NewToolResultError("diretório indisponível"), nil
		}
		email, err := req.RequireString("email")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		pass, err := req.RequireString("password")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		role, err := req.RequireString("role")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		u, err := orgs.CreateUser(email, req.GetString("name", ""), pass, role, req.GetString("org_id", "default"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		raw, _ := json.MarshalIndent(u, "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})

	listTeams := mcp.NewTool("list_teams",
		mcp.WithDescription("Lista times e membros da organização. Somente leitura. Use antes de add_team_member para não duplicar time."),
	)
	s.AddTool(listTeams, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if orgs == nil {
			return mcp.NewToolResultError("diretório indisponível"), nil
		}
		payload := map[string]any{"teams": orgs.Teams(""), "memberships": orgs.Memberships("")}
		raw, _ := json.MarshalIndent(payload, "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})

	createTeam := mcp.NewTool("create_team",
		mcp.WithDescription("Cria um time dentro da organização. Não concede acesso ao Docker por si só: o papel da conta é o que a API aplica. Bloqueado em modo somente leitura."),
		mcp.WithString("name", mcp.Required(), mcp.Description("nome do time")),
		mcp.WithString("org_id", mcp.Description("organização; padrão default")),
	)
	s.AddTool(createTeam, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if orgs == nil {
			return mcp.NewToolResultError("diretório indisponível"), nil
		}
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		t, err := orgs.CreateTeam(req.GetString("org_id", "default"), name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		raw, _ := json.MarshalIndent(t, "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})

	addMember := mcp.NewTool("add_team_member",
		mcp.WithDescription("Associa um usuário existente a um time. user_id vem de list_users (não use o email). role é o papel dentro do time. Bloqueado em modo somente leitura."),
		mcp.WithString("team_id", mcp.Required(), mcp.Description("id do time")),
		mcp.WithString("user_id", mcp.Required(), mcp.Description("id do usuário")),
		mcp.WithString("role", mcp.Description("admin, operator, auditor ou viewer")),
	)
	s.AddTool(addMember, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if orgs == nil {
			return mcp.NewToolResultError("diretório indisponível"), nil
		}
		teamID, err := req.RequireString("team_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		userID, err := req.RequireString("user_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		m, err := orgs.AddMember(teamID, userID, req.GetString("role", "operator"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		raw, _ := json.MarshalIndent(m, "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})
}

func registerStackWrite(s *server.MCPServer) {
	create := mcp.NewTool("create_stack",
		mcp.WithDescription("Cria uma stack: grava o compose no project_path, guarda uma revisão para rollback e registra o ship. Não executa docker compose a menos que apply=true. Confirme o YAML com o operador antes de apply. Bloqueado em modo somente leitura."),
		mcp.WithString("project", mcp.Required(), mcp.Description("nome do projeto")),
		mcp.WithString("compose", mcp.Required(), mcp.Description("conteúdo do docker-compose.yml")),
		mcp.WithString("project_path", mcp.Description("pasta destino; padrão DOCKERPILOT_COMPOSE_PATH")),
		mcp.WithString("apply", mcp.Description("true para rodar compose up depois de gravar")),
	)
	s.AddTool(create, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return stackWrite(ctx, req, "create")
	})
	update := mcp.NewTool("update_stack",
		mcp.WithDescription("Atualiza o compose de uma stack existente e guarda a revisão anterior implicitamente (uma nova revisão é salva antes de qualquer up). Use list de revisões via a API /api/stacks/revisions se precisar escolher um ponto. apply=true sobe a stack. Bloqueado em modo somente leitura."),
		mcp.WithString("project", mcp.Required(), mcp.Description("nome do projeto")),
		mcp.WithString("compose", mcp.Required(), mcp.Description("compose completo novo")),
		mcp.WithString("project_path", mcp.Description("pasta do projeto")),
		mcp.WithString("apply", mcp.Description("true para compose up")),
	)
	s.AddTool(update, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return stackWrite(ctx, req, "update")
	})
	rollback := mcp.NewTool("rollback_stack",
		mcp.WithDescription("Restaura uma revisão de stack já salva (id de create_stack/update_stack ou do deploy) e registra o ship como rollback. apply=true executa compose up. Não apaga volumes. Bloqueado em modo somente leitura. Prefira what_broke antes, para confirmar que o rollback é a sugestão."),
		mcp.WithString("revision_id", mcp.Required(), mcp.Description("id da revisão")),
		mcp.WithString("project_path", mcp.Description("pasta onde reescrever o compose; padrão: a pasta gravada na revisão")),
		mcp.WithString("apply", mcp.Description("true para subir depois de restaurar o arquivo")),
	)
	s.AddTool(rollback, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("revision_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		rev, err := deploy.LoadRevision(envx.DataDir(), id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		path := req.GetString("project_path", rev.Path)
		if path == "" {
			path = deploy.DefaultComposePath()
		}
		written, err := deploy.WriteComposeFile(path, rev.Compose)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		_ = triage.SaveShip(envx.DataDir(), triage.ShipRecord{
			Tag: rev.Tag, Project: rev.Project, HostID: "default", Action: "rollback", Actor: "mcp",
		})
		msg := fmt.Sprintf("compose restaurado em %s (revisão %s, tag %s)", written, rev.ID, rev.Tag)
		if req.GetString("apply", "") == "true" {
			res, err := deploy.RunCompose(ctx, deploy.ComposeRequest{ProjectPath: path, Action: "up", Detach: true})
			if err != nil {
				return mcp.NewToolResultError(msg + " mas o up falhou: " + err.Error()), nil
			}
			msg += "\n" + res.Output
		}
		return mcp.NewToolResultText(msg), nil
	})
}

func stackWrite(ctx context.Context, req mcp.CallToolRequest, action string) (*mcp.CallToolResult, error) {
	if err := ensureWritable(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	compose, err := req.RequireString("compose")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	path := req.GetString("project_path", deploy.DefaultComposePath())
	rev, err := deploy.SaveRevision(envx.DataDir(), deploy.Revision{
		Project: project, Path: path, Compose: compose, Actor: "mcp", Action: action,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	written := ""
	if path != "" {
		written, err = deploy.WriteComposeFile(path, compose)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	_, tag := deploy.FirstImageTag(compose)
	prev, _ := triage.LoadShipFromHost(envx.DataDir(), "default")
	_ = triage.SaveShip(envx.DataDir(), triage.ShipRecord{
		Tag: tag, PreviousTag: prev.Tag, Project: project, HostID: "default", Action: action, Actor: "mcp",
	})
	msg := fmt.Sprintf("revisão %s gravada (%s), arquivo %s, tag %s", rev.ID, action, written, tag)
	if req.GetString("apply", "") == "true" {
		res, err := deploy.RunCompose(ctx, deploy.ComposeRequest{ProjectPath: path, Action: "up", Build: true, Detach: true})
		if err != nil {
			return mcp.NewToolResultError(msg + " mas o up falhou: " + err.Error()), nil
		}
		msg += "\n" + res.Output
	}
	return mcp.NewToolResultText(msg), nil
}

func registerProxies(s *server.MCPServer, pool *dockerclient.Pool) {
	dockerTool := mcp.NewTool("docker_proxy",
		mcp.WithDescription("Proxy cru da Engine API no ambiente Dockpilot (não passa por Portainer). path começa com / e não pode conter .., ? ou #. Em DOCKERPILOT_READONLY só GET e HEAD. Prefira as ferramentas nomeadas (list_containers, container_logs) quando existirem — use o proxy para rotas sem ferramenta, como /info ou /containers/{id}/json. Não envie segredos no body."),
		mcp.WithString("method", mcp.Required(), mcp.Description("GET, HEAD, POST, PUT, PATCH ou DELETE")),
		mcp.WithString("path", mcp.Required(), mcp.Description("caminho da Engine API, ex: /containers/json")),
		mcp.WithString("body", mcp.Description("JSON cru, só em escritas")),
		mcp.WithString("host_id", mcp.Description("ambiente")),
	)
	s.AddTool(dockerTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		method, err := req.RequireString("method")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		path, err := req.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := proxy.ValidatePath(path); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := proxy.ValidateMethod(method, envx.ReadOnly()); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dc, err := hostClient(pool, req.GetString("host_id", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		status, raw, err := proxy.DoDocker(ctx, dc.CLI, method, path, []byte(req.GetString("body", "")), envx.ReadOnly())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		text := string(raw)
		if len(text) > 8000 {
			text = text[:8000] + "\n…(truncado)"
		}
		return mcp.NewToolResultText(fmt.Sprintf("status %d\n%s", status, text)), nil
	})

	k8s := mcp.NewTool("kubernetes_proxy",
		mcp.WithDescription("Proxy da API Kubernetes quando DOCKERPILOT_K8S_API (e opcionalmente DOCKERPILOT_K8S_TOKEN) estão definidos. Sem isso, recuse a operação e explique que o cluster não está configurado — não invente um kubeconfig. path começa com /, sem .. nem query string. Somente GET/HEAD em modo somente leitura."),
		mcp.WithString("method", mcp.Required(), mcp.Description("GET, HEAD, POST, PUT, PATCH ou DELETE")),
		mcp.WithString("path", mcp.Required(), mcp.Description("caminho, ex: /api/v1/namespaces")),
		mcp.WithString("body", mcp.Description("JSON cru")),
	)
	s.AddTool(k8s, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		method, err := req.RequireString("method")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		path, err := req.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		status, raw, err := proxy.DoKubernetes(ctx, method, path, []byte(req.GetString("body", "")), envx.ReadOnly())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		text := string(raw)
		if len(text) > 8000 {
			text = text[:8000] + "\n…(truncado)"
		}
		return mcp.NewToolResultText(fmt.Sprintf("status %d\n%s", status, text)), nil
	})
}

func registerAuditTool(s *server.MCPServer, aud *audit.Log) {
	tool := mcp.NewTool("list_audit",
		mcp.WithDescription("Lista as ações de escrita recentes do painel (quem, rota, status HTTP). Não inclui corpos de request. Use para investigar quem fez um deploy ou um restart. Somente leitura."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if aud == nil {
			return mcp.NewToolResultText("[]"), nil
		}
		events, err := aud.List(50)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		raw, _ := json.MarshalIndent(events, "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})
}

func registerCostTool(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("cost_insights",
		mcp.WithDescription("Estima USD/dia a partir de reservas de CPU e memória e das taxas DOCKERPILOT_CPU_USD_HOUR e DOCKERPILOT_MEM_GB_USD_HOUR. É uma estimativa para conversa com a organização, não uma fatura de cloud. Somente leitura. Sem reservas configuradas o valor daquele container é zero."),
		mcp.WithString("host_id", mcp.Description("ambiente")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var rows []cost.Reservation
		if dc, err := hostClient(pool, hostIDFrom(req, pool)); err == nil {
			list, err := dc.CLI.ContainerList(ctx, dockerContainer.ListOptions{All: true})
			if err == nil {
				for _, c := range list {
					info, err := dc.CLI.ContainerInspect(ctx, c.ID)
					if err != nil || info.HostConfig == nil {
						continue
					}
					name := shortID(c.ID)
					if len(c.Names) > 0 {
						name = strings.TrimPrefix(c.Names[0], "/")
					}
					rows = append(rows, cost.Reservation{Name: name, NanoCPUs: info.HostConfig.NanoCPUs, MemoryBytes: info.HostConfig.Memory})
				}
			}
		}
		sum := cost.FromReservations(rows, cost.RatesFromEnv())
		raw, _ := json.MarshalIndent(sum, "", "  ")
		return mcp.NewToolResultText(string(raw)), nil
	})
}

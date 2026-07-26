// Servidor MCP stdio (Cursor / Claude Desktop).
// Reutiliza o mesmo dockerclient + diagnostics da API REST.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"dockpanel/internal/deploy"
	"dockpanel/internal/dockerclient"

	dockerContainer "github.com/docker/docker/api/types/container"
	dockerFilters "github.com/docker/docker/api/types/filters"
	dockerImage "github.com/docker/docker/api/types/image"
	dockerVolume "github.com/docker/docker/api/types/volume"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	pool, err := dockerclient.NewPoolFromEnv()
	if err != nil {
		log.Fatalf("erro ao conectar no Docker: %v", err)
	}

	ctx := context.Background()
	for _, h := range pool.List() {
		dc, err := pool.Get(h.ID)
		if err != nil {
			log.Printf("mcp host %q: %v", h.ID, err)
			continue
		}
		host := h.DockerHost
		if host == "" {
			host = "socket local"
		}
		if err := dc.Ping(ctx); err != nil {
			log.Printf("mcp host %q (%s) offline: %v", h.ID, h.Label, err)
		} else {
			log.Printf("mcp host %q ok: %s → %s", h.ID, h.Label, host)
		}
	}

	s := server.NewMCPServer("dockpanel", "0.1.0")

	registerListContainers(s, pool)
	registerContainerAction(s, pool)
	registerContainerLogs(s, pool)
	registerDiagnoseContainer(s, pool)
	registerScanProblems(s, pool)
	registerSafePrune(s, pool)
	registerRemoveResource(s, pool)
	registerSystemOverview(s, pool)
	registerDeployCompose(s, pool)
	registerListHosts(s, pool)
	registerComposeDrift(s, pool)
	registerScanImage(s, pool)
	registerBackupVolume(s, pool)
	registerStackHealth(s, pool)
	registerInvestigateTool(s, pool)
	registerExecutiveTool(s, pool)
	registerSecurityAuditTool(s, pool)
	registerDeepDriftTool(s, pool)
	registerSearchLogs(s)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("erro no servidor MCP: %v", err)
	}
}

// ---------- list_containers ----------

func registerListContainers(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("list_containers",
		mcp.WithDescription("Lista containers Docker do host configurado (use host_id; padrão: DOCKPANEL_HOSTS)."),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := dc.CLI.ContainerList(ctx, dockerContainer.ListOptions{All: true})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		if len(list) == 0 {
			sb.WriteString("Nenhum container encontrado no host.")
		}
		for _, c := range list {
			name := c.ID[:12]
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			fmt.Fprintf(&sb, "- %s | imagem: %s | estado: %s | status: %s | id: %s\n",
				name, c.Image, c.State, c.Status, c.ID[:12])
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------- container_action (start/stop/restart) ----------

func registerContainerAction(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("container_action",
		mcp.WithDescription("Inicia, para ou reinicia um container pelo ID ou nome no host remoto/local."),
		mcp.WithString("container", mcp.Required(), mcp.Description("ID ou nome do container")),
		mcp.WithString("action", mcp.Required(), mcp.Description("uma de: start, stop, restart")),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("container")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		action, err := req.RequireString("action")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch action {
		case "start":
			err = dc.CLI.ContainerStart(ctx, id, dockerContainer.StartOptions{})
		case "stop":
			err = dc.CLI.ContainerStop(ctx, id, dockerContainer.StopOptions{})
		case "restart":
			err = dc.CLI.ContainerRestart(ctx, id, dockerContainer.StopOptions{})
		default:
			return mcp.NewToolResultError("ação inválida: use start, stop ou restart"), nil
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("ok: %s executado em %s", action, id)), nil
	})
}

// ---------- container_logs ----------

func registerContainerLogs(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("container_logs",
		mcp.WithDescription("Retorna as últimas linhas de log de um container no host configurado."),
		mcp.WithString("container", mcp.Required(), mcp.Description("ID ou nome do container")),
		mcp.WithString("tail", mcp.Description("quantas linhas do final trazer (padrão 100)")),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("container")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		tail := req.GetString("tail", "100")

		reader, err := dc.CLI.ContainerLogs(ctx, id, dockerContainer.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       tail,
			Timestamps: true,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer reader.Close()

		buf := make([]byte, 64*1024)
		n, _ := reader.Read(buf)
		text := cleanControlBytes(string(buf[:n]))
		if text == "" {
			text = "(sem saída de log)"
		}
		return mcp.NewToolResultText(text), nil
	})
}

func cleanControlBytes(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------- diagnose_container (o principal) ----------

func registerDiagnoseContainer(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("diagnose_container",
		mcp.WithDescription("Investiga um container: exit code, OOM, restarts, eventos e logs com padrão de erro."),
		mcp.WithString("container", mcp.Required(), mcp.Description("ID ou nome do container")),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		eng, _, err := diagnosticsFor(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("container")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		d, err := eng.Diagnose(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Container: %s (%s)\n", d.Name, d.ContainerID[:12])
		fmt.Fprintf(&sb, "Severidade: %s | Estado: %s | Exit code: %d | OOMKilled: %v | Restarts: %d\n\n",
			d.Severity, d.State, d.ExitCode, d.OOMKilled, d.RestartCount)

		if len(d.Findings) > 0 {
			sb.WriteString("Achados:\n")
			for _, f := range d.Findings {
				fmt.Fprintf(&sb, "- %s\n", f)
			}
		}
		if len(d.ErrorLines) > 0 {
			sb.WriteString("\nLinhas de log com padrão de erro (mais recentes):\n")
			for _, l := range d.ErrorLines {
				fmt.Fprintf(&sb, "  %s\n", l)
			}
		}
		if len(d.RecentEvents) > 0 {
			sb.WriteString("\nEventos recentes do Docker:\n")
			for _, e := range d.RecentEvents {
				fmt.Fprintf(&sb, "  %s\n", e)
			}
		}
		fmt.Fprintf(&sb, "\nSugestão: %s\n", d.Recommendation)

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------- scan_problems ----------

func registerScanProblems(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("scan_problems",
		mcp.WithDescription("Varre containers do host e devolve os com problema (crash loop, OOM, exit code, restarting)."),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		eng, _, err := diagnosticsFor(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		problems, err := eng.ScanProblems(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(problems) == 0 {
			return mcp.NewToolResultText("Nenhum problema encontrado — todos os containers estão saudáveis."), nil
		}
		var sb strings.Builder
		for _, p := range problems {
			fmt.Fprintf(&sb, "[%s] %s — %s (estado: %s, exit: %d, restarts: %d, id: %s)\n",
				strings.ToUpper(string(p.Severity)), p.Name, p.Reason, p.State, p.ExitCode, p.RestartCount, p.ContainerID[:12])
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------- safe_prune_report ----------

func registerSafePrune(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("safe_prune_report",
		mcp.WithDescription("Lista imagens dangling, volumes e containers parados que poderiam ser removidos — sem remover."),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		var total int64

		imgs, err := dc.CLI.ImageList(ctx, dockerImage.ListOptions{
			Filters: dockerFilters.NewArgs(dockerFilters.Arg("dangling", "true")),
		})
		if err == nil && len(imgs) > 0 {
			sb.WriteString("Imagens dangling (sobra de build, seguras de remover):\n")
			for _, img := range imgs {
				fmt.Fprintf(&sb, "  - %s (%.1f MB)\n", img.ID[:19], float64(img.Size)/1e6)
				total += img.Size
			}
		}

		vols, err := dc.CLI.VolumeList(ctx, dockerVolume.ListOptions{
			Filters: dockerFilters.NewArgs(dockerFilters.Arg("dangling", "true")),
		})
		if err == nil && len(vols.Volumes) > 0 {
			sb.WriteString("\nVolumes não usados por nenhum container:\n")
			for _, v := range vols.Volumes {
				fmt.Fprintf(&sb, "  - %s\n", v.Name)
			}
		}

		list, err := dc.CLI.ContainerList(ctx, dockerContainer.ListOptions{
			All:     true,
			Filters: dockerFilters.NewArgs(dockerFilters.Arg("status", "exited")),
		})
		if err == nil && len(list) > 0 {
			sb.WriteString("\nContainers parados:\n")
			for _, c := range list {
				name := c.ID[:12]
				if len(c.Names) > 0 {
					name = strings.TrimPrefix(c.Names[0], "/")
				}
				fmt.Fprintf(&sb, "  - %s (%s)\n", name, c.Status)
			}
		}

		fmt.Fprintf(&sb, "\nEspaço estimado a liberar (só imagens dangling): %.1f MB\n", float64(total)/1e6)
		if sb.Len() == 0 {
			return mcp.NewToolResultText("Nada pra limpar — host já está enxuto."), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------- remove_resource ----------

func registerRemoveResource(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("remove_resource",
		mcp.WithDescription("Remove container, imagem ou volume no host configurado. AÇÃO DESTRUTIVA."),
		mcp.WithString("kind", mcp.Required(), mcp.Description("um de: container, image, volume")),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID ou nome do recurso")),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		kind, err := req.RequireString("kind")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch kind {
		case "container":
			err = dc.CLI.ContainerRemove(ctx, id, dockerContainer.RemoveOptions{Force: true})
		case "image":
			_, err = dc.CLI.ImageRemove(ctx, id, dockerImage.RemoveOptions{Force: true})
		case "volume":
			err = dc.CLI.VolumeRemove(ctx, id, true)
		default:
			return mcp.NewToolResultError("kind inválido: use container, image ou volume"), nil
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("removido: %s %s", kind, id)), nil
	})
}

// ---------- system_overview ----------

func registerSystemOverview(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("system_overview",
		mcp.WithDescription("Visão geral do host Docker: versão, SO, CPUs, memória, containers/imagens."),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		info, err := dc.CLI.Info(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		version, _ := dc.CLI.ServerVersion(ctx)
		text := fmt.Sprintf(
			"Docker %s | SO: %s | Kernel: %s | CPUs: %d | Memória: %.1f GB | Containers: %d (rodando: %d) | Imagens: %d",
			version.Version, info.OperatingSystem, info.KernelVersion, info.NCPU,
			float64(info.MemTotal)/1e9, info.Containers, info.ContainersRunning, info.Images,
		)
		return mcp.NewToolResultText(text), nil
	})
}

// ---------- deploy_compose ----------

func registerDeployCompose(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("deploy_compose",
		mcp.WithDescription("Executa docker compose no host (local ou ssh://). Path: project_path, DOCKPANEL_COMPOSE_PATH ou DOCKPANEL_COMPOSE_PATH_REMOTE (SSH)."),
		mcp.WithString("project_path", mcp.Description("pasta com docker-compose.yml (padrão: DOCKPANEL_COMPOSE_PATH)")),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts); padrão: primeiro do DOCKPANEL_HOSTS")),
		mcp.WithString("action", mcp.Description("up, down, build, ps ou pull (padrão: up)")),
		mcp.WithString("build", mcp.Description("true/false — incluir --build no up (padrão true para up)")),
		mcp.WithString("detach", mcp.Description("true/false — incluir -d no up (padrão true para up)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		hostID := req.GetString("host_id", pool.DefaultID())
		dockerHost := dockerHostFor(pool, hostID)
		path := strings.TrimSpace(req.GetString("project_path", ""))
		path = deploy.ResolveComposePath(dockerHost, path)
		action := req.GetString("action", "up")
		build := req.GetString("build", "true") == "true"
		detach := req.GetString("detach", "true") == "true"

		res, err := deploy.RunCompose(ctx, deploy.ComposeRequest{
			ProjectPath: path,
			Action:      action,
			Build:       build,
			Detach:      detach,
			DockerHost:  dockerHost,
		})
		if err != nil {
			msg := err.Error()
			if res != nil && res.Output != "" {
				msg += "\n\n--- saída ---\n" + res.Output
			}
			return mcp.NewToolResultError(msg), nil
		}

		hostLabel := dockerHost
		if hostLabel == "" {
			hostLabel = deploy.DockerHostLabel()
		} else {
			hostLabel = "DOCKER_HOST=" + hostLabel
		}
		text := fmt.Sprintf("ok: compose %s em %s (%s) em %s\nHost Docker: %s\n\n%s",
			res.Action, res.Path, res.Compose, res.Duration, hostLabel, res.Output)
		return mcp.NewToolResultText(text), nil
	})
}

func dockerHostFor(pool *dockerclient.Pool, hostID string) string {
	if hostID == "" {
		hostID = pool.DefaultID()
	}
	for _, h := range pool.List() {
		if h.ID == hostID {
			return h.DockerHost
		}
	}
	return ""
}

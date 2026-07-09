package main

import (
	"context"
	"fmt"
	"strings"

	"dockpanel/internal/backup"
	"dockpanel/internal/deploy"
	"dockpanel/internal/diagnostics"
	"dockpanel/internal/drift"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/insights"
	"dockpanel/internal/scan"
	"dockpanel/internal/stacks"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerListHosts(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("list_hosts",
		mcp.WithDescription("Lista hosts Docker configurados (local, VPS, etc.) para usar com host_id nas outras tools."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var sb strings.Builder
		for _, h := range pool.List() {
			sb.WriteString(fmt.Sprintf("- %s: %s (DOCKER_HOST=%s)\n", h.ID, h.Label, h.DockerHost))
		}
		if sb.Len() == 0 {
			return mcp.NewToolResultText("nenhum host configurado"), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerComposeDrift(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("check_compose_drift",
		mcp.WithDescription("Compara docker-compose.yml local com containers no host remoto — detecta drift."),
		mcp.WithString("project_path", mcp.Description("pasta local do compose (padrão: DOCKPANEL_COMPOSE_PATH)")),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := strings.TrimSpace(req.GetString("project_path", ""))
		if path == "" {
			path = deploy.ResolveComposePath("", "")
		}
		dc, err := pool.Get(req.GetString("host_id", pool.DefaultID()))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		report, err := drift.Check(ctx, dc.CLI, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Project: %s · drift: %d\n\n", report.ProjectName, report.DriftCount)
		for _, it := range report.Items {
			flag := "ok"
			if it.Drift {
				flag = "DRIFT"
			}
			fmt.Fprintf(&sb, "[%s] %s — %s\n  compose: %s\n  running: %s\n  %s\n\n",
				flag, it.Service, it.ContainerName, it.ComposeImage, it.RunningImage, it.Detail)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerScanImage(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("scan_image_vulnerabilities",
		mcp.WithDescription("Escaneia CVEs com Trivy local (imagem precisa existir no daemon do host_id ou local)."),
		mcp.WithString("image", mcp.Required(), mcp.Description("ref da imagem ex: redecoop/dockpanel:0.0.1")),
		mcp.WithString("host_id", mcp.Description("id do host — Trivy roda localmente; imagem deve estar pullada")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		image, err := req.RequireString("image")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		_ = hostIDFrom(req, pool)
		report, err := scan.ScanImage(ctx, image)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if report.RawNote != "" && report.VulnCount == 0 {
			return mcp.NewToolResultText(report.RawNote), nil
		}
		text := fmt.Sprintf("Imagem: %s\nTotal: %d (critical: %d, high: %d)\n",
			report.Image, report.VulnCount, report.CriticalCount, report.HighCount)
		for _, v := range report.Vulnerabilities {
			text += fmt.Sprintf("- [%s] %s %s — %s\n", v.Severity, v.ID, v.PkgName, v.Title)
		}
		return mcp.NewToolResultText(text), nil
	})
}

func registerBackupVolume(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("backup_volume",
		mcp.WithDescription("Cria tarball de backup de um volume Docker no host configurado."),
		mcp.WithString("volume", mcp.Required(), mcp.Description("nome do volume")),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts)")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("volume")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hostID := hostIDFrom(req, pool)
		dockerHost := dockerHostFor(pool, hostID)
		res, err := backup.BackupVolume(ctx, name, "", dockerHost)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("backup ok: %s (%.1f MB) em %s",
			res.VolumeName, float64(res.SizeBytes)/1e6, res.BackupPath)), nil
	})
}

func registerStackHealth(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("stack_health",
		mcp.WithDescription("Saúde agregada por projeto docker-compose: serviços, severidade, cascata de falhas."),
		mcp.WithString("host_id", mcp.Description("id do host (list_hosts); padrão: default")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := pool.Get(req.GetString("host_id", pool.DefaultID()))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hostID := req.GetString("host_id", pool.DefaultID())
		label := hostID
		for _, h := range pool.List() {
			if h.ID == hostID {
				label = h.Label
				break
			}
		}
		list, err := stacks.ListStacks(ctx, dc.CLI, hostID, label)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		for _, st := range list {
			fmt.Fprintf(&sb, "[%s] %s — %d/%d rodando · critical:%d warning:%d\n",
				strings.ToUpper(string(st.Severity)), st.Project, st.Running, st.Total, st.Critical, st.Warning)
			for _, svc := range st.Services {
				if svc.Severity != diagnostics.SeverityOK || svc.AffectedBy != "" {
					fmt.Fprintf(&sb, "  · %s (%s) %s", svc.Name, svc.State, svc.Reason)
					if svc.AffectedBy != "" {
						fmt.Fprintf(&sb, " [cascata: %s]", svc.AffectedBy)
					}
					sb.WriteString("\n")
				}
			}
			for _, n := range st.CascadeNotes {
				fmt.Fprintf(&sb, "  ⚡ %s\n", n)
			}
			sb.WriteString("\n")
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerInvestigateTool(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("investigate_container",
		mcp.WithDescription("Investigação profunda: diagnose + inspect resumido + riscos de segurança do container."),
		mcp.WithString("container", mcp.Required(), mcp.Description("ID ou nome")),
		mcp.WithString("host_id", mcp.Description("id do host")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := pool.Get(req.GetString("host_id", pool.DefaultID()))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, err := req.RequireString("container")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		eng := diagnostics.New(dc.CLI)
		d, err := eng.Diagnose(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		info, _ := dc.CLI.ContainerInspect(ctx, d.ContainerID)
		sec, _ := insights.Audit(ctx, dc.CLI, pool.DefaultID(), "")
		var sb strings.Builder
		fmt.Fprintf(&sb, "=== %s ===\nSeveridade: %s | %s | restarts: %d\n\n", d.Name, d.Severity, d.State, d.RestartCount)
		for _, f := range d.Findings {
			fmt.Fprintf(&sb, "• %s\n", f)
		}
		fmt.Fprintf(&sb, "\nInspect: image=%s project=%s privileged=%v user=%q\n",
			info.Config.Image, info.Config.Labels["com.docker.compose.project"],
			info.HostConfig.Privileged, info.Config.User)
		if sec != nil {
			for _, f := range sec.Findings {
				if f.ContainerID == d.ContainerID {
					fmt.Fprintf(&sb, "⚠ [%s] %s: %s\n", f.Severity, f.Category, f.Detail)
				}
			}
		}
		fmt.Fprintf(&sb, "\nSugestão: %s\n", d.Recommendation)
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerExecutiveTool(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("executive_summary",
		mcp.WithDescription("Resumo executivo multi-host: problemas, stacks critical, segurança, pressão de disco."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var sb strings.Builder
		var totalCrit, totalWarn int
		for _, h := range pool.List() {
			dc, err := pool.Get(h.ID)
			if err != nil {
				continue
			}
			if err := dc.Ping(ctx); err != nil {
				fmt.Fprintf(&sb, "✗ %s (%s) offline\n", h.Label, h.ID)
				continue
			}
			eng := diagnostics.New(dc.CLI)
			problems, _ := eng.ScanProblems(ctx)
			crit, warn := 0, 0
			for _, p := range problems {
				if p.Severity == diagnostics.SeverityCritical {
					crit++
				} else {
					warn++
				}
			}
			totalCrit += crit
			totalWarn += warn
			sec, _ := insights.Audit(ctx, dc.CLI, h.ID, h.Label)
			secCrit := 0
			if sec != nil {
				secCrit = sec.CriticalCount
			}
			fmt.Fprintf(&sb, "• %s — critical:%d warning:%d segurança:%d trivy:%v\n",
				h.Label, crit, warn, secCrit, scan.TrivyAvailable())
		}
		fmt.Fprintf(&sb, "\nTotal: %d critical, %d warning em %d hosts\n", totalCrit, totalWarn, len(pool.List()))
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerSecurityAuditTool(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("security_audit",
		mcp.WithDescription("Auditoria de risco: root, privileged, portas expostas, :latest, healthcheck."),
		mcp.WithString("host_id", mcp.Description("id do host")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := pool.Get(req.GetString("host_id", pool.DefaultID()))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hostID := req.GetString("host_id", pool.DefaultID())
		label := hostID
		for _, h := range pool.List() {
			if h.ID == hostID {
				label = h.Label
				break
			}
		}
		rep, err := insights.Audit(ctx, dc.CLI, hostID, label)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Host %s — %d critical, %d warning, %d com :latest\n\n", label, rep.CriticalCount, rep.WarningCount, rep.LatestTagCount)
		for _, f := range rep.Findings {
			if f.Severity == "info" {
				continue
			}
			fmt.Fprintf(&sb, "[%s] %s — %s: %s\n", f.Severity, f.Name, f.Category, f.Detail)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerDeepDriftTool(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("deep_compose_drift",
		mcp.WithDescription("Drift profundo: compose local vs containers no host remoto."),
		mcp.WithString("project_path", mcp.Description("pasta local do compose (padrão: DOCKPANEL_COMPOSE_PATH)")),
		mcp.WithString("host_id", mcp.Description("id do host")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := pool.Get(req.GetString("host_id", pool.DefaultID()))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		path := strings.TrimSpace(req.GetString("project_path", ""))
		if path == "" {
			path = deploy.ResolveComposePath("", "")
		}
		report, err := drift.DeepCheck(ctx, dc.CLI, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Project %s — %d drift(s)\n\n", report.ProjectName, report.TotalDrift)
		for _, it := range report.DeepItems {
			if it.Drift {
				fmt.Fprintf(&sb, "[DRIFT:%s] %s — %s\n", it.Kind, it.Service, it.Detail)
			}
		}
		for _, o := range report.Orphans {
			fmt.Fprintf(&sb, "[ORPHAN] %s — %s\n", o.ContainerName, o.Detail)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

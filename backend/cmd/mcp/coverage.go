package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/logcenter"
	"dockerpilot/internal/store"

	dockerTypes "github.com/docker/docker/api/types"
	dockerImage "github.com/docker/docker/api/types/image"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerCoverageTools fecha a lacuna entre o painel e o MCP: imagens, volumes,
// redes, métricas, alertas, incidentes e retenção.
func registerCoverageTools(s *server.MCPServer, pool *dockerclient.Pool) {
	registerListImages(s, pool)
	registerListVolumes(s, pool)
	registerListNetworks(s, pool)
	registerContainerStats(s, pool)
	registerListAlerts(s)
	registerLogIncidents(s)
	registerLogRetentionTool(s)
}

func registerListImages(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("list_images",
		mcp.WithDescription("Lista imagens locais do ambiente, como a tela Imagens. Somente leitura. Use scan_image_vulnerabilities para CVEs."),
		mcp.WithString("host_id", mcp.Description("ambiente")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := dc.CLI.ImageList(ctx, dockerImage.ListOptions{All: true})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(list) == 0 {
			return mcp.NewToolResultText("Nenhuma imagem."), nil
		}
		var sb strings.Builder
		for _, img := range list {
			tag := "<none>"
			if len(img.RepoTags) > 0 && img.RepoTags[0] != "" {
				tag = img.RepoTags[0]
			}
			fmt.Fprintf(&sb, "- %s | %s | %.1f MB\n", tag, shortID(img.ID), float64(img.Size)/1e6)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerListVolumes(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("list_volumes",
		mcp.WithDescription("Lista volumes Docker do ambiente, como a tela Volumes. Somente leitura. Backup continua em backup_volume."),
		mcp.WithString("host_id", mcp.Description("ambiente")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		vols, err := dc.CLI.VolumeList(ctx, dockerVolumeList())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if vols.Volumes == nil || len(vols.Volumes) == 0 {
			return mcp.NewToolResultText("Nenhum volume."), nil
		}
		var sb strings.Builder
		for _, v := range vols.Volumes {
			fmt.Fprintf(&sb, "- %s | driver %s\n", v.Name, v.Driver)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerListNetworks(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("list_networks",
		mcp.WithDescription("Lista redes Docker do ambiente, como a tela Redes. Somente leitura."),
		mcp.WithString("host_id", mcp.Description("ambiente")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := dc.CLI.NetworkList(ctx, dockerTypes.NetworkListOptions{})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		for _, n := range list {
			fmt.Fprintf(&sb, "- %s | %s | %s\n", n.Name, n.Driver, shortID(n.ID))
		}
		if sb.Len() == 0 {
			return mcp.NewToolResultText("Nenhuma rede."), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerContainerStats(s *server.MCPServer, pool *dockerclient.Pool) {
	tool := mcp.NewTool("container_stats",
		mcp.WithDescription("Uma amostra de CPU e memória de um container (não é stream). Equivale ao número ao vivo da tela Containers. Somente leitura."),
		mcp.WithString("container", mcp.Required(), mcp.Description("id ou nome")),
		mcp.WithString("host_id", mcp.Description("ambiente")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("container")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dc, err := hostClient(pool, hostIDFrom(req, pool))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		stats, err := dc.CLI.ContainerStats(ctx, id, false)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer stats.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(stats.Body, 1<<20))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var parsed dockerTypes.StatsJSON
		_ = jsonUnmarshal(raw, &parsed)
		mem := parsed.MemoryStats.Usage
		return mcp.NewToolResultText(fmt.Sprintf("container %s\nmemória: %d bytes\ncpu total: %d\n", id, mem, parsed.CPUStats.CPUUsage.TotalUsage)), nil
	})
}

func registerListAlerts(s *server.MCPServer) {
	tool := mcp.NewTool("list_alerts",
		mcp.WithDescription("Histórico de alertas gravado pelo scanner do painel (critical e warning). Somente leitura. Vazio se o coletor ainda não rodou."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := store.Open("")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		alerts, err := st.RecentAlerts(20)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(alerts) == 0 {
			return mcp.NewToolResultText("Nenhum alerta no histórico."), nil
		}
		var sb strings.Builder
		for _, a := range alerts {
			fmt.Fprintf(&sb, "- %s | %s | %s\n", a.Severity, a.ContainerName, a.Title)
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerLogIncidents(s *server.MCPServer) {
	tool := mcp.NewTool("list_log_incidents",
		mcp.WithDescription("Agrupa linhas warning/critical do Log Center em incidentes, como a aba Incidentes. Somente leitura."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := logcenter.Open("")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer st.Close()
		inc, err := st.Incidents(0, 0, 0)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(inc) == 0 {
			return mcp.NewToolResultText("Nenhum incidente na janela."), nil
		}
		var sb strings.Builder
		for _, it := range inc {
			fmt.Fprintf(&sb, "- %s | %d linhas | %s\n", it.ID, it.EntryCount, strings.Join(it.Containers, ", "))
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func registerLogRetentionTool(s *server.MCPServer) {
	tool := mcp.NewTool("log_retention",
		mcp.WithDescription("Lê ou define a retenção do Log Center em dias (1 a 60), como /api/logs/incidents/retention. Sem o argumento days, só lê. Definir dias é escrita e respeita somente leitura e o papel."),
		mcp.WithString("days", mcp.Description("se informado, grava a retenção")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := logcenter.Open("")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer st.Close()
		if days := strings.TrimSpace(req.GetString("days", "")); days != "" {
			if err := ensureWritable(); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			n := 0
			fmt.Sscanf(days, "%d", &n)
			if err := st.SetRetentionDays(n); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		return mcp.NewToolResultText(fmt.Sprintf("retenção: %d dias (máx %d)", st.GetRetentionDays(), logcenter.MaxRetentionDays)), nil
	})
}

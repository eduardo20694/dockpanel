package main

import (
	"context"

	"dockerpilot/internal/composeedit"
	"dockerpilot/internal/files"
	"dockerpilot/internal/scan"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGapTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("compose_preview",
		mcp.WithDescription("Lê um compose e devolve os serviços editáveis, sem aplicar."),
		mcp.WithString("yaml", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		svcs, err := composeedit.Parse(req.GetString("yaml", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(composeedit.Render(svcs)), nil
	})
	s.AddTool(mcp.NewTool("export_sarif",
		mcp.WithDescription("Converte um achado CVE em SARIF 2.1.0."),
		mcp.WithString("id", mcp.Required()),
		mcp.WithString("severity"),
		mcp.WithString("image"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, err := scan.ToSARIF(scan.Report{
			Image: req.GetString("image", "image"),
			Vulnerabilities: []scan.Vulnerability{{
				ID: req.GetString("id", ""), Severity: req.GetString("severity", "HIGH"), Title: req.GetString("id", ""),
			}},
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(raw)), nil
	})
	s.AddTool(mcp.NewTool("list_files",
		mcp.WithDescription("Interpreta uma listagem ls -ln de um container ou volume. Não altera arquivos."),
		mcp.WithString("path"),
		mcp.WithString("listing", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entries := files.ParseLS(req.GetString("path", "/"), req.GetString("listing", ""))
		if len(entries) == 0 {
			return mcp.NewToolResultText("pasta vazia"), nil
		}
		return mcp.NewToolResultText(entries[0].Path), nil
	})
}

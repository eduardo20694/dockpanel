package main

import (
	"context"
	"time"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/audit"
	"dockerpilot/internal/envx"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func toolGate(aud *audit.Log) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name := req.Params.Name
			if p, ok := apikey.FromContext(ctx); ok {
				if err := apikey.AllowTool(p, name); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				if err := p.AllowsCall(name, req.GetString("host_id", ""), req.GetString("stack", "")); err != nil {
					return mcp.NewToolResultError("fora do escopo desta chave"), nil
				}
				if apikey.IsMutating(name) && envx.ReadOnly() && name != "docker_proxy" && name != "kubernetes_proxy" {
					return mcp.NewToolResultError("modo somente leitura"), nil
				}
			}
			res, err := next(ctx, req)
			if p, ok := apikey.FromContext(ctx); ok && aud != nil {
				status := 200
				if err != nil || (res != nil && res.IsError) {
					status = 400
				}
				_ = aud.Append(audit.Event{
					At: time.Now().UTC(), Actor: p.UserID, OrgID: p.OrgID,
					Action: "MCP " + name, Status: status, Detail: "key:" + p.KeyID,
				})
			}
			return res, err
		}
	}
}

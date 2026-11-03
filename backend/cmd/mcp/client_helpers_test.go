package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func mustClient(t *testing.T, s *server.MCPServer) *client.Client {
	t.Helper()
	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = c.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcp.Implementation{Name: "dockpilot-test", Version: "0"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func callTool(t *testing.T, c *client.Client, name string, args map[string]any) (string, bool, error) {
	t.Helper()
	res, err := c.CallTool(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}})
	if err != nil {
		return "", true, err
	}
	var text string
	for _, block := range res.Content {
		if c, ok := block.(mcp.TextContent); ok {
			text += c.Text
		}
	}
	return text, res.IsError, nil
}

func trim(s string) string {
	if len(s) > 400 {
		return s[:400]
	}
	return s
}

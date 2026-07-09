package main

import (
	"dockpanel/internal/diagnostics"
	"dockpanel/internal/dockerclient"

	"github.com/mark3labs/mcp-go/mcp"
)

func hostIDFrom(req mcp.CallToolRequest, pool *dockerclient.Pool) string {
	return req.GetString("host_id", pool.DefaultID())
}

func hostClient(pool *dockerclient.Pool, hostID string) (*dockerclient.Client, error) {
	if hostID == "" {
		hostID = pool.DefaultID()
	}
	return pool.Get(hostID)
}

func diagnosticsFor(pool *dockerclient.Pool, hostID string) (*diagnostics.Engine, *dockerclient.Client, error) {
	dc, err := hostClient(pool, hostID)
	if err != nil {
		return nil, nil, err
	}
	return diagnostics.New(dc.CLI), dc, nil
}

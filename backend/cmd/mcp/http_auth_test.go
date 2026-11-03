package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/audit"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/org"
	"dockerpilot/internal/resolved"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPHTTPKeyAuth(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_DATA_DIR", dir)
	t.Setenv("DOCKERPILOT_HOSTS", `[{"id":"local","label":"Local","dockerHost":"unix:///var/run/docker.sock"}]`)
	pool, err := dockerclient.NewPoolFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	orgs, _ := org.Open(dir)
	aud, _ := audit.Open(dir)
	res, _ := resolved.Open(dir)
	keys, err := apikey.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	admin, _, err := keys.Issue("default", "admin", "admin", "admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	viewer, viewKey, err := keys.Issue("default", "view", "view", "viewer", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	revoked, revKey, err := keys.Issue("default", "old", "old", "admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Revoke("default", revKey.ID); err != nil {
		t.Fatal(err)
	}
	_ = viewKey

	mcpSrv := buildMCP(pool, orgs, aud, res, Services{})
	ts := httptest.NewServer(mcpHandler(mcpSrv, keys, aud))
	defer ts.Close()

	t.Run("válida", func(t *testing.T) {
		out := httpCall(t, ts.URL, admin, "dockpilot_guidance", nil)
		if !strings.Contains(out, "list_environments") {
			t.Fatalf("%s", out)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/sse", nil)
		req.Header.Set("Authorization", "Bearer "+admin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil && ctx.Err() == nil {
			t.Fatal(err)
		}
		if resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
				t.Fatalf("sse %d %s", resp.StatusCode, b)
			}
		}
	})
	t.Run("inválida", func(t *testing.T) {
		if _, err := connect(ts.URL, "dp_live_naoexiste"); err == nil {
			t.Fatal("esperava recusa")
		}
	})
	t.Run("revogada", func(t *testing.T) {
		if _, err := connect(ts.URL, revoked); err == nil {
			t.Fatal("esperava recusa")
		}
	})
	t.Run("sem permissão", func(t *testing.T) {
		out := httpCall(t, ts.URL, viewer, "container_action", map[string]any{"container": "x", "action": "stop"})
		if !strings.Contains(out, "permissão") {
			t.Fatalf("%s", out)
		}
	})
}

func connect(base, token string) (*client.Client, error) {
	c, err := client.NewStreamableHttpClient(base+"/mcp", transport.WithHTTPHeaders(map[string]string{
		"Authorization": "Bearer " + token,
	}))
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		return nil, err
	}
	_, err = c.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcp.Implementation{Name: "sdk", Version: "0"},
	}})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func httpCall(t *testing.T, base, token, name string, args map[string]any) string {
	t.Helper()
	c, err := connect(base, token)
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := callTool(t, c, name, args)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

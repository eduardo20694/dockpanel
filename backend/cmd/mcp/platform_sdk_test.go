package main

import (
	"strings"
	"testing"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/audit"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/gitops"
	"dockerpilot/internal/org"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/secrets"
)

func TestPlatformToolsSDK(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_DATA_DIR", dir)
	t.Setenv("DOCKERPILOT_SECRETS_KEY", "sdk-master")
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
	sec, err := secrets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	gitStore, err := gitops.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := mustClient(t, buildMCP(pool, orgs, aud, res, Services{Git: gitStore, Secrets: sec, Keys: keys}))
	out, _, err := callTool(t, c, "list_templates", nil)
	if err != nil || !strings.Contains(out, "postgres") || !strings.Contains(out, "grafana-prometheus") {
		t.Fatalf("%v %s", err, out)
	}
	out, _, err = callTool(t, c, "secrets_scan", map[string]any{"compose": "PASSWORD: plain\nTOKEN: ${TOKEN}\n"})
	if err != nil || !strings.Contains(out, "cofre") || strings.Contains(out, "${TOKEN}") && strings.Count(out, "linha") != 1 && !strings.Contains(out, "PASSWORD") {
		t.Fatalf("%v %s", err, out)
	}
	out, _, err = callTool(t, c, "secrets_put", map[string]any{"stack": "api", "name": "db", "value": "hidden-value", "target": "DB_PASSWORD"})
	if err != nil || strings.Contains(out, "hidden-value") {
		t.Fatalf("%v %s", err, out)
	}
	out, _, err = callTool(t, c, "render_report", map[string]any{"title": "Incidente", "incident": "crash loop", "final_state": "saudável"})
	if err != nil || !strings.Contains(out, "Dockpilot") || !strings.Contains(out, "crash loop") {
		t.Fatalf("%v %s", err, out)
	}
	out, _, err = callTool(t, c, "create_guest_access", map[string]any{"name": "Lia", "role": "operator", "hours": "3", "stacks": "api", "hosts": "prod"})
	if err != nil || !strings.Contains(out, "dp_live_") {
		t.Fatalf("%v %s", err, out)
	}
}

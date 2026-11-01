package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"dockerpilot/internal/audit"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/org"
	"dockerpilot/internal/resolved"
)

func TestLiveDockerTools(t *testing.T) {
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skip("sem docker")
	}
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_DATA_DIR", dir)
	t.Setenv("DOCKERPILOT_HOSTS", "")
	t.Setenv("DOCKERPILOT_READONLY", "")
	t.Setenv("DOCKERPILOT_COMPOSE_PATH", "/tmp/dp-compose")
	pool, err := dockerclient.NewPoolFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cli, err := pool.Get(""); err == nil {
		if err := cli.Ping(context.Background()); err != nil {
			t.Skip(err)
		}
	}
	orgs, err := org.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	aud, err := audit.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := resolved.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := mustClient(t, buildMCP(pool, orgs, aud, res))
	calls := []struct {
		name string
		args map[string]any
		want string
	}{
		{"list_containers", nil, "dp-sleep"},
		{"list_images", nil, "busybox"},
		{"list_volumes", nil, "dp-data"},
		{"list_networks", nil, "bridge"},
		{"container_logs", map[string]any{"container": "dp-sleep", "tail": "20"}, ""},
		{"container_stats", map[string]any{"container": "dp-sleep"}, "memória"},
		{"diagnose_container", map[string]any{"container": "dp-exit"}, ""},
		{"scan_problems", nil, "dp-exit"},
		{"investigate_container", map[string]any{"container": "dp-sleep"}, ""},
		{"security_audit", nil, ""},
		{"safe_prune_report", nil, ""},
		{"system_overview", nil, ""},
		{"executive_summary", nil, ""},
		{"stack_health", nil, ""},
		{"what_broke", nil, "dp-exit"},
		{"cost_insights", nil, "dp-sleep"},
		{"list_hosts", nil, ""},
		{"list_environments", nil, ""},
		{"list_alerts", nil, ""},
		{"search_logs", nil, ""},
		{"list_log_incidents", nil, ""},
		{"log_retention", nil, "retenção"},
		{"dockpilot_guidance", nil, "list_environments"},
		{"list_users", nil, ""},
		{"list_teams", nil, ""},
		{"list_audit", nil, ""},
		{"check_compose_drift", map[string]any{"project_path": "/tmp/dp-compose"}, ""},
		{"deep_compose_drift", map[string]any{"project_path": "/tmp/dp-compose"}, ""},
		{"deploy_compose", map[string]any{"project_path": "/tmp/dp-compose", "action": "ps"}, ""},
		{"scan_image_vulnerabilities", map[string]any{"image": "busybox:1.36"}, ""},
		{"backup_volume", map[string]any{"volume": "dp-data"}, "backup"},
		{"container_action", map[string]any{"container": "dp-sleep", "action": "restart"}, "ok"},
		{"resolve_problem", map[string]any{"container_id": "ad86069fc61e", "reason": "parou com erro", "name": "dp-exit"}, "marcado"},
		{"create_user", map[string]any{"email": "ops@dockpilot.dev", "password": "senha-forte", "role": "operator", "name": "Ops"}, "ops@"},
		{"docker_proxy", map[string]any{"method": "GET", "path": "/info"}, "status"},
		{"kubernetes_proxy", map[string]any{"method": "GET", "path": "/api/v1/pods"}, "não configurado"},
		{"create_stack", map[string]any{"project": "demo", "project_path": dir + "/stack", "compose": "services:\n  nap:\n    image: busybox:1.36\n    command: [\"sleep\", \"30\"]\n"}, "revisão"},
	}
	var failed int
	for _, call := range calls {
		out, isErr, err := callTool(t, c, call.name, call.args)
		ok := err == nil
		if isErr && call.name != "kubernetes_proxy" {
			ok = false
		}
		if call.want != "" && !strings.Contains(out, call.want) {
			ok = false
		}
		status := "ok"
		if !ok {
			status = "FAIL"
			failed++
			t.Errorf("%s: err=%v isError=%v out=%s", call.name, err, isErr, trim(out))
		}
		t.Logf("%s %s", status, call.name)
	}
	if failed > 0 {
		t.Fatalf("%d tools falharam", failed)
	}
}

package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/kube"
)

func TestHostsShowDetectedStatusAndBlockSwarm(t *testing.T) {
	pool, err := dockerclient.NewPool([]dockerclient.HostConfig{
		{ID: "local", Label: "Local", DockerHost: "unix:///tmp/dockpilot-ausente.sock"},
		{ID: "pod", Label: "Podman", Kind: "auto", DockerHost: "unix:///run/podman/podman.sock"},
		{ID: "manual", Label: "Forçado", Kind: "docker", DockerHost: "unix:///tmp/dockpilot-ausente.sock"},
	}, "local")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(`apiVersion: v1
kind: Config
clusters:
- name: c1
  cluster: {server: https://127.0.0.1:1}
contexts:
- name: kind-dockpilot
  context: {cluster: c1, user: u}
users:
- name: u
  user: {token: t}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := testServer(t)
	s.Hosts = pool
	s.Kube = kube.Open(SourceWait(path))
	tok := login(t, s, "admin@dockpilot.dev", "secret-pass")

	rr := do(s, http.MethodGet, "/api/hosts", "", tok)
	body := rr.Body.String()
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, body)
	}
	for _, want := range []string{
		`"id":"local"`,
		"Falha de conexão",
		`"kind":"podman"`,
		`"detectedKind":"podman"`,
		`"manual":true`,
		"Cluster antigo",
		"kind-dockpilot",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("falta %q em %s", want, body)
		}
	}

	nodes := do(s, http.MethodGet, "/api/swarm/nodes?host=local", "", tok)
	if nodes.Code == http.StatusOK || !strings.Contains(nodes.Body.String(), "Falha de conexão") {
		t.Fatalf("swarm %d %s", nodes.Code, nodes.Body.String())
	}
}

func SourceWait(path string) []kube.Source {
	return []kube.Source{{ID: "old", Label: "Cluster antigo", Kubeconfig: path, Context: "kind-dockpilot"}}
}

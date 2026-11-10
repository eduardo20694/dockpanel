package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenConnectionFailureAndInCluster(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	raw := []byte(`apiVersion: v1
kind: Config
clusters:
- name: c1
  cluster: {server: https://127.0.0.1:1}
contexts:
- name: dev
  context: {cluster: c1, user: u}
current-context: dev
users:
- name: u
  user: {token: t}
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	set := Open([]Source{{ID: "dev", Label: "Dev", Kubeconfig: path, Context: "dev"}})
	c, err := set.Get("dev")
	if err != nil || c.Status != "offline" || !strings.Contains(c.Detail, "Falha de conexão") || c.Client != nil {
		t.Fatalf("%+v %v", c, err)
	}

	in := Open([]Source{{ID: "incluster", Label: "Cluster local", InCluster: true}})
	got, err := in.Get("incluster")
	if err != nil || got.Status != "offline" || !strings.Contains(got.Detail, "Falha de conexão") {
		t.Fatalf("%+v %v", got, err)
	}
}

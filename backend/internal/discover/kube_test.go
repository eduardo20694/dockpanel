package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandKubeContextsAndMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	raw := []byte(`apiVersion: v1
kind: Config
clusters:
- name: c1
  cluster: {server: https://127.0.0.1:6443}
contexts:
- name: dev
  context: {cluster: c1, user: u}
- name: prod
  context: {cluster: c1, user: u}
current-context: dev
users:
- name: u
  user: {token: t}
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	all, err := ExpandKube("lab", "Lab", path, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("%d %v", len(all), err)
	}
	one, err := ExpandKube("lab", "Lab", path, "dev")
	if err != nil || len(one) != 1 || one[0].Context != "dev" || one[0].Status != "online" {
		t.Fatalf("%+v %v", one, err)
	}
	missing, err := ExpandKube("lab", "Lab", path, "nao-existe")
	if err != nil || missing[0].Status != "offline" || missing[0].Detail == "" {
		t.Fatalf("%+v", missing)
	}
	bad, _ := ExpandKube("x", "x", filepath.Join(dir, "ausente"), "")
	if len(bad) != 1 || bad[0].Status != "offline" {
		t.Fatalf("%+v", bad)
	}
}

func TestServiceAccountDetection(t *testing.T) {
	dir := t.TempDir()
	if serviceAccountAt(dir) {
		t.Fatal("vazio")
	}
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("tok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !serviceAccountAt(dir) {
		t.Fatal("token não detectado")
	}
	got := InClusterTarget()
	if ServiceAccountPresent() {
		if got.Status != "online" {
			t.Fatalf("%+v", got)
		}
	} else if os.Getenv("DOCKERPILOT_K8S_INCLUSTER") == "1" && got.InCluster && got.Detail == "" {
		t.Fatal("sem motivo")
	}
}

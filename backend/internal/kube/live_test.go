package kube

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestLiveKind(t *testing.T) {
	if _, err := exec.LookPath("kind"); err != nil {
		t.Skip("sem kind")
	}
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skip("sem docker")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	kubeconfig := t.TempDir() + "/kubeconfig"
	cmd := exec.CommandContext(ctx, "kind", "get", "kubeconfig", "--name", "dockpilot")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		t.Skip("cluster kind dockpilot indisponível")
	}
	if err := os.WriteFile(kubeconfig, out, 0o600); err != nil {
		t.Fatal(err)
	}
	set := Open([]Source{{ID: "kind", Label: "kind", Kubeconfig: kubeconfig}})
	c, err := set.Get("kind")
	if err != nil || c.Client == nil {
		t.Fatal(err)
	}
	ns := fmt.Sprintf("dp-live-%d", time.Now().UnixNano()%100000)
	t.Cleanup(func() { _ = c.DeleteNamespace(context.Background(), ns) })
	manifest := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: ` + ns + `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: ` + ns + `
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: web
        image: nginx:1.27-alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: ` + ns + `
spec:
  selector:
    app: web
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: web-config
  namespace: ` + ns + `
data:
  index: hello
`)
	applied, err := c.ApplyYAML(ctx, manifest, ns)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) < 3 {
		t.Fatalf("applied %v", applied)
	}
	nss, err := c.Namespaces(ctx)
	if err != nil || !hasNS(nss, ns) {
		t.Fatalf("ns %+v %v", nss, err)
	}
	if err := c.Scale(ctx, "Deployment", ns, "web", 2); err != nil {
		t.Fatal(err)
	}
	if err := c.RolloutRestart(ctx, "Deployment", ns, "web"); err != nil {
		t.Fatal(err)
	}
	wls, err := c.Workloads(ctx, ns)
	if err != nil || len(wls) == 0 || wls[0].Kind != "Deployment" {
		t.Fatalf("workloads %+v %v", wls, err)
	}
	svcs, err := c.Services(ctx, ns)
	if err != nil || len(svcs) == 0 {
		t.Fatalf("svc %+v %v", svcs, err)
	}
	cms, err := c.ConfigMaps(ctx, ns)
	if err != nil || !hasKey(cms, "web-config") {
		t.Fatalf("cm %+v %v", cms, err)
	}
	secs, err := c.Secrets(ctx, ns)
	if err != nil {
		t.Fatal(err)
	}
	_ = secs
	ings, err := c.Ingresses(ctx, ns)
	if err != nil {
		t.Fatal(err)
	}
	_ = ings
	if _, err := c.Events(ctx, ns); err != nil {
		t.Fatal(err)
	}

	bad := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: broken
  namespace: ` + ns + `
spec:
  replicas: 1
  selector:
    matchLabels:
      app: broken
  template:
    metadata:
      labels:
        app: broken
    spec:
      containers:
      - name: app
        image: example.invalid/no-such-image:missing
`)
	if _, err := c.ApplyYAML(ctx, bad, ns); err != nil {
		t.Fatal(err)
	}
	var problems []Problem
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		problems, err = c.WhatBroke(ctx, ns)
		if err == nil && hasPull(problems) {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if !hasPull(problems) {
		t.Fatalf("sem ImagePull em %+v (%v)", problems, err)
	}
	pods, err := c.Pods(ctx, ns)
	if err != nil || len(pods) == 0 {
		t.Fatalf("pods %+v %v", pods, err)
	}
	for _, p := range pods {
		if strings.HasPrefix(p.Name, "web-") && p.Phase == "Running" {
			logs, err := c.PodLogs(ctx, ns, p.Name, "web", 20)
			if err != nil {
				t.Fatal(err)
			}
			_ = logs
			out, err := c.Exec(ctx, ns, p.Name, []string{"echo", "ok"})
			if err != nil || !strings.Contains(out, "ok") {
				t.Fatalf("exec %q %v", out, err)
			}
			break
		}
	}
}

func hasNS(list []NamespaceView, name string) bool {
	for _, n := range list {
		if n.Name == name {
			return true
		}
	}
	return false
}

func hasKey(list []NamedKeys, name string) bool {
	for _, n := range list {
		if n.Name == name {
			return true
		}
	}
	return false
}

func hasPull(list []Problem) bool {
	for _, p := range list {
		if p.Reason == "ImagePullBackOff" || p.Reason == "ErrImagePull" {
			return true
		}
	}
	return false
}

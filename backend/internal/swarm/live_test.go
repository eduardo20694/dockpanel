package swarm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

func TestLiveSwarm(t *testing.T) {
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skip("sem docker")
	}
	cli, err := client.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock"), client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := Active(ctx, cli); err != nil {
		t.Skip(err)
	}

	nodes, err := ListNodes(ctx, cli)
	if err != nil || len(nodes) == 0 || nodes[0].Role == "" {
		t.Fatalf("nodes %+v %v", nodes, err)
	}

	name := "dp-live-svc"
	_ = RemoveService(ctx, cli, name)
	if _, err := CreateService(ctx, cli, name, "busybox:1.36", 1); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = RemoveService(context.Background(), cli, name) })

	if err := Scale(ctx, cli, name, 2); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(40 * time.Second)
	var tasks []TaskView
	for time.Now().Before(deadline) {
		tasks, err = Tasks(ctx, cli, name)
		if err == nil && len(tasks) >= 2 {
			break
		}
		time.Sleep(time.Second)
	}
	if len(tasks) < 2 {
		t.Fatalf("tasks %+v %v", tasks, err)
	}
	if _, err := Logs(ctx, cli, name, "20"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateImage(ctx, cli, name, "busybox:1.36"); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(ctx, cli, name); err != nil {
		t.Fatal(err)
	}

	secID, err := CreateSecret(ctx, cli, "dp-live-secret", []byte("segredo"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = RemoveSecret(context.Background(), cli, secID) })
	secs, err := ListSecrets(ctx, cli)
	if err != nil || !containsName(secs, "dp-live-secret") {
		t.Fatalf("secrets %+v %v", secs, err)
	}
	cfgID, err := CreateConfig(ctx, cli, "dp-live-config", []byte("chave=valor"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = RemoveConfig(context.Background(), cli, cfgID) })
	cfgs, err := ListConfigs(ctx, cli)
	if err != nil || !containsName(cfgs, "dp-live-config") {
		t.Fatalf("configs %+v %v", cfgs, err)
	}

	compose := []byte("services:\n  web:\n    image: nginx:1.27-alpine\n    deploy:\n      replicas: 1\n")
	out, err := DeployStack(ctx, cli, "", "dp-live-stack", compose)
	if err != nil {
		t.Fatal(err, out)
	}
	t.Cleanup(func() { _ = RemoveStack(context.Background(), cli, "", "dp-live-stack") })
	time.Sleep(2 * time.Second)
	stacks, err := ListStacks(ctx, cli)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range stacks {
		if s.Name == "dp-live-stack" && len(s.Services) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("stacks %+v output %s", stacks, out)
	}
	if err := RemoveStack(ctx, cli, "", "dp-live-stack"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nodes[0].State, "ready") && nodes[0].State != "ready" {
		t.Fatalf("node state %s", nodes[0].State)
	}
}

func containsName(list []Named, name string) bool {
	for _, n := range list {
		if n.Name == name {
			return true
		}
	}
	return false
}

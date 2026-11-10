package discover

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

func TestLiveEngineAndUnreachable(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.WithHost("unix:///tmp/dockpilot-ausente.sock"), client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	facts := ProbeEngine(context.Background(), cli, "unix:///tmp/dockpilot-ausente.sock")
	got := Classify("", facts)
	if got.Status != "offline" || !strings.Contains(got.Detail, "Falha de conexão") {
		t.Fatalf("%+v facts=%+v", got, facts)
	}

	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skip("sem docker")
	}
	live, err := client.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock"), client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	okFacts := ProbeEngine(ctx, live, "")
	view := Classify("auto", okFacts)
	t.Logf("daemon %+v view %+v", okFacts, view)
	state := strings.ToLower(okFacts.SwarmState)
	switch {
	case state == "active" && okFacts.Control:
		if view.Kind != "swarm" || view.Role != "manager" || !view.ManageSwarm() || view.Status != "online" {
			t.Fatalf("manager %+v facts=%+v", view, okFacts)
		}
	case state == "active" && !okFacts.Control && okFacts.NodeID != "":
		if view.Kind != "swarm" || view.Role != "worker" || view.ManageSwarm() {
			t.Fatalf("worker %+v facts=%+v", view, okFacts)
		}
	default:
		if view.Kind != "docker" && view.Kind != "podman" {
			t.Fatalf("sem swarm %+v facts=%+v", view, okFacts)
		}
		if view.Role != "" || view.ManageSwarm() || view.Status == "offline" {
			t.Fatalf("sem swarm %+v facts=%+v", view, okFacts)
		}
	}
}

func TestLiveKindContext(t *testing.T) {
	if _, err := exec.LookPath("kind"); err != nil {
		t.Skip("sem kind")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kind", "get", "kubeconfig", "--name", "dockpilot")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		t.Skip("cluster kind ausente")
	}
	path := t.TempDir() + "/config"
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	targets, err := ExpandKube("kind", "Kind", path, "")
	if err != nil || len(targets) == 0 || targets[0].Status != "online" || targets[0].Context == "" {
		t.Fatalf("%+v %v", targets, err)
	}
}

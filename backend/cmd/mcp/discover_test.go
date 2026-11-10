package main

import (
	"context"
	"strings"
	"testing"

	"dockerpilot/internal/dockerclient"
)

func TestDescribeEnvironmentsOffline(t *testing.T) {
	pool, err := dockerclient.NewPool([]dockerclient.HostConfig{
		{ID: "local", Label: "Local", DockerHost: "unix:///tmp/dockpilot-ausente.sock"},
		{ID: "pod", Label: "Podman", Kind: "podman", DockerHost: "unix:///run/podman/podman.sock"},
	}, "local")
	if err != nil {
		t.Fatal(err)
	}
	envs := describeEnvironments(context.Background(), pool, nil)
	raw := ""
	for _, env := range envs {
		raw += env["id"].(string) + " " + env["kind"].(string) + " " + env["status"].(string) + " " + env["detail"].(string) + "\n"
		caps, _ := env["capabilities"].([]string)
		if env["status"] == "offline" && len(caps) != 0 && env["kind"] != "podman" {
			t.Fatalf("capabilities %v", env)
		}
	}
	if !strings.Contains(raw, "local docker offline") || !strings.Contains(raw, "Falha de conexão") {
		t.Fatal(raw)
	}
	if !strings.Contains(raw, "pod podman offline") {
		t.Fatal(raw)
	}
}

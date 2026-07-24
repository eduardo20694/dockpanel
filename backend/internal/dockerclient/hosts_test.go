package dockerclient_test

import (
	"testing"

	"dockpanel/internal/dockerclient"
)

func TestGetIfAllowedBlocksForeignHost(t *testing.T) {
	pool, err := dockerclient.NewPool([]dockerclient.HostConfig{
		{ID: "env-a", Label: "A", DockerHost: ""},
		{ID: "env-b", Label: "B", DockerHost: ""},
	}, "env-a")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"env-a": true}
	if _, _, err := pool.GetIfAllowed("env-b", allowed); err == nil {
		t.Fatal("expected foreign host to be denied")
	}
	cli, id, err := pool.GetIfAllowed("env-a", allowed)
	if err != nil || cli == nil || id != "env-a" {
		t.Fatalf("allowed host failed: %v id=%s", err, id)
	}
}

func TestBaselineDoesNotIncludeMerged(t *testing.T) {
	pool, err := dockerclient.NewPool([]dockerclient.HostConfig{
		{ID: "env", Label: "env", DockerHost: ""},
	}, "env")
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.UpsertHost(dockerclient.HostConfig{ID: "org-b-server", Label: "leak", DockerHost: ""}); err != nil {
		t.Fatal(err)
	}
	base := pool.Baseline()
	for _, h := range base {
		if h.ID == "org-b-server" {
			t.Fatal("merged org host must not appear in Baseline")
		}
	}
	if !pool.IsBaseline("env") {
		t.Fatal("env should be baseline")
	}
	if pool.IsBaseline("org-b-server") {
		t.Fatal("org server must not be baseline")
	}
}

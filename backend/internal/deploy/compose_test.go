package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindComposeFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := findComposeFile(dir); err == nil {
		t.Fatal("expected error for empty dir")
	}

	name := "docker-compose.yml"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("services: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := findComposeFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != name {
		t.Fatalf("got %q want %q", got, name)
	}
}

func TestFindComposeFileVariants(t *testing.T) {
	tests := []struct {
		file string
	}{
		{"docker-compose.yaml"},
		{"compose.yml"},
		{"compose.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.file), []byte("services: {}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			got, err := findComposeFile(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.file {
				t.Fatalf("got %q want %q", got, tt.file)
			}
		})
	}
}

func TestRunComposeValidation(t *testing.T) {
	tests := []struct {
		name string
		req  ComposeRequest
	}{
		{name: "empty path", req: ComposeRequest{}},
		{name: "missing dir", req: ComposeRequest{ProjectPath: t.TempDir() + "/nope"}},
		{name: "file not dir", req: ComposeRequest{ProjectPath: writeTempFile(t, "notdir.txt", "x")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RunCompose(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRunComposeInvalidAction(t *testing.T) {
	dir := composeProjectDir(t)
	_, err := RunCompose(context.Background(), ComposeRequest{
		ProjectPath: dir,
		Action:      "destroy",
	})
	if err == nil || !strings.Contains(err.Error(), "ação inválida") {
		t.Fatalf("expected invalid action error, got %v", err)
	}
}

func TestRunComposeNoComposeFile(t *testing.T) {
	dir := t.TempDir()
	_, err := RunCompose(context.Background(), ComposeRequest{ProjectPath: dir, Action: "ps"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePSJSON(t *testing.T) {
	input := `{"Name":"dockpanel-backend","Service":"backend","State":"running","Status":"Up 2 hours","Ports":"","Image":"dockpanel-backend","Health":""}
{"Name":"dockpanel-frontend","Service":"frontend","State":"running","Status":"Up 2 hours","Ports":"0.0.0.0:9090->80/tcp","Image":"dockpanel-frontend","Health":""}`

	services, err := ParsePSJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 2 {
		t.Fatalf("got %d services", len(services))
	}
	if services[0].Service != "backend" || services[1].Ports == "" {
		t.Fatalf("unexpected parse: %+v", services)
	}
}

func TestParsePSJSONEmpty(t *testing.T) {
	services, err := ParsePSJSON("")
	if err != nil {
		t.Fatal(err)
	}
	if services != nil {
		t.Fatalf("expected nil, got %v", services)
	}
}

func TestParsePSJSONInvalid(t *testing.T) {
	_, err := ParsePSJSON("{bad json")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDockerHostLabel(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	if got := DockerHostLabel(); got != "local (socket/pipe padrão)" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("DOCKER_HOST", "ssh://root@1.2.3.4")
	if got := DockerHostLabel(); got != "ssh://root@1.2.3.4" {
		t.Fatalf("got %q", got)
	}
}

func TestPresetsWithCompose(t *testing.T) {
	dir := composeProjectDir(t)
	t.Setenv("DOCKPANEL_COMPOSE_PATH", dir)
	presets := Presets()
	if len(presets) != 1 {
		t.Fatalf("expected 1 preset, got %d", len(presets))
	}
	if presets[0].ComposeFile != "docker-compose.yml" {
		t.Fatalf("compose file %q", presets[0].ComposeFile)
	}
}

func TestPresetsEmptyWhenNoCompose(t *testing.T) {
	t.Setenv("DOCKPANEL_COMPOSE_PATH", t.TempDir())
	if presets := Presets(); len(presets) != 0 {
		t.Fatalf("expected no presets, got %v", presets)
	}
}

func composeProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

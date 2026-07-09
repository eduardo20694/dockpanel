package drift

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeImage(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"nginx", "nginx:latest"},
		{"NGINX", "nginx:latest"},
		{"nginx:1.25", "nginx:1.25"},
		{"repo/app@sha256:abc", "repo/app:latest"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeImage(tt.in); got != tt.want {
				t.Fatalf("normalizeImage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestImageCompatible(t *testing.T) {
	if !imageCompatible("nginx:1.25", "nginx:1.25") {
		t.Fatal("same tag should match")
	}
	if imageCompatible("nginx:1.25", "nginx:2.0") {
		t.Fatal("different tags should not match")
	}
	if !imageCompatible("nginx", "nginx:latest") {
		t.Fatal("implicit latest should match")
	}
}

func TestLoadCompose(t *testing.T) {
	dir := t.TempDir()
	yaml := `services:
  api:
    image: myapp:1.0
  db:
    image: mysql:8
`
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	abs, name, services, project, err := loadCompose(dir)
	if err != nil {
		t.Fatal(err)
	}
	if name != "docker-compose.yml" || project != filepath.Base(abs) {
		t.Fatalf("name=%q project=%q", name, project)
	}
	if len(services) != 2 || services["api"].Image != "myapp:1.0" {
		t.Fatalf("services=%v", services)
	}
}

func TestLoadComposeMissing(t *testing.T) {
	_, _, _, _, err := loadCompose(t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseComposeEnv(t *testing.T) {
	env := parseComposeEnv([]interface{}{
		"FOO=bar",
		"BAZ=1",
		123,
	})
	if env["FOO"] != "bar" || env["BAZ"] != "1" {
		t.Fatalf("env=%v", env)
	}
}

func TestEnvDiff(t *testing.T) {
	want := map[string]string{"A": "1", "B": "2"}
	got := []string{"A=1", "B=9", "C=3"}
	missing, extra := envDiff(want, got)
	if len(missing) == 0 || len(extra) == 0 {
		t.Fatalf("missing=%v extra=%v", missing, extra)
	}
}

func TestPortsMatch(t *testing.T) {
	if !portsMatch(nil, nil) {
		t.Fatal("empty should match")
	}
	if portsMatch([]string{"8080:80"}, nil) {
		t.Fatal("missing ports should not match")
	}
	if !portsMatch([]string{"8080:80"}, []string{"0.0.0.0:8080->80/tcp"}) {
		t.Fatal("same count should match in simplified mode")
	}
}

func TestFormatComposePorts(t *testing.T) {
	out := formatComposePorts([]interface{}{"8080:80", 9090})
	if len(out) != 2 || out[0] != "8080:80" {
		t.Fatalf("got %v", out)
	}
}

package deploy

import (
	"os"
	"path/filepath"
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

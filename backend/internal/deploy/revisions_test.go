package deploy

import (
	"strings"
	"testing"
)

func TestRevisionsRoundTripAndRollbackFile(t *testing.T) {
	data := t.TempDir()
	project := t.TempDir()
	compose := "services:\n  api:\n    image: ghcr.io/acme/api:1.4.0\n"
	rev, err := SaveRevision(data, Revision{Project: "api", Path: project, Compose: compose, Actor: "ana", Action: "up"})
	if err != nil {
		t.Fatal(err)
	}
	if rev.Tag != "1.4.0" {
		t.Fatalf("tag %q", rev.Tag)
	}
	list, err := ListRevisions(data, "api")
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %+v", err, list)
	}
	loaded, err := LoadRevision(data, rev.ID)
	if err != nil || !strings.Contains(loaded.Compose, "1.4.0") {
		t.Fatal(err)
	}
	name, err := WriteComposeFile(project, loaded.Compose)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, "docker-compose.yml") {
		t.Fatal(name)
	}
	image, tag := FirstImageTag("image: redis:7\n")
	if image != "redis:7" || tag != "7" {
		t.Fatalf("%s %s", image, tag)
	}
}

func TestRejectBadRevisionID(t *testing.T) {
	if _, err := LoadRevision(t.TempDir(), "../etc/passwd"); err == nil {
		t.Fatal("expected reject")
	}
}

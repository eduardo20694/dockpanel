package secrets

import (
	"os"
	"strings"
	"testing"
)

func TestVaultRoundTripAndScan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_SECRETS_KEY", "unit-test-master-key")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := s.Put("default", "api", "db", "env", "DB_PASSWORD", "s3cret-value")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := readFile(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "s3cret-value") {
		t.Fatal("plaintext stored")
	}
	list := s.List("default", "api")
	if len(list) != 1 || list[0].Name != "db" || list[0].ID != meta.ID {
		t.Fatalf("%+v", list)
	}
	env, files, err := s.Materialize("default", "api")
	if err != nil || env["DB_PASSWORD"] != "s3cret-value" || len(files) != 0 {
		t.Fatalf("%v %v %v", env, files, err)
	}
	if _, err := s.Put("default", "api", "db", "env", "DB_PASSWORD", "other"); err != nil {
		t.Fatal(err)
	}
	env, _, err = s.Materialize("default", "api")
	if err != nil || env["DB_PASSWORD"] != "other" || len(s.List("default", "api")) != 1 {
		t.Fatalf("replace %v %v", env, err)
	}

	t.Setenv("DOCKERPILOT_SECRETS_KEY", "another-key")
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s2.Materialize("default", "api"); err == nil {
		t.Fatal("expected key mismatch")
	}

	findings := ScanCompose("services:\n  db:\n    environment:\n      PASSWORD: plain\n      TOKEN: ${TOKEN}\n")
	if len(findings) != 1 || findings[0].Line != 4 || !strings.Contains(findings[0].Suggestion, "cofre") {
		t.Fatalf("%+v", findings)
	}
	if Redact("password: hunter2") != "password:***" && !strings.Contains(Redact("password: hunter2"), "***") {
		t.Fatal(Redact("password: hunter2"))
	}
}

func readFile(s *Store) (string, error) {
	b, err := os.ReadFile(s.path)
	return string(b), err
}

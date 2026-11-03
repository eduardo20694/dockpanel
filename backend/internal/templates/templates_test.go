package templates

import (
	"os"
	"strings"
	"testing"
)

func TestPrepareSendsSecretsOutOfCompose(t *testing.T) {
	if len(Catalog()) < 8 {
		t.Fatalf("catalog %d", len(Catalog()))
	}
	dir := t.TempDir()
	prep, err := Prepare(Request{
		ID: "postgres", Stack: "db", ProjectPath: dir,
		Vars: map[string]string{"POSTGRES_PASSWORD": "s3cret-db"},
	}, dir)
	if err != nil {
		t.Fatal(err)
	}
	text := string(prep.Compose)
	if strings.Contains(text, "s3cret-db") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "${POSTGRES_PASSWORD}") || prep.Secrets["POSTGRES_PASSWORD"] != "s3cret-db" {
		t.Fatalf("%s %+v", text, prep.Secrets)
	}
	raw, err := os.ReadFile(dir + "/docker-compose.yml")
	if err != nil || strings.Contains(string(raw), "s3cret-db") {
		t.Fatal(string(raw))
	}
	if _, ok := ByID("grafana-prometheus"); !ok {
		t.Fatal("grafana")
	}
}

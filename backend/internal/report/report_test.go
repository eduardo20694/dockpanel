package report

import (
	"strings"
	"testing"
	"time"
)

func TestMarkdownAndPDF(t *testing.T) {
	in := Input{
		Title: "Incidente api", Session: "abc", Incident: "crash loop",
		Problems: []string{"OOM no worker"}, Actions: []string{"rollback para 1.3.0"},
		FinalState: "saudável", When: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC),
	}
	md, pdf := Build(in)
	if !strings.Contains(md, "Dockpilot") || !strings.Contains(md, "crash loop") || !strings.Contains(md, "rollback") {
		t.Fatal(md)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") || !strings.Contains(string(pdf), "DOCKPILOT") || !strings.Contains(string(pdf), "crash loop") {
		t.Fatalf("pdf %q", pdf[:80])
	}
	id, _, bin, err := Save(t.TempDir(), in)
	if err != nil || id == "" || len(bin) == 0 {
		t.Fatal(err)
	}
}

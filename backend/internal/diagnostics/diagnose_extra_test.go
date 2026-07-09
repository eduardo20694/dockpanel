package diagnostics

import (
	"testing"

	"github.com/docker/docker/api/types/events"
)

func TestCleanDockerLogLine(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello world", "hello world"},
		{"  trimmed  ", "trimmed"},
		{"line\x00with\x01ctrl", "linewithctrl"},
	}
	for _, tt := range tests {
		if got := cleanDockerLogLine(tt.in); got != tt.want {
			t.Fatalf("cleanDockerLogLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEventMatchesContainer(t *testing.T) {
	ev := events.Message{Actor: events.Actor{ID: "abc123def456"}}
	if !eventMatchesContainer(ev, "abc123") {
		t.Fatal("prefix match expected")
	}
	if eventMatchesContainer(ev, "zzz") {
		t.Fatal("unrelated id should not match")
	}
}

func TestClassifyExtended(t *testing.T) {
	if classify(&Diagnosis{RestartCount: 1, State: "running"}) != SeverityWarning {
		t.Fatal("restart > 0 should warn")
	}
	if classify(&Diagnosis{State: "running", ExitCode: 0, RestartCount: 0}) != SeverityOK {
		t.Fatal("healthy running should be ok")
	}
}

func TestRecommendDefault(t *testing.T) {
	got := recommend(&Diagnosis{State: "running"})
	if got != "nenhum problema óbvio encontrado" {
		t.Fatalf("got %q", got)
	}
}

package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestTerminalPolicyAndInput(t *testing.T) {
	t.Setenv("DOCKERPILOT_READONLY", "1")
	if err := terminalPolicy("admin"); err == nil {
		t.Fatal("readonly")
	}
	t.Setenv("DOCKERPILOT_READONLY", "")
	if err := terminalPolicy("viewer"); err == nil || terminalPolicy("auditor") == nil {
		t.Fatal("role")
	}
	if err := terminalPolicy("operator"); err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	cmds := consumeTerminalInput(&buf, []byte("echo hi"))
	if len(cmds) != 0 {
		t.Fatal(cmds)
	}
	cmds = consumeTerminalInput(&buf, []byte("\rpassword=hunter2\n"))
	joined := strings.Join(cmds, "|")
	if !strings.Contains(joined, "echo hi") || strings.Contains(joined, "hunter2") || !strings.Contains(joined, "***") {
		t.Fatalf("%q", joined)
	}
}

func TestTerminalReadonlyHTTP(t *testing.T) {
	s := testServer(t)
	t.Setenv("DOCKERPILOT_READONLY", "1")
	tok := login(t, s, "admin@dockpilot.dev", "secret-pass")
	rr := do(s, http.MethodGet, "/api/containers/abc/terminal/ws", "", tok)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

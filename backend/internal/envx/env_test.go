package envx

import "testing"

func TestDockerpilotWinsOverLegacy(t *testing.T) {
	t.Setenv("DOCKERPILOT_ADMIN_EMAIL", "new@dockpilot.dev")
	t.Setenv("DOCKPANEL_ADMIN_EMAIL", "old@dockpanel.dev")
	if got := Get("ADMIN_EMAIL"); got != "new@dockpilot.dev" {
		t.Fatalf("got %q", got)
	}
}

func TestLegacyFallback(t *testing.T) {
	t.Setenv("DOCKERPILOT_HOSTS", "")
	t.Setenv("DOCKPANEL_HOSTS", `[{"id":"local"}]`)
	if got := GetRaw("HOSTS"); got != `[{"id":"local"}]` {
		t.Fatalf("got %q", got)
	}
}

func TestReadOnly(t *testing.T) {
	t.Setenv("DOCKERPILOT_READONLY", "1")
	if !ReadOnly() {
		t.Fatal("expected read-only")
	}
	t.Setenv("DOCKERPILOT_READONLY", "")
	t.Setenv("DOCKPANEL_READONLY", "true")
	if !ReadOnly() {
		t.Fatal("expected legacy read-only")
	}
	t.Setenv("DOCKPANEL_READONLY", "")
	if ReadOnly() {
		t.Fatal("expected writable")
	}
}

func TestDataDirDefault(t *testing.T) {
	t.Setenv("DOCKERPILOT_DATA_DIR", "")
	t.Setenv("DOCKPANEL_DATA_DIR", "")
	if DataDir() != "data" {
		t.Fatalf("got %q", DataDir())
	}
	t.Setenv("DOCKPANEL_DATA_DIR", "/tmp/legacy")
	if DataDir() != "/tmp/legacy" {
		t.Fatalf("got %q", DataDir())
	}
}

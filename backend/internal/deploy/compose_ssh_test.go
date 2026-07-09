package deploy

import "testing"

func TestIsSSHHost(t *testing.T) {
	if !IsSSHHost("ssh://root@1.2.3.4") {
		t.Fatal("expected ssh host")
	}
	if IsSSHHost("unix:///var/run/docker.sock") {
		t.Fatal("expected not ssh")
	}
}

func TestResolveComposePath(t *testing.T) {
	t.Setenv("DOCKPANEL_COMPOSE_PATH_REMOTE", "/opt/dockpanel")
	got := ResolveComposePath("ssh://root@host", `c:\Github\dockpanel`)
	if got != "/opt/dockpanel" {
		t.Fatalf("remote: got %q", got)
	}
	got = ResolveComposePath("ssh://root@host", "/srv/app")
	if got != "/srv/app" {
		t.Fatalf("explicit remote: got %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	if shellQuote("/root/dockpanel") != "'/root/dockpanel'" {
		t.Fatal(shellQuote("/root/dockpanel"))
	}
}

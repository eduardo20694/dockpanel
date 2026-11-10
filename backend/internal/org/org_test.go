package org

import "testing"

func TestUsersTeamsAndLogin(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Orgs()) != 1 || s.Orgs()[0].ID != "default" {
		t.Fatalf("%+v", s.Orgs())
	}
	u, err := s.CreateUser("ops@dockpilot.dev", "Ops", "senha-forte", "operator", "default")
	if err != nil {
		t.Fatal(err)
	}
	if u.PasswordHash != "" {
		t.Fatal("hash leaked")
	}
	if _, err := s.CreateUser("ops@dockpilot.dev", "Ops", "senha-forte", "operator", "default"); err != ErrExists {
		t.Fatalf("dup: %v", err)
	}
	got, ok, err := s.Authenticate("ops@dockpilot.dev", "senha-forte")
	if err != nil || !ok || got.Role != "operator" {
		t.Fatalf("auth %+v %v %v", got, ok, err)
	}
	if _, ok, err := s.Authenticate("ops@dockpilot.dev", "errada-demais"); err == nil || !ok {
		t.Fatalf("want bad password, ok=%v err=%v", ok, err)
	}
	team, err := s.CreateTeam("default", "Plataforma")
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.AddMember(team.ID, u.ID, "operator")
	if err != nil || m.UserID != u.ID {
		t.Fatal(err)
	}
	updated, err := s.UpdateUser(u.ID, "Ops 2", "viewer")
	if err != nil || updated.Role != "viewer" || updated.Name != "Ops 2" {
		t.Fatalf("%+v %v", updated, err)
	}
}

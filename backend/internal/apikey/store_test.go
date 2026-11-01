package apikey

import (
	"testing"
	"time"
)

func TestIssueAuthenticateRevokeExpire(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token, key, err := s.Issue("default", "ana", "laptop", "operator", time.Hour)
	if err != nil || token == "" || key.Prefix == "" {
		t.Fatal(err)
	}
	p, err := s.Authenticate(token)
	if err != nil || p.Role != "operator" || p.OrgID != "default" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.Authenticate("dp_live_nope"); err != ErrUnauthorized {
		t.Fatalf("got %v", err)
	}
	s.Touch(key.ID)
	list, err := s.List("default")
	if err != nil || len(list) != 1 || list[0].LastUsedAt == nil {
		t.Fatalf("%+v %v", list, err)
	}
	if err := s.Revoke("default", key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(token); err != ErrRevoked {
		t.Fatalf("got %v", err)
	}

	short, _, err := s.Issue("default", "ana", "curta", "viewer", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := s.Authenticate(short); err != ErrExpired {
		t.Fatalf("got %v", err)
	}
	if err := AllowTool(Principal{Role: "viewer"}, "container_action"); err != ErrForbidden {
		t.Fatalf("got %v", err)
	}
	if err := AllowTool(Principal{Role: "operator"}, "list_containers"); err != nil {
		t.Fatal(err)
	}
}

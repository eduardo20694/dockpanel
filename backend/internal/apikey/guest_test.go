package apikey

import (
	"testing"
	"time"
)

func TestGuestScopeRevoke(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.IssueGuest("default", "lia", "operator", 80, []string{"api"}, nil); err == nil {
		t.Fatal("hours")
	}
	if _, _, err := s.IssueGuest("default", "lia", "operator", 4, nil, nil); err == nil {
		t.Fatal("scope")
	}
	token, key, err := s.IssueGuest("default", "lia", "operator", 4, []string{"api"}, []string{"prod"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Authenticate(token)
	if err != nil || !p.Guest || p.AllowsCall("list_containers", "prod", "") != nil {
		t.Fatalf("%+v %v", p, err)
	}
	if p.AllowsCall("list_containers", "other", "") == nil {
		t.Fatal("host scope")
	}
	if p.AllowsCall("deploy_template", "prod", "other") == nil {
		t.Fatal("stack scope")
	}
	if err := s.Revoke("default", key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(token); err != ErrRevoked {
		t.Fatalf("%v", err)
	}
	if time.Until(*key.ExpiresAt) > 5*time.Hour || time.Until(*key.ExpiresAt) < 3*time.Hour {
		t.Fatal(key.ExpiresAt)
	}
}

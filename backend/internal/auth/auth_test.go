package auth

import "testing"

func TestLoginEnvAdmin(t *testing.T) {
	t.Setenv("DOCKPANEL_ADMIN_EMAIL", "admin@test.local")
	t.Setenv("DOCKPANEL_ADMIN_PASSWORD", "secret-pass")
	t.Setenv("DOCKPANEL_JWT_SECRET", "unit-test-secret-key")
	t.Setenv("DOCKPANEL_ADMIN_NAME", "Admin")

	svc, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	if !svc.Enabled {
		t.Fatal("expected enabled")
	}
	res, err := svc.Login("admin@test.local", "secret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if res.AccessToken == "" || res.User.Email != "admin@test.local" {
		t.Fatalf("bad result: %+v", res)
	}
	claims, err := svc.ParseToken(res.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Email != "admin@test.local" {
		t.Fatalf("claims: %+v", claims)
	}
	if _, err := svc.Login("admin@test.local", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("want invalid credentials, got %v", err)
	}
}

func TestAuthDisabledWithoutEnv(t *testing.T) {
	t.Setenv("DOCKPANEL_ADMIN_EMAIL", "")
	t.Setenv("DOCKPANEL_ADMIN_PASSWORD", "")
	svc, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled {
		t.Fatal("expected disabled")
	}
}

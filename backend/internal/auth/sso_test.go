package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestExchangeSSOMapsAdminGroup(t *testing.T) {
	t.Setenv("DOCKERPILOT_ADMIN_EMAIL", "admin@dockpilot.dev")
	t.Setenv("DOCKERPILOT_ADMIN_PASSWORD", "secret-pass")
	t.Setenv("DOCKERPILOT_JWT_SECRET", "unit-test-secret-key")
	t.Setenv("DOCKERPILOT_OIDC_ISSUER", "https://login.example.com")
	t.Setenv("DOCKERPILOT_OIDC_CLIENT_ID", "dockpilot")
	t.Setenv("DOCKERPILOT_OIDC_HMAC_SECRET", "sso-secret-for-tests")

	svc, err := NewService()
	if err != nil || !svc.Enabled {
		t.Fatal(err)
	}
	if !SSOConfig().Enabled || SSOConfig().Mode != "hmac" {
		t.Fatalf("%+v", SSOConfig())
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":    "https://login.example.com",
		"aud":    "dockpilot",
		"email":  "ana@org.dev",
		"name":   "Ana",
		"groups": []string{"dockpilot-admins"},
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString([]byte("sso-secret-for-tests"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := svc.ExchangeSSO(signed)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@org.dev" || u.Role != "admin" {
		t.Fatalf("%+v", u)
	}
	if _, err := svc.ExchangeSSO(signed + "x"); err == nil {
		t.Fatal("expected reject")
	}
}

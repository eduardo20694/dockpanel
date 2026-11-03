package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCPresetsAndRS256(t *testing.T) {
	g, err := PresetIssuer("google", "", "")
	if err != nil || g != "https://accounts.google.com" {
		t.Fatal(g, err)
	}
	e, err := PresetIssuer("entra", "contoso", "")
	if err != nil || e != "https://login.microsoftonline.com/contoso/v2.0" {
		t.Fatal(e, err)
	}
	k, err := PresetIssuer("keycloak", "dock", "https://id.example/auth")
	if err != nil || k != "https://id.example/auth/realms/dock" {
		t.Fatal(k, err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	eb := big.NewInt(int64(key.E)).Bytes()
	ee := base64.RawURLEncoding.EncodeToString(eb)
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": srv.URL, "authorization_endpoint": srv.URL + "/auth",
			"token_endpoint": srv.URL + "/token", "jwks_uri": srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1", "n": n, "e": ee}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": srv.URL, "email": "ana@org.dev", "name": "Ana", "groups": []string{"dockpilot-admins"},
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tok.Header["kid"] = "k1"
		signed, err := tok.SignedString(key)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed})
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("DOCKERPILOT_OIDC_ADMIN_GROUP", "dockpilot-admins")
	o := OIDC{Issuer: srv.URL, ClientID: "dock", ClientSecret: "sec", Redirect: "http://localhost/cb", HTTP: srv.Client()}
	url, err := o.AuthURL(context.Background(), "xyz")
	if err != nil || url == "" {
		t.Fatal(err, url)
	}
	user, err := o.Exchange(context.Background(), "code")
	if err != nil || user.Email != "ana@org.dev" || user.Role != "admin" {
		t.Fatalf("%+v %v", user, err)
	}
}

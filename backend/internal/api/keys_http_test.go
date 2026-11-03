package api

import (
	"net/http"
	"strings"
	"testing"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/auth"
	"dockerpilot/internal/org"
)

func TestAPIKeyHTTP(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_ADMIN_EMAIL", "admin@dockpilot.dev")
	t.Setenv("DOCKERPILOT_ADMIN_PASSWORD", "secret-pass")
	t.Setenv("DOCKERPILOT_JWT_SECRET", "unit-test-secret-key-32-characters!")
	svc, err := auth.NewService()
	if err != nil {
		t.Fatal(err)
	}
	orgs, err := org.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := apikey.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServerFromDeps(Deps{Auth: svc, Orgs: orgs, Keys: keys, DataDir: dir})
	login := do(s, http.MethodPost, "/api/auth/login", `{"email":"admin@dockpilot.dev","password":"secret-pass"}`, "")
	var tok string
	for _, c := range login.Result().Cookies() {
		if c.Name == "dockerpilot_token" {
			tok = c.Value
		}
	}
	if tok == "" {
		t.Fatal(login.Body.String())
	}

	gone := do(s, http.MethodGet, "/api/billing", "", tok)
	if gone.Code != http.StatusNotFound {
		t.Fatalf("billing %d %s", gone.Code, gone.Body.String())
	}

	createdUser := do(s, http.MethodPost, "/api/users", `{"email":"a@b.c","name":"A","password":"senha-forte","role":"viewer"}`, tok)
	if createdUser.Code != http.StatusOK {
		t.Fatalf("user %d %s", createdUser.Code, createdUser.Body.String())
	}

	created := do(s, http.MethodPost, "/api/api-keys", `{"name":"claude","role":"operator","ttlDays":30}`, tok)
	if created.Code != 200 || !strings.Contains(created.Body.String(), "dp_live_") {
		t.Fatalf("key %d %s", created.Code, created.Body.String())
	}
	listed := do(s, http.MethodGet, "/api/api-keys", "", tok)
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), `"name":"claude"`) {
		t.Fatalf("list %d %s", listed.Code, listed.Body.String())
	}
	id := ""
	if i := strings.Index(listed.Body.String(), `"id":"`); i >= 0 {
		rest := listed.Body.String()[i+6:]
		if j := strings.Index(rest, `"`); j > 0 {
			id = rest[:j]
		}
	}
	if id == "" {
		t.Fatalf("id ausente em %s", listed.Body.String())
	}
	revoked := do(s, http.MethodDelete, "/api/api-keys/"+id, "", tok)
	if revoked.Code != 200 || !strings.Contains(revoked.Body.String(), `"ok":true`) {
		t.Fatalf("revoke %d %s", revoked.Code, revoked.Body.String())
	}
}

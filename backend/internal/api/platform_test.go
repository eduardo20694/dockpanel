package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/gitops"
	"dockerpilot/internal/secrets"
)

type stubFetch struct{}

func (stubFetch) Fetch(ctx context.Context, link gitops.Link) (gitops.Remote, error) {
	return gitops.Remote{Commit: "abc", Compose: []byte("services: {}\n")}, nil
}

func TestPlatformHTTP(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_SECRETS_KEY", "http-master")
	s := testServer(t)
	keys, err := apikey.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	sec, err := secrets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	gitStore, err := gitops.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Keys = keys
	s.Secrets = sec
	s.GitOps = gitStore
	s.DataDir = dir
	tok := login(t, s, "admin@dockpilot.dev", "secret-pass")

	created := do(s, http.MethodPost, "/api/sessions", `{"name":"Lia","role":"operator","hours":2,"stacks":["api"],"hosts":["prod"]}`, tok)
	if created.Code != 200 || !strings.Contains(created.Body.String(), "dp_live_") {
		t.Fatalf("session %d %s", created.Code, created.Body.String())
	}
	id := between(created.Body.String(), `"id":"`, `"`)
	line := do(s, http.MethodGet, "/api/sessions/"+id+"/timeline", "", tok)
	if line.Code != 200 || !strings.Contains(line.Body.String(), "guest invite") {
		t.Fatalf("timeline %d %s", line.Code, line.Body.String())
	}
	rev := do(s, http.MethodDelete, "/api/sessions/"+id, "", tok)
	if rev.Code != 200 {
		t.Fatalf("revoke %d %s", rev.Code, rev.Body.String())
	}

	put := do(s, http.MethodPut, "/api/secrets", `{"stack":"api","name":"db","kind":"env","target":"DB_PASSWORD","value":"s3cret-value"}`, tok)
	if put.Code != 200 || strings.Contains(put.Body.String(), "s3cret-value") {
		t.Fatalf("secret %d %s", put.Code, put.Body.String())
	}
	list := do(s, http.MethodGet, "/api/secrets?stack=api", "", tok)
	if !strings.Contains(list.Body.String(), "DB_PASSWORD") || strings.Contains(list.Body.String(), "s3cret-value") {
		t.Fatalf("list %s", list.Body.String())
	}
	scan := do(s, http.MethodPost, "/api/secrets/scan", `{"compose":"PASSWORD: plain"}`, tok)
	if !strings.Contains(scan.Body.String(), "cofre") {
		t.Fatalf("scan %s", scan.Body.String())
	}

	secret := "whsec"
	on := true
	if err := gitStore.SetGlobal(gitops.GlobalInput{Enabled: &on, WebhookSecret: &secret}); err != nil {
		t.Fatal(err)
	}
	if err := gitStore.Upsert(gitops.LinkInput{Stack: "api", RepoURL: "https://github.com/acme/api.git", Branch: "main", Mode: "approval", Enabled: &on}); err != nil {
		t.Fatal(err)
	}
	gitStore.Fetch = stubFetch{}
	req := httptest.NewRequest(http.MethodPost, "/api/gitops/webhook", strings.NewReader(`{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/acme/api.git"}}`))
	req.Header.Set("X-Hub-Signature-256", "sha256=nope")
	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("webhook %d %s", rr.Code, rr.Body.String())
	}

	rep := do(s, http.MethodPost, "/api/reports", `{"title":"Incidente","incident":"crash loop","finalState":"saudável","session":"`+id+`"}`, tok)
	if rep.Code != 200 || !strings.Contains(rep.Body.String(), "Dockpilot") {
		t.Fatalf("report %d %s", rep.Code, rep.Body.String())
	}
	rid := between(rep.Body.String(), `"id":"`, `"`)
	pdf := do(s, http.MethodGet, "/api/reports/"+rid+"/pdf", "", tok)
	if pdf.Code != 200 || !strings.HasPrefix(pdf.Body.String(), "%PDF") {
		t.Fatalf("pdf %d %s", pdf.Code, pdf.Body.String())
	}
	tpl := do(s, http.MethodGet, "/api/templates", "", tok)
	if !strings.Contains(tpl.Body.String(), "n8n") || !strings.Contains(tpl.Body.String(), "traefik") {
		t.Fatalf("templates %s", tpl.Body.String())
	}
}

func between(s, a, b string) string {
	i := strings.Index(s, a)
	if i < 0 {
		return ""
	}
	s = s[i+len(a):]
	j := strings.Index(s, b)
	if j < 0 {
		return ""
	}
	return s[:j]
}

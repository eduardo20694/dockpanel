package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dockerpilot/internal/audit"
	"dockerpilot/internal/auth"
	"dockerpilot/internal/logcenter"
	"dockerpilot/internal/org"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/triage"

	"github.com/golang-jwt/jwt/v5"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_DATA_DIR", dir)
	t.Setenv("DOCKERPILOT_ADMIN_EMAIL", "admin@dockpilot.dev")
	t.Setenv("DOCKERPILOT_ADMIN_PASSWORD", "secret-pass")
	t.Setenv("DOCKERPILOT_JWT_SECRET", "unit-test-secret-key-32-characters!")
	t.Setenv("DOCKERPILOT_ADMIN_NAME", "Admin")
	t.Setenv("DOCKERPILOT_READONLY", "")
	t.Setenv("DOCKPANEL_READONLY", "")
	svc, err := auth.NewService()
	if err != nil {
		t.Fatal(err)
	}
	logs, err := logcenter.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logs.Close() })
	aud, err := audit.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	orgs, err := org.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := resolved.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return NewServerFromDeps(Deps{
		Auth: svc, LogStore: logs, Audit: aud, Orgs: orgs, Resolved: res, DataDir: dir,
	})
}

func do(s *Server, method, path, body, cookie string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "dockerpilot_token", Value: cookie})
	}
	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)
	return rr
}

func login(t *testing.T, s *Server, email, password string) string {
	t.Helper()
	rr := do(s, http.MethodPost, "/api/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("login %d %s", rr.Code, rr.Body.String())
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "dockerpilot_token" && c.Value != "" {
			return c.Value
		}
	}
	t.Fatal("missing cookie")
	return ""
}

func TestIntegrationAuthTriageRetentionResolved(t *testing.T) {
	s := testServer(t)
	tok := login(t, s, "admin@dockpilot.dev", "secret-pass")

	me := do(s, http.MethodGet, "/api/auth/me", "", tok)
	if me.Code != 200 || !strings.Contains(me.Body.String(), "admin@dockpilot.dev") {
		t.Fatalf("me %d %s", me.Code, me.Body.String())
	}

	// Legacy header is accepted by the host resolver (no docker needed for triage).
	if err := triage.SaveShip(s.DataDir, triage.ShipRecord{
		At: time.Now().Add(-30 * time.Minute), Tag: "1.4.0", PreviousTag: "1.3.0",
		Project: "api", HostID: "default", Actor: "ana",
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/triage", nil)
	req.Header.Set("X-Dockpanel-Host", "default")
	req.AddCookie(&http.Cookie{Name: "dockerpilot_token", Value: tok})
	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "1.4.0") || !strings.Contains(rr.Body.String(), "story") {
		t.Fatalf("triage %d %s", rr.Code, rr.Body.String())
	}

	put := do(s, http.MethodPut, "/api/logs/incidents/retention", `{"days":15}`, tok)
	if put.Code != 200 || !strings.Contains(put.Body.String(), `"days":15`) {
		t.Fatalf("retention put %d %s", put.Code, put.Body.String())
	}
	get := do(s, http.MethodGet, "/api/logs/retention", "", tok)
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"days":15`) {
		t.Fatalf("retention get %d %s", get.Code, get.Body.String())
	}

	res := do(s, http.MethodPost, "/api/system/problems/resolve", `{"containerId":"abc","name":"api","reason":"crash loop"}`, tok)
	if res.Code != 200 {
		t.Fatalf("resolve %d %s", res.Code, res.Body.String())
	}
	listed := do(s, http.MethodGet, "/api/system/problems/resolved", "", tok)
	if !strings.Contains(listed.Body.String(), "crash loop") {
		t.Fatalf("listed %s", listed.Body.String())
	}

	auditRR := do(s, http.MethodGet, "/api/audit", "", tok)
	if auditRR.Code != 200 || !strings.Contains(auditRR.Body.String(), "problems/resolve") {
		t.Fatalf("audit %d %s", auditRR.Code, auditRR.Body.String())
	}
}

func TestIntegrationRBACReadonlySSOAndStacks(t *testing.T) {
	s := testServer(t)
	admin := login(t, s, "admin@dockpilot.dev", "secret-pass")

	created := do(s, http.MethodPost, "/api/users", `{"email":"view@dockpilot.dev","name":"View","password":"senha-forte","role":"viewer"}`, admin)
	if created.Code != 200 {
		t.Fatalf("create user %d %s", created.Code, created.Body.String())
	}
	viewer := login(t, s, "view@dockpilot.dev", "senha-forte")
	denied := do(s, http.MethodPost, "/api/containers/abc/start", "", viewer)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("viewer write %d %s", denied.Code, denied.Body.String())
	}
	opDenied := do(s, http.MethodPost, "/api/users", `{"email":"x@dockpilot.dev","name":"X","password":"senha-forte","role":"viewer"}`, viewer)
	if opDenied.Code != http.StatusForbidden {
		t.Fatalf("viewer create user %d %s", opDenied.Code, opDenied.Body.String())
	}

	t.Setenv("DOCKERPILOT_READONLY", "1")
	blocked := do(s, http.MethodPost, "/api/system/problems/resolve", `{"containerId":"z","reason":"x"}`, admin)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("readonly %d %s", blocked.Code, blocked.Body.String())
	}
	t.Setenv("DOCKERPILOT_READONLY", "")

	proj := t.TempDir()
	body := `{"project":"api","projectPath":"` + proj + `","compose":"services:\n  api:\n    image: ghcr.io/acme/api:2.0.0\n","action":"save"}`
	applied := do(s, http.MethodPost, "/api/stacks/apply", body, admin)
	if applied.Code != 200 {
		t.Fatalf("apply %d %s", applied.Code, applied.Body.String())
	}
	var payload struct {
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	if err := json.Unmarshal(applied.Body.Bytes(), &payload); err != nil || payload.Revision.ID == "" {
		t.Fatalf("payload %s %v", applied.Body.String(), err)
	}
	rb := do(s, http.MethodPost, "/api/stacks/rollback", `{"revisionId":"`+payload.Revision.ID+`","projectPath":"`+proj+`"}`, admin)
	if rb.Code != 200 || !strings.Contains(rb.Body.String(), "2.0.0") {
		t.Fatalf("rollback %d %s", rb.Code, rb.Body.String())
	}

	costRR := do(s, http.MethodGet, "/api/insights/cost", "", admin)
	if costRR.Code != 200 || !strings.Contains(costRR.Body.String(), "cpuPerCoreHour") {
		t.Fatalf("cost %d %s", costRR.Code, costRR.Body.String())
	}
	k8s := do(s, http.MethodPost, "/api/proxy/kubernetes", `{"method":"GET","path":"/api/v1/pods"}`, admin)
	if k8s.Code != http.StatusNotImplemented {
		t.Fatalf("k8s %d %s", k8s.Code, k8s.Body.String())
	}
	bad := do(s, http.MethodPost, "/api/proxy/docker", `{"method":"GET","path":"../etc/passwd"}`, admin)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("proxy %d %s", bad.Code, bad.Body.String())
	}

	t.Setenv("DOCKERPILOT_OIDC_ISSUER", "https://login.example.com")
	t.Setenv("DOCKERPILOT_OIDC_CLIENT_ID", "dockpilot")
	t.Setenv("DOCKERPILOT_OIDC_HMAC_SECRET", "sso-secret-for-tests")
	cfg := do(s, http.MethodGet, "/api/auth/sso", "", "")
	if cfg.Code != 200 || !strings.Contains(cfg.Body.String(), `"enabled":true`) {
		t.Fatalf("sso config %d %s", cfg.Code, cfg.Body.String())
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "https://login.example.com", "aud": "dockpilot", "email": "ana@org.dev", "name": "Ana",
		"groups": []string{"dockpilot-operators"}, "exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString([]byte("sso-secret-for-tests"))
	if err != nil {
		t.Fatal(err)
	}
	ex := do(s, http.MethodPost, "/api/auth/sso/exchange", `{"idToken":"`+signed+`"}`, "")
	if ex.Code != 200 || !strings.Contains(ex.Body.String(), "ana@org.dev") {
		t.Fatalf("exchange %d %s", ex.Code, ex.Body.String())
	}

	legacy := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	legacy.AddCookie(&http.Cookie{Name: "dockpanel_token", Value: admin})
	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, legacy)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "admin@dockpilot.dev") {
		t.Fatalf("legacy cookie %d %s", rr.Code, rr.Body.String())
	}
}

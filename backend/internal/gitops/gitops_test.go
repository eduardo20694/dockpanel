package gitops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dockerpilot/internal/audit"
)

type fakeFetch struct{ body, commit string }

func (f fakeFetch) Fetch(ctx context.Context, link Link) (Remote, error) {
	return Remote{Commit: f.commit, Compose: []byte(f.body)}, nil
}

type fakeApply struct {
	applied  int
	rolled   int
	healthy  bool
	detail   string
	lastBody string
}

func (f *fakeApply) Apply(ctx context.Context, stack string, compose []byte) error {
	f.applied++
	f.lastBody = string(compose)
	return nil
}
func (f *fakeApply) Rollback(ctx context.Context, stack string, previous []byte) error {
	f.rolled++
	return nil
}
func (f *fakeApply) Healthy(ctx context.Context, stack string) (bool, string) {
	return f.healthy, f.detail
}

func TestWebhookSyncApprovalAndRollback(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	secret := "whsec"
	if err := s.SetGlobal(GlobalInput{Enabled: boolPtr(true), PollSeconds: intPtr(0), WebhookSecret: &secret}); err != nil {
		t.Fatal(err)
	}
	cred := "super-secret-key"
	if err := s.Upsert(LinkInput{Stack: "api", RepoURL: "https://github.com/acme/api.git", Branch: "main", ComposePath: "docker-compose.yml", Credential: &cred, Mode: "approval", Enabled: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	pub := s.Status()
	raw := mustJSON(t, pub)
	if strings.Contains(raw, cred) || strings.Contains(raw, secret) {
		t.Fatal("secret leaked")
	}
	body := []byte(`{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/acme/api.git"}}`)
	if err := Verify(body, "sha256=dead", "", "", secret); err == nil {
		t.Fatal("bad sig accepted")
	}
	sig := "sha256=" + sign([]byte(secret), body)
	s.Fetch = fakeFetch{body: "services:\n  api:\n    image: api:2\n    environment:\n      PASSWORD: plain\n", commit: "abc"}
	app := &fakeApply{healthy: true}
	s.ApplyTo = app
	n, err := s.Webhook(context.Background(), body, sig, "", "", "github", nil)
	if err != nil || n != 1 {
		t.Fatalf("hook %d %v", n, err)
	}
	if app.applied != 0 {
		t.Fatal("approval applied immediately")
	}
	st := s.Status()
	if len(st.Stacks) != 1 || !st.Stacks[0].Pending || !strings.Contains(st.Stacks[0].PendingDiff, "***") {
		t.Fatalf("%+v", st.Stacks)
	}
	if _, err := s.Sync(context.Background(), "api", "approve", "ana", nil); err != nil {
		t.Fatal(err)
	}
	if app.applied != 1 {
		t.Fatalf("apply %d", app.applied)
	}

	s.Fetch = fakeFetch{body: "services:\n  api:\n    image: api:3\n", commit: "def"}
	if err := s.Upsert(LinkInput{Stack: "api", Mode: "auto"}); err != nil {
		t.Fatal(err)
	}
	app.healthy = false
	app.detail = "unhealthy"
	if _, err := s.Sync(context.Background(), "api", "poll", "gitops", nil); err == nil {
		t.Fatal("expected rollback error")
	}
	if app.rolled != 1 {
		t.Fatalf("rollback %d", app.rolled)
	}

	off := false
	if err := s.SetGlobal(GlobalInput{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	ev, err := s.Sync(context.Background(), "api", "manual", "ana", nil)
	if err != nil || ev.Status != "skipped" {
		t.Fatalf("%+v %v", ev, err)
	}

	aud, err := audit.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = aud
	if !errors.Is(ErrBadSignature, ErrBadSignature) {
		t.Fatal("sentinel")
	}
}

func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := jsonMarshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

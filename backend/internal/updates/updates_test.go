package updates

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type rt func(*http.Request) (*http.Response, error)

func (f rt) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRegistryAndRollback(t *testing.T) {
	ref := Parse("nginx:1.25.0")
	if ref.Host != "docker.io" || ref.Repo != "library/nginx" || ref.Tag != "1.25.0" {
		t.Fatalf("%+v", ref)
	}
	if Parse("ghcr.io/acme/api:1").Host != "ghcr.io" {
		t.Fatal(Parse("ghcr.io/acme/api:1"))
	}
	reg := &HTTPRegistry{Client: &http.Client{Transport: rt(func(r *http.Request) (*http.Response, error) {
		body := "{}"
		h := make(http.Header)
		switch {
		case strings.Contains(r.URL.Path, "token") || strings.Contains(r.URL.RawQuery, "scope="):
			body = `{"token":"t"}`
		case strings.Contains(r.URL.Path, "/manifests/"):
			body = `{}`
			h.Set("Docker-Content-Digest", "sha256:new")
		case strings.Contains(r.URL.Path, "/tags/list"):
			body = `{"tags":["1.25.0","1.26.0"]}`
		default:
			body = `{}`
		}
		h.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	d, err := reg.Latest(context.Background(), "nginx:1.25.0", "", "", "")
	if err != nil || d.Digest != "sha256:new" || d.NewerTag != "1.26.0" {
		t.Fatalf("%+v %v", d, err)
	}

	eng := &fakeEngine{healthy: false, detail: "unhealthy", list: []Running{{Name: "api", Image: "nginx:1.25.0", Digest: "sha256:old"}}}
	dir := t.TempDir()
	svc, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc.Engine = eng
	svc.Registry = reg
	items, err := svc.Check(context.Background())
	if err != nil || len(items) != 1 || !items[0].Outdated {
		t.Fatalf("%+v %v", items, err)
	}
	if eng.pulled != 0 {
		t.Fatal("approval pulled")
	}
	res, err := svc.Apply(context.Background(), "api", "ana", nil)
	if err == nil || res.Status != "rolled_back" || eng.rolled != 1 || eng.pulled != 1 {
		t.Fatalf("%+v %v pulled=%d rolled=%d", res, err, eng.pulled, eng.rolled)
	}
}

type fakeEngine struct {
	list    []Running
	healthy bool
	detail  string
	pulled  int
	rolled  int
}

func (f *fakeEngine) List(ctx context.Context) ([]Running, error)                { return f.list, nil }
func (f *fakeEngine) Pull(ctx context.Context, image string) error               { f.pulled++; return nil }
func (f *fakeEngine) Replace(ctx context.Context, container, image string) error { return nil }
func (f *fakeEngine) Rollback(ctx context.Context, container, previous string) error {
	f.rolled++
	return nil
}
func (f *fakeEngine) Healthy(ctx context.Context, container string) (bool, string) {
	return f.healthy, f.detail
}

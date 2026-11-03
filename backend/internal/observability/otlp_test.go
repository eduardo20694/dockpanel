package observability

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPushMetrics(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		if r.URL.Path != "/v1/metrics" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
	if err := PushMetrics(context.Background(), srv.Client(), "dockpilot_containers", 4); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "dockpilot_containers") || !strings.Contains(got, "4") {
		t.Fatal(got)
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if err := PushMetrics(context.Background(), nil, "x", 1); err != nil {
		t.Fatal(err)
	}
}

package metricsx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrometheusText(t *testing.T) {
	body := Text(Snapshot{Hosts: 2, Containers: 5, Alerts: 1, Deploys: 3})
	for _, name := range []string{"dockpilot_hosts 2", "dockpilot_containers 5", "dockpilot_alerts 1", "dockpilot_deploys 3"} {
		if !strings.Contains(body, name) {
			t.Fatal(body)
		}
	}
	rr := httptest.NewRecorder()
	Handler(Snapshot{Hosts: 1}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "dockpilot_hosts 1") {
		t.Fatal(rr.Body.String())
	}
}

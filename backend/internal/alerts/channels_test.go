package alerts

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestSlackTeamsPagerDutyWebhook(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen[r.URL.Path] = strings.Contains(string(b), "api caiu") || strings.Contains(string(b), "critical")
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()
	n := &Notifier{
		slackWebhook: srv.URL + "/slack", teamsWebhook: srv.URL + "/teams",
		pagerdutyKey: "rk", pagerdutyURL: srv.URL + "/pd", genericWebhook: srv.URL + "/hook",
		http: srv.Client(),
	}
	if err := n.SendSeverity("critical", "api caiu", "detalhe", nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/slack", "/teams", "/pd", "/hook"} {
		if !seen[p] {
			t.Fatalf("missing %s %+v", p, seen)
		}
	}
}

package tgmsg

import (
	"strings"
	"testing"
	"time"
)

func TestMetricsSpacing(t *testing.T) {
	msg := Metrics(2, 1, []MetricItem{
		{Name: "nginx", Host: "Docker", State: "running", CPUPct: 12.5, MemPct: 20, MemHuman: "64.0 MB", Running: true},
		{Name: "old", Host: "Docker", State: "exited", Running: false},
	})
	for _, want := range []string{"<b>Métricas</b>", "nginx", "CPU", "RAM", sep} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in:\n%s", want, msg)
		}
	}
}

func TestDailyAndAlert(t *testing.T) {
	a := Alert("critical", "web", "Docker", "OOM", "exited", 137, 3, "abcdef1234567890")
	if !strings.Contains(a, "Critical") || !strings.Contains(a, "Motivo") {
		t.Fatalf("alert:\n%s", a)
	}
	d := Daily(time.Now(), nil, 5, 4, []MetricItem{{Name: "db", CPUPct: 1, MemHuman: "100 MB", Running: true}})
	if !strings.Contains(d, "Resumo diário") || !strings.Contains(d, "Nenhum problema") {
		t.Fatalf("daily:\n%s", d)
	}
}

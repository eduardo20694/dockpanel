package store

import (
	"testing"
	"time"
)

func TestStoreMetricsAndAlerts(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := st.RecordMetric("vps", "cid1", "web", 12.5, 40, 1024, 0); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordMetric("vps", "cid1", "web", 15.0, 45, 2048, 1); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordRestart("vps", "cid1", "web", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordAlert("vps", "cid1", "web", "critical", "OOM", "mem exceeded"); err != nil {
		t.Fatal(err)
	}

	since := time.Now().Add(-time.Hour)
	history, err := st.MetricsHistory("vps", "cid1", since, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("metrics history len=%d", len(history))
	}

	restarts, err := st.RestartHistory("vps", "cid1", since, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(restarts) != 1 {
		t.Fatalf("restarts len=%d", len(restarts))
	}

	alerts, err := st.RecentAlerts(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Title != "OOM" {
		t.Fatalf("alerts=%+v", alerts)
	}
}

func TestStorePruneOlderThan(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordMetric("vps", "old", "x", 1, 1, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := st.PruneOlderThan(-time.Hour); err != nil {
		t.Fatal(err)
	}
	history, err := st.MetricsHistory("vps", "old", time.Now().Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("expected prune to remove rows, got %d", len(history))
	}
}

func TestDefaultRetention(t *testing.T) {
	t.Setenv("DOCKPANEL_HISTORY_DAYS", "14")
	if got := DefaultRetention(); got != 14*24*time.Hour {
		t.Fatalf("got %v", got)
	}
	t.Setenv("DOCKPANEL_HISTORY_DAYS", "")
	if got := DefaultRetention(); got != 7*24*time.Hour {
		t.Fatalf("default got %v", got)
	}
}

func TestSplitLines(t *testing.T) {
	lines := splitLines([]byte("a\nb\nc"))
	if len(lines) != 3 || string(lines[1]) != "b" {
		t.Fatalf("lines=%v", lines)
	}
}

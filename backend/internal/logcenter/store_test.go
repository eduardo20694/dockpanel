package logcenter

import (
	"testing"
	"time"
)

func TestRetentionPrune(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	old := time.Now().Add(-10 * 24 * time.Hour).UnixMilli()
	recent := time.Now().UnixMilli()
	if _, err := st.Insert(Entry{HostID: "h", ContainerID: "c1", ContainerName: "web", TimestampMs: old, Message: "old line", Severity: "ok"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Insert(Entry{HostID: "h", ContainerID: "c1", ContainerName: "web", TimestampMs: recent, Message: "new line", Severity: "ok"}); err != nil {
		t.Fatal(err)
	}
	n, err := st.PruneOlderThan(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned=%d want 1", n)
	}
	res, err := st.Search(SearchParams{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 1 || res.Entries[0].Message != "new line" {
		t.Fatalf("entries=%+v", res.Entries)
	}
}

func TestFTSSearch(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UnixMilli()
	_, _ = st.Insert(Entry{HostID: "h", ContainerID: "c1", ContainerName: "api", TimestampMs: now, Message: "connection refused to redis", Severity: "warning"})
	_, _ = st.Insert(Entry{HostID: "h", ContainerID: "c1", ContainerName: "api", TimestampMs: now + 1, Message: "all good here", Severity: "ok"})
	res, err := st.Search(SearchParams{Query: "refused", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("want 1 got %d (%+v)", len(res.Entries), res.Entries)
	}
}

func TestDedupeInsert(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ts := time.Now().UnixMilli()
	ok1, err := st.Insert(Entry{HostID: "h", ContainerID: "c1", ContainerName: "x", TimestampMs: ts, Message: "a", Severity: "ok"})
	if err != nil || !ok1 {
		t.Fatalf("first insert ok=%v err=%v", ok1, err)
	}
	ok2, err := st.Insert(Entry{HostID: "h", ContainerID: "c1", ContainerName: "x", TimestampMs: ts, Message: "a", Severity: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if ok2 {
		t.Fatal("expected duplicate skipped")
	}
	res, _ := st.Search(SearchParams{Limit: 10})
	if len(res.Entries) != 1 {
		t.Fatalf("len=%d", len(res.Entries))
	}
}

func TestIncidentsWindow(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	base := time.Now().Add(-time.Hour).UnixMilli()
	_, _ = st.Insert(Entry{HostID: "h", ContainerID: "app1", ContainerName: "api", TimestampMs: base, Message: "fatal crash", Severity: "critical"})
	_, _ = st.Insert(Entry{HostID: "h", ContainerID: "db1", ContainerName: "postgres", TimestampMs: base + 30_000, Message: "error connection", Severity: "critical"})
	_, _ = st.Insert(Entry{HostID: "h", ContainerID: "other", ContainerName: "worker", TimestampMs: base + 10*60_000, Message: "fatal elsewhere", Severity: "critical"})

	inc, err := st.Incidents(base-1000, base+20*60_000, 2*60_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(inc) != 2 {
		t.Fatalf("want 2 incidents got %d: %+v", len(inc), inc)
	}
	if len(inc[0].Containers) < 2 {
		t.Fatalf("first group should merge app+db: %+v", inc[0])
	}
}

func TestRetentionClamp(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetRetentionDays(90); err != nil {
		t.Fatal(err)
	}
	if d := st.GetRetentionDays(); d != 60 {
		t.Fatalf("want 60 got %d", d)
	}
}

package audit

import "testing"

func TestAppendAndList(t *testing.T) {
	l, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Append(Event{Actor: "ana@org.dev", Action: "POST /api/stacks/apply", Status: 200, OrgID: "default"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(Event{Actor: "ana@org.dev", Action: "POST /api/containers/x/restart", Status: 200}); err != nil {
		t.Fatal(err)
	}
	got, err := l.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Action != "POST /api/containers/x/restart" {
		t.Fatalf("%+v", got)
	}
}

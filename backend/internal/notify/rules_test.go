package notify

import "testing"

func TestRules(t *testing.T) {
	rules := []Rule{
		{Event: "deploy", MinSeverity: "warning", Channels: []string{"slack", "pagerduty"}},
		{Event: "alert", MinSeverity: "critical", Channels: []string{"telegram"}},
	}
	ch, limited := Channels(rules, "deploy", "info")
	if !limited || len(ch) != 0 {
		t.Fatalf("info %+v", ch)
	}
	ch, _ = Channels(rules, "deploy", "critical")
	if len(ch) != 2 {
		t.Fatalf("%+v", ch)
	}
	ch, _ = Channels(rules, "alert", "warning")
	if len(ch) != 0 {
		t.Fatalf("alert warning %+v", ch)
	}
	if _, limited := Channels(nil, "deploy", "critical"); limited {
		t.Fatal("empty rules should not limit")
	}
}

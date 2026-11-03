package notify

import "strings"

type Rule struct {
	Event       string   `json:"event"`
	MinSeverity string   `json:"minSeverity"`
	Channels    []string `json:"channels"`
}

var rank = map[string]int{"info": 1, "warning": 2, "critical": 3}

// Channels returns the channels that should receive this event.
// No rules means the caller keeps its default (every configured channel).
func Channels(rules []Rule, event, severity string) (channels []string, limited bool) {
	if len(rules) == 0 {
		return nil, false
	}
	sev := rank[strings.ToLower(severity)]
	if sev == 0 {
		sev = 1
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		if rule.Event != "" && rule.Event != "*" && !strings.EqualFold(rule.Event, event) {
			continue
		}
		min := rank[strings.ToLower(rule.MinSeverity)]
		if min == 0 {
			min = 1
		}
		if sev < min {
			continue
		}
		for _, ch := range rule.Channels {
			ch = strings.ToLower(strings.TrimSpace(ch))
			if ch != "" && !seen[ch] {
				seen[ch] = true
				channels = append(channels, ch)
			}
		}
	}
	return channels, true
}

package logcenter

import (
	"strconv"
	"strings"
	"time"
)

type Incident struct {
	ID          string   `json:"id"`
	StartMs     int64    `json:"startMs"`
	EndMs       int64    `json:"endMs"`
	Containers  []string `json:"containers"`
	Severities  []string `json:"severities"`
	EntryCount  int      `json:"entryCount"`
	SampleLines []string `json:"sampleLines"`
	RelatedHint string   `json:"relatedHint,omitempty"`
}

// Incidents groups warning/critical entries that fall within window of each other.
func (s *Store) Incidents(fromMs, toMs, windowMs int64) ([]Incident, error) {
	if windowMs <= 0 {
		windowMs = int64((2 * time.Minute) / time.Millisecond)
	}
	if toMs <= 0 {
		toMs = time.Now().UnixMilli()
	}
	if fromMs <= 0 {
		fromMs = toMs - int64(24*time.Hour/time.Millisecond)
	}
	entries, err := s.EntriesInRange(fromMs, toMs, []string{"warning", "critical"})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []Incident{}, nil
	}

	var groups []Incident
	var cur *Incident
	containerSet := map[string]struct{}{}
	sevSet := map[string]struct{}{}

	flush := func() {
		if cur == nil {
			return
		}
		cur.Containers = keys(containerSet)
		cur.Severities = keys(sevSet)
		if looksLikeAppAndDB(cur.Containers) {
			cur.RelatedHint = "possível cascata app ↔ banco (nomes sugerem dependência)"
		}
		groups = append(groups, *cur)
		containerSet = map[string]struct{}{}
		sevSet = map[string]struct{}{}
		cur = nil
	}

	for _, e := range entries {
		name := e.ContainerName
		if name == "" {
			name = e.ContainerID
			if len(name) > 12 {
				name = name[:12]
			}
		}
		if cur == nil || e.TimestampMs-cur.EndMs > windowMs {
			flush()
			cur = &Incident{
				ID:          formatIncidentID(e.TimestampMs, e.ID),
				StartMs:     e.TimestampMs,
				EndMs:       e.TimestampMs,
				EntryCount:  1,
				SampleLines: []string{trimSample(name, e.Message)},
			}
			containerSet[name] = struct{}{}
			sevSet[e.Severity] = struct{}{}
			continue
		}
		cur.EndMs = e.TimestampMs
		cur.EntryCount++
		containerSet[name] = struct{}{}
		sevSet[e.Severity] = struct{}{}
		if len(cur.SampleLines) < 5 {
			cur.SampleLines = append(cur.SampleLines, trimSample(name, e.Message))
		}
	}
	flush()
	return groups, nil
}

func formatIncidentID(ts, id int64) string {
	return time.UnixMilli(ts).UTC().Format("20060102T150405") + "-" + strconv.FormatInt(id, 10)
}

func trimSample(name, msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) > 160 {
		msg = msg[:160] + "…"
	}
	return name + ": " + msg
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func looksLikeAppAndDB(names []string) bool {
	dbHints := []string{"mysql", "maria", "postgres", "mongo", "redis", "mssql", "db"}
	hasDB, hasApp := false, false
	for _, n := range names {
		l := strings.ToLower(n)
		isDB := false
		for _, h := range dbHints {
			if strings.Contains(l, h) {
				isDB = true
				break
			}
		}
		if isDB {
			hasDB = true
		} else {
			hasApp = true
		}
	}
	return hasDB && hasApp
}

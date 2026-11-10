// Package audit is an append-only JSONL log of mutating API calls.
package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const fileName = "audit.jsonl"

// Event is one auditable action. Bodies are never stored (they may hold secrets).
type Event struct {
	At     time.Time `json:"at"`
	Actor  string    `json:"actor,omitempty"`
	OrgID  string    `json:"orgId,omitempty"`
	Action string    `json:"action"`
	Status int       `json:"status,omitempty"`
	Host   string    `json:"host,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

type Log struct {
	mu   sync.Mutex
	path string
}

func Open(dir string) (*Log, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Log{path: filepath.Join(dir, fileName)}, nil
}

func (l *Log) Append(ev Event) error {
	if l == nil {
		return nil
	}
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(raw, '\n'))
	return err
}

// List returns the newest events, up to limit (default 100, max 500).
func (l *Log) List(limit int) ([]Event, error) {
	if l == nil {
		return []Event{}, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var all []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		all = append(all, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	// Newest first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if all == nil {
		all = []Event{}
	}
	return all, nil
}

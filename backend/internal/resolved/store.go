// Package resolved persists problems an operator marked as handled.
// The file name matches the 1.0.8 deployment: resolved_problems.json.
package resolved

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const fileName = "resolved_problems.json"

// Item is one dismissed problem. It reappears if the reason changes.
type Item struct {
	Key         string    `json:"key"`
	HostID      string    `json:"hostId"`
	ContainerID string    `json:"containerId"`
	Name        string    `json:"name"`
	Reason      string    `json:"reason"`
	ResolvedAt  time.Time `json:"resolvedAt"`
	By          string    `json:"by,omitempty"`
}

// Problem is the subset of a diagnostic problem needed to filter.
type Problem struct {
	ContainerID string
	Name        string
	Reason      string
}

type Store struct {
	mu    sync.Mutex
	path  string
	items []Item
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, fileName)}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(raw, &s.items); err != nil {
		return nil, err
	}
	return s, nil
}

func Key(host, containerID, reason string) string {
	if host == "" {
		host = "default"
	}
	return host + "|" + containerID + "|" + reason
}

func (s *Store) Mark(item Item) error {
	if item.ResolvedAt.IsZero() {
		item.ResolvedAt = time.Now().UTC()
	}
	if item.HostID == "" {
		item.HostID = "default"
	}
	item.Key = Key(item.HostID, item.ContainerID, item.Reason)
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.items {
		if s.items[i].Key == item.Key {
			s.items[i] = item
			replaced = true
			break
		}
	}
	if !replaced {
		s.items = append(s.items, item)
	}
	return s.flush()
}

func (s *Store) Unmark(host, containerID, reason string) error {
	key := Key(host, containerID, reason)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.items[:0]
	for _, it := range s.items {
		if it.Key != key {
			out = append(out, it)
		}
	}
	s.items = out
	return s.flush()
}

func (s *Store) List(host string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		if host == "" || it.HostID == host {
			out = append(out, it)
		}
	}
	return out
}

// Filter drops problems whose key was marked resolved.
func (s *Store) Filter(host string, problems []Problem) []Problem {
	if s == nil || len(problems) == 0 {
		return problems
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	known := map[string]struct{}{}
	for _, it := range s.items {
		if host == "" || it.HostID == host {
			known[it.Key] = struct{}{}
		}
	}
	out := make([]Problem, 0, len(problems))
	for _, p := range problems {
		if _, ok := known[Key(host, p.ContainerID, p.Reason)]; ok {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

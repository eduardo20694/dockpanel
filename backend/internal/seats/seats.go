// Package seats tracks purchased capacity for an organization.
package seats

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrSeatLimit = errors.New("limite de assentos atingido")

type Subscription struct {
	OrgID          string    `json:"orgId"`
	Plan           string    `json:"plan"`
	Seats          int       `json:"seats"`
	Status         string    `json:"status"`
	Provider       string    `json:"provider,omitempty"`
	CustomerID     string    `json:"customerId,omitempty"`
	SubscriptionID string    `json:"subscriptionId,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Store struct {
	mu   sync.Mutex
	path string
	by   map[string]Subscription
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "subscriptions.json"), by: map[string]Subscription{}}
	raw, err := os.ReadFile(s.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.by); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Get(orgID string) Subscription {
	if orgID == "" {
		orgID = "default"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.by[orgID]; ok {
		return sub
	}
	return Subscription{OrgID: orgID, Plan: "individual", Seats: 1, Status: "active"}
}

func (s *Store) Save(sub Subscription) error {
	if sub.OrgID == "" {
		sub.OrgID = "default"
	}
	if sub.Seats < 1 {
		sub.Seats = 1
	}
	if sub.Plan == "" {
		sub.Plan = "individual"
	}
	if sub.Plan == "individual" {
		sub.Seats = 1
	}
	if sub.Status == "" {
		sub.Status = "active"
	}
	sub.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.by[sub.OrgID] = sub
	return s.flush()
}

// Ensure reports whether another member can be added.
func (s *Store) Ensure(orgID string, currentMembers int) error {
	sub := s.Get(orgID)
	if sub.Status == "canceled" || sub.Status == "past_due" {
		return ErrSeatLimit
	}
	if currentMembers >= sub.Seats {
		return ErrSeatLimit
	}
	return nil
}

func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.by, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

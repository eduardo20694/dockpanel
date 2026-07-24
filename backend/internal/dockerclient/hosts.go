package dockerclient

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type HostConfig struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DockerHost string `json:"dockerHost"`
}

type Pool struct {
	mu        sync.RWMutex
	hosts     []HostConfig
	baseline  map[string]HostConfig // hosts from env
	clis      map[string]*Client
	defaultID string
}

func NewPoolFromEnv() (*Pool, error) {
	raw := os.Getenv("DOCKPANEL_HOSTS")
	if raw != "" {
		var hosts []HostConfig
		if err := json.Unmarshal([]byte(raw), &hosts); err != nil {
			return nil, fmt.Errorf("DOCKPANEL_HOSTS inválido: %w", err)
		}
		if len(hosts) == 0 {
			return nil, fmt.Errorf("DOCKPANEL_HOSTS vazio")
		}
		return NewPool(hosts, hosts[0].ID)
	}

	host := os.Getenv("DOCKER_HOST")
	label := "local"
	if host != "" {
		label = host
	}
	return NewPool([]HostConfig{{
		ID:         "default",
		Label:      label,
		DockerHost: host,
	}}, "default")
}

func NewPool(hosts []HostConfig, defaultID string) (*Pool, error) {
	p := &Pool{
		hosts:     append([]HostConfig(nil), hosts...),
		baseline:  make(map[string]HostConfig, len(hosts)),
		clis:      make(map[string]*Client),
		defaultID: defaultID,
	}
	for _, h := range hosts {
		p.baseline[h.ID] = h
		cli, err := newWithHost(h.DockerHost)
		if err != nil {
			return nil, fmt.Errorf("host %q: %w", h.ID, err)
		}
		p.clis[h.ID] = cli
	}
	return p, nil
}

// Baseline returns hosts configured from environment.
func (p *Pool) Baseline() []HostConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]HostConfig, 0, len(p.baseline))
	for _, h := range p.baseline {
		out = append(out, h)
	}
	return out
}

func (p *Pool) IsBaseline(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if id == "" || id == "default" {
		id = p.defaultID
	}
	_, ok := p.baseline[id]
	return ok
}

func (p *Pool) List() []HostConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]HostConfig, len(p.hosts))
	copy(out, p.hosts)
	return out
}

func (p *Pool) DefaultID() string { return p.defaultID }

func (p *Pool) Get(id string) (*Client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if id == "" || id == "default" {
		id = p.defaultID
	}
	c, ok := p.clis[id]
	if !ok {
		return nil, fmt.Errorf("host desconhecido: %q", id)
	}
	return c, nil
}

// GetIfAllowed returns a client only when id is in allowedIDs (or empty id → first allowed / default if allowed).
func (p *Pool) GetIfAllowed(id string, allowedIDs map[string]bool) (*Client, string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if id == "" || id == "default" {
		if allowedIDs[p.defaultID] {
			id = p.defaultID
		} else {
			for aid := range allowedIDs {
				id = aid
				break
			}
		}
	}
	if id == "" || !allowedIDs[id] {
		return nil, "", fmt.Errorf("host não permitido")
	}
	c, ok := p.clis[id]
	if !ok {
		return nil, "", fmt.Errorf("host desconhecido: %q", id)
	}
	return c, id, nil
}

func HostIDFromRequest(hostHeader, queryHost string) string {
	if h := strings.TrimSpace(hostHeader); h != "" && h != "default" {
		return h
	}
	if h := strings.TrimSpace(queryHost); h != "" && h != "default" {
		return h
	}
	return ""
}

// UpsertHost adds or replaces a host connection in the pool.
func (p *Pool) UpsertHost(h HostConfig) error {
	cli, err := newWithHost(h.DockerHost)
	if err != nil {
		return fmt.Errorf("host %q: %w", h.ID, err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	found := false
	for i := range p.hosts {
		if p.hosts[i].ID == h.ID {
			p.hosts[i] = h
			found = true
			break
		}
	}
	if !found {
		p.hosts = append(p.hosts, h)
	}
	if old, ok := p.clis[h.ID]; ok && old != nil && old.CLI != nil {
		_ = old.CLI.Close()
	}
	p.clis[h.ID] = cli
	if p.defaultID == "" {
		p.defaultID = h.ID
	}
	return nil
}

// RemoveHost removes a non-baseline host from the pool.
func (p *Pool) RemoveHost(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, isBase := p.baseline[id]; isBase {
		return
	}
	if c, ok := p.clis[id]; ok && c != nil && c.CLI != nil {
		_ = c.CLI.Close()
		delete(p.clis, id)
	}
	filtered := p.hosts[:0]
	for _, h := range p.hosts {
		if h.ID != id {
			filtered = append(filtered, h)
		}
	}
	p.hosts = filtered
	if p.defaultID == id && len(p.hosts) > 0 {
		p.defaultID = p.hosts[0].ID
	}
}

// MergeHosts upserts multiple hosts without removing existing ones.
func (p *Pool) MergeHosts(hosts []HostConfig) error {
	for _, h := range hosts {
		if err := p.UpsertHost(h); err != nil {
			return err
		}
	}
	return nil
}

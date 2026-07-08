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
		hosts:     hosts,
		clis:      make(map[string]*Client),
		defaultID: defaultID,
	}
	for _, h := range hosts {
		cli, err := newWithHost(h.DockerHost)
		if err != nil {
			return nil, fmt.Errorf("host %q: %w", h.ID, err)
		}
		p.clis[h.ID] = cli
	}
	return p, nil
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

func HostIDFromRequest(hostHeader, queryHost string) string {
	if h := strings.TrimSpace(hostHeader); h != "" && h != "default" {
		return h
	}
	if h := strings.TrimSpace(queryHost); h != "" && h != "default" {
		return h
	}
	return ""
}

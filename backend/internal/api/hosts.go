package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"dockerpilot/internal/discover"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/kube"

	"github.com/docker/docker/client"
)

func (s *Server) dockerClient(r *http.Request) (*dockerclient.Client, error) {
	id := dockerclient.HostIDFromRequest(hostHeader(r), r.URL.Query().Get("host"))
	if s.Hosts == nil {
		return nil, fmt.Errorf("nenhum host Docker configurado")
	}
	allowed := map[string]bool{}
	for _, h := range s.Hosts.List() {
		allowed[h.ID] = true
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("nenhum host Docker configurado (DOCKERPILOT_HOSTS ou DOCKPANEL_HOSTS)")
	}
	cli, _, err := s.Hosts.GetIfAllowed(id, allowed)
	return cli, err
}

func (s *Server) listHosts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.environmentView(r.Context()))
}

func (s *Server) environmentView(ctx context.Context) map[string]any {
	var hosts []dockerclient.HostConfig
	def := ""
	if s.Hosts != nil {
		hosts = s.Hosts.List()
		def = s.Hosts.DefaultID()
	}
	if def == "" && len(hosts) > 0 {
		def = hosts[0].ID
	}
	type env struct {
		dockerclient.HostConfig
		DetectedKind string   `json:"detectedKind"`
		Role         string   `json:"role,omitempty"`
		Status       string   `json:"status"`
		Detail       string   `json:"detail"`
		Capabilities []string `json:"capabilities"`
		Manual       bool     `json:"manual"`
	}
	out := make([]env, 0, len(hosts)+4)
	seen := map[string]bool{}
	for _, h := range hosts {
		kind := strings.ToLower(strings.TrimSpace(h.Kind))
		if kind == "kubernetes" || kind == "k8s" || h.Kubeconfig != "" {
			continue
		}
		var cli *client.Client
		if s.Hosts != nil {
			if dc, err := s.Hosts.Get(h.ID); err == nil && dc != nil {
				cli = dc.CLI
			}
		}
		view := discover.Classify(h.Kind, discover.ProbeEngine(ctx, cli, h.DockerHost))
		h.Kind = view.Kind
		out = append(out, env{
			HostConfig: h, DetectedKind: view.DetectedKind, Role: view.Role, Status: view.Status,
			Detail: view.Detail, Capabilities: view.Capabilities, Manual: view.Manual,
		})
		seen[h.ID] = true
	}
	if s.Kube != nil {
		for _, c := range s.Kube.List() {
			if seen[c.ID] {
				continue
			}
			status := c.Status
			if status == "" {
				status = "online"
			}
			detail := c.Detail
			if c.Client == nil && detail == "" {
				status = "offline"
				detail = "Cluster sem cliente."
			}
			out = append(out, env{
				HostConfig:   dockerclient.HostConfig{ID: c.ID, Label: c.Label, Kind: "kubernetes"},
				DetectedKind: "kubernetes", Status: status, Detail: detail,
				Capabilities: []string{"workloads", "pods", "services", "eventos"},
			})
		}
	}
	if out == nil {
		out = []env{}
	}
	return map[string]any{"defaultHost": def, "hosts": out}
}

func (s *Server) cluster(r *http.Request) (*kube.Cluster, error) {
	if s.Kube == nil {
		return nil, fmt.Errorf("nenhum cluster Kubernetes configurado")
	}
	id := dockerclient.HostIDFromRequest(hostHeader(r), r.URL.Query().Get("host"))
	return s.Kube.Get(id)
}

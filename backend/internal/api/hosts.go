package api

import (
	"fmt"
	"net/http"

	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/kube"
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
	cfgs := s.Hosts.List()
	def := s.Hosts.DefaultID()
	if def == "" && len(cfgs) > 0 {
		def = cfgs[0].ID
	}
	seen := map[string]bool{}
	for i := range cfgs {
		if cfgs[i].Kind == "" {
			cfgs[i].Kind = "docker"
		}
		seen[cfgs[i].ID] = true
	}
	if s.Kube != nil {
		for _, c := range s.Kube.List() {
			if seen[c.ID] {
				continue
			}
			cfgs = append(cfgs, dockerclient.HostConfig{ID: c.ID, Label: c.Label, Kind: "kubernetes"})
		}
	}
	writeJSON(w, map[string]interface{}{
		"defaultHost": def,
		"hosts":       cfgs,
	})
}

func (s *Server) cluster(r *http.Request) (*kube.Cluster, error) {
	if s.Kube == nil {
		return nil, fmt.Errorf("nenhum cluster Kubernetes configurado")
	}
	id := dockerclient.HostIDFromRequest(hostHeader(r), r.URL.Query().Get("host"))
	return s.Kube.Get(id)
}

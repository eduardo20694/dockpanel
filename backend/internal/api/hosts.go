package api

import (
	"fmt"
	"net/http"

	"dockpanel/internal/dockerclient"
)

func (s *Server) dockerClient(r *http.Request) (*dockerclient.Client, error) {
	id := dockerclient.HostIDFromRequest(r.Header.Get("X-Dockpanel-Host"), r.URL.Query().Get("host"))
	if s.Hosts == nil {
		return nil, fmt.Errorf("nenhum host Docker configurado")
	}
	allowed := map[string]bool{}
	for _, h := range s.Hosts.List() {
		allowed[h.ID] = true
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("nenhum host Docker configurado (DOCKPANEL_HOSTS)")
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
	writeJSON(w, map[string]interface{}{
		"defaultHost": def,
		"hosts":       cfgs,
	})
}

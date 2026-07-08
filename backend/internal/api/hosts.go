package api

import (
	"net/http"

	"dockpanel/internal/dockerclient"
)

func (s *Server) dockerClient(r *http.Request) (*dockerclient.Client, error) {
	id := dockerclient.HostIDFromRequest(r.Header.Get("X-Dockpanel-Host"), r.URL.Query().Get("host"))
	return s.Hosts.Get(id)
}

func (s *Server) listHosts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"defaultHost": s.Hosts.DefaultID(),
		"hosts":       s.Hosts.List(),
	})
}

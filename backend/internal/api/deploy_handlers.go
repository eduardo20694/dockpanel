package api

import (
	"encoding/json"
	"net/http"

	"dockpanel/internal/deploy"
	"dockpanel/internal/dockerclient"
)

func (s *Server) deployPresets(w http.ResponseWriter, r *http.Request) {
	hostID := s.resolveHostID(r)
	dockerHost := deploy.DockerHostLabel()
	for _, h := range s.Hosts.List() {
		if h.ID == hostID {
			if h.DockerHost != "" {
				dockerHost = h.DockerHost
			}
			break
		}
	}
	writeJSON(w, map[string]interface{}{
		"dockerHost": dockerHost,
		"presets":    deploy.Presets(),
	})
}

func (s *Server) deployCompose(w http.ResponseWriter, r *http.Request) {
	var req deploy.ComposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corpo inválido", http.StatusBadRequest)
		return
	}
	if req.DockerHost == "" {
		hostID := dockerclient.HostIDFromRequest(r.Header.Get("X-Dockpanel-Host"), r.URL.Query().Get("host"))
		for _, h := range s.Hosts.List() {
			if h.ID == hostID || (hostID == "" && h.ID == s.Hosts.DefaultID()) {
				req.DockerHost = h.DockerHost
				break
			}
		}
	}

	// up implícito: build + detach quando action=up e flags omitidas
	if req.Action == "" || req.Action == "up" {
		if !req.Build && !req.Detach && r.URL.Query().Get("defaults") != "false" {
			req.Build = true
			req.Detach = true
		}
	}

	res, err := deploy.RunCompose(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  err.Error(),
			"result": res,
		})
		return
	}
	writeJSON(w, res)
}

func (s *Server) deployComposeStatus(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		presets := deploy.Presets()
		if len(presets) > 0 {
			path = presets[0].ProjectPath
		}
	}
	hostID := dockerclient.HostIDFromRequest(r.Header.Get("X-Dockpanel-Host"), r.URL.Query().Get("host"))
	req := deploy.ComposeRequest{ProjectPath: path, Action: "ps"}
	for _, h := range s.Hosts.List() {
		if h.ID == hostID || (hostID == "" && h.ID == s.Hosts.DefaultID()) {
			req.DockerHost = h.DockerHost
			break
		}
	}
	services, res, err := deploy.ListServices(r.Context(), req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{
		"services": services,
		"result":   res,
	})
}

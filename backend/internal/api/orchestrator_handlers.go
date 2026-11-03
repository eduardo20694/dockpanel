package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/envx"
	"dockerpilot/internal/kube"
	"dockerpilot/internal/swarm"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
)

type engine struct {
	cli        *client.Client
	dockerHost string
}

func (s *Server) engine(r *http.Request) (engine, error) {
	dc, err := s.dockerClient(r)
	if err != nil {
		return engine{}, err
	}
	id := dockerclient.HostIDFromRequest(hostHeader(r), r.URL.Query().Get("host"))
	host := ""
	if s.Hosts != nil {
		for _, h := range s.Hosts.List() {
			if h.ID == id || (id == "" && h.ID == s.Hosts.DefaultID()) {
				host = h.DockerHost
			}
		}
	}
	return engine{cli: dc.CLI, dockerHost: host}, nil
}

func rejectReadonly(w http.ResponseWriter) bool {
	if !envx.ReadOnly() {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"modo somente leitura"}`))
	return true
}

func (s *Server) swarmNodes(w http.ResponseWriter, r *http.Request) {
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := swarm.Active(r.Context(), e.cli); err != nil {
		writeErr(w, err)
		return
	}
	list, err := swarm.ListNodes(r.Context(), e.cli)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"nodes": list})
}

func (s *Server) swarmServices(w http.ResponseWriter, r *http.Request) {
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := swarm.ListServices(r.Context(), e.cli)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"services": list})
}

func (s *Server) swarmScale(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Replicas uint64 `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := swarm.Scale(r.Context(), e.cli, chi.URLParam(r, "name"), body.Replicas); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) swarmUpdate(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := swarm.UpdateImage(r.Context(), e.cli, chi.URLParam(r, "name"), body.Image); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) swarmRollback(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := swarm.Rollback(r.Context(), e.cli, chi.URLParam(r, "name")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) swarmTasks(w http.ResponseWriter, r *http.Request) {
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := swarm.Tasks(r.Context(), e.cli, chi.URLParam(r, "name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"tasks": list})
}

func (s *Server) swarmLogs(w http.ResponseWriter, r *http.Request) {
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	text, err := swarm.Logs(r.Context(), e.cli, chi.URLParam(r, "name"), r.URL.Query().Get("tail"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"logs": text})
}

func (s *Server) swarmSecrets(w http.ResponseWriter, r *http.Request) {
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := swarm.ListSecrets(r.Context(), e.cli)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"secrets": list})
}

func (s *Server) swarmSecretCreate(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	id, err := swarm.CreateSecret(r.Context(), e.cli, body.Name, []byte(body.Data))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"id": id})
}

func (s *Server) swarmSecretDelete(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := swarm.RemoveSecret(r.Context(), e.cli, chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) swarmConfigs(w http.ResponseWriter, r *http.Request) {
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := swarm.ListConfigs(r.Context(), e.cli)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"configs": list})
}

func (s *Server) swarmConfigCreate(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	id, err := swarm.CreateConfig(r.Context(), e.cli, body.Name, []byte(body.Data))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"id": id})
}

func (s *Server) swarmConfigDelete(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := swarm.RemoveConfig(r.Context(), e.cli, chi.URLParam(r, "id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) swarmStacks(w http.ResponseWriter, r *http.Request) {
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	list, err := swarm.ListStacks(r.Context(), e.cli)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"stacks": list})
}

func (s *Server) swarmStackDeploy(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name    string `json:"name"`
		Compose string `json:"compose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	out, err := swarm.DeployStack(r.Context(), e.cli, e.dockerHost, body.Name, []byte(body.Compose))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"output": out})
}

func (s *Server) swarmStackRemove(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	e, err := s.engine(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := swarm.RemoveStack(r.Context(), e.cli, e.dockerHost, chi.URLParam(r, "name")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) k8s(w http.ResponseWriter, r *http.Request) (*kube.Cluster, bool) {
	c, err := s.cluster(r)
	if err != nil {
		writeErr(w, err)
		return nil, false
	}
	return c, true
}

func (s *Server) k8sNamespaces(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.Namespaces(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"namespaces": list})
}

func (s *Server) k8sWorkloads(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.Workloads(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"workloads": list})
}

func (s *Server) k8sPods(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.Pods(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"pods": list})
}

func (s *Server) k8sPodLogs(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	tail, _ := strconv.ParseInt(r.URL.Query().Get("tail"), 10, 64)
	text, err := c.PodLogs(r.Context(), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), r.URL.Query().Get("container"), tail)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"logs": text})
}

func (s *Server) k8sPodExec(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	var body struct {
		Command []string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	out, err := c.Exec(r.Context(), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), body.Command)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"output": out})
}

func (s *Server) k8sPodRestart(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	if err := c.RestartPod(r.Context(), chi.URLParam(r, "namespace"), chi.URLParam(r, "name")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) k8sServices(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.Services(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"services": list})
}

func (s *Server) k8sIngresses(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.Ingresses(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"ingresses": list})
}

func (s *Server) k8sConfigMaps(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.ConfigMaps(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"configMaps": list})
}

func (s *Server) k8sSecrets(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.Secrets(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"secrets": list})
}

func (s *Server) k8sScale(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Replicas  int32  `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := c.Scale(r.Context(), body.Kind, body.Namespace, body.Name, body.Replicas); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) k8sRollout(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := c.RolloutRestart(r.Context(), body.Kind, body.Namespace, body.Name); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) k8sUndo(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	var body struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := c.Undo(r.Context(), body.Namespace, body.Name); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) k8sEvents(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.Events(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"events": list})
}

func (s *Server) k8sBroke(w http.ResponseWriter, r *http.Request) {
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	list, err := c.WhatBroke(r.Context(), r.URL.Query().Get("namespace"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"problems": list})
}

func (s *Server) k8sApply(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, `{"error":"corpo inválido"}`, http.StatusBadRequest)
		return
	}
	var wrap struct {
		YAML      string `json:"yaml"`
		Namespace string `json:"namespace"`
	}
	ns := r.URL.Query().Get("namespace")
	if json.Unmarshal(body, &wrap) == nil && wrap.YAML != "" {
		body = []byte(wrap.YAML)
		if wrap.Namespace != "" {
			ns = wrap.Namespace
		}
	}
	applied, err := c.ApplyYAML(r.Context(), body, ns)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"applied": applied})
}

func (s *Server) k8sHelm(w http.ResponseWriter, r *http.Request) {
	if rejectReadonly(w) {
		return
	}
	c, ok := s.k8s(w, r)
	if !ok {
		return
	}
	var body struct {
		Release   string `json:"release"`
		Chart     string `json:"chart"`
		Namespace string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	kubeconfig := ""
	id := dockerclient.HostIDFromRequest(hostHeader(r), r.URL.Query().Get("host"))
	if s.Hosts != nil {
		for _, h := range s.Hosts.List() {
			if h.ID == id {
				kubeconfig = h.Kubeconfig
			}
		}
	}
	out, err := kube.HelmUpgrade(r.Context(), kubeconfig, body.Namespace, body.Release, body.Chart)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = c
	writeJSON(w, map[string]string{"output": out})
}

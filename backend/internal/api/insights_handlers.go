package api

import (
	"context"
	"net/http"
	"time"

	"dockpanel/internal/diagnostics"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/drift"
	"dockpanel/internal/insights"
	"dockpanel/internal/stacks"
	"dockpanel/internal/store"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listStacks(w http.ResponseWriter, r *http.Request) {
	hostID := s.resolveHostID(r)
	cli, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	result, err := stacks.ListStacks(r.Context(), cli.CLI, hostID, s.hostLabel(hostID))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) listStacksAll(w http.ResponseWriter, r *http.Request) {
	var all []stacks.StackHealth
	for _, h := range s.Hosts.List() {
		cli, err := s.Hosts.Get(h.ID)
		if err != nil {
			continue
		}
		result, err := stacks.ListStacks(r.Context(), cli.CLI, h.ID, h.Label)
		if err != nil {
			continue
		}
		all = append(all, result...)
	}
	writeJSON(w, all)
}

func (s *Server) securityAudit(w http.ResponseWriter, r *http.Request) {
	hostID := s.resolveHostID(r)
	cli, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	report, err := insights.Audit(r.Context(), cli.CLI, hostID, s.hostLabel(hostID))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, report)
}

func (s *Server) securityAuditAll(w http.ResponseWriter, r *http.Request) {
	var reports []*insights.SecurityReport
	for _, h := range s.Hosts.List() {
		cli, err := s.Hosts.Get(h.ID)
		if err != nil {
			continue
		}
		rep, err := insights.Audit(r.Context(), cli.CLI, h.ID, h.Label)
		if err != nil {
			continue
		}
		reports = append(reports, rep)
	}
	writeJSON(w, reports)
}

func (s *Server) deepDrift(w http.ResponseWriter, r *http.Request) {
	cli, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "informe ?path=pasta/do/compose", http.StatusBadRequest)
		return
	}
	report, err := drift.DeepCheck(r.Context(), cli.CLI, path)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, report)
}

type InvestigateReport struct {
	HostID         string                 `json:"hostId"`
	HostLabel      string                 `json:"hostLabel"`
	Diagnosis      *diagnostics.Diagnosis `json:"diagnosis"`
	Inspect        map[string]interface{} `json:"inspect"`
	Security       []insights.Finding     `json:"security"`
	MetricHistory  []store.MetricSample   `json:"metricHistory"`
	RestartHistory []store.RestartEvent   `json:"restartHistory"`
}

func (s *Server) investigateContainer(w http.ResponseWriter, r *http.Request) {
	hostID := s.resolveHostID(r)
	cli, err := s.dockerClient(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")

	eng := diagnostics.New(cli.CLI)
	diag, err := eng.Diagnose(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}

	info, err := cli.CLI.ContainerInspect(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}

	secFindings := filterSecurity(r.Context(), cli.CLI, hostID, s.hostLabel(hostID), info.ID)

	networks := make([]string, 0, len(info.NetworkSettings.Networks))
	for n := range info.NetworkSettings.Networks {
		networks = append(networks, n)
	}
	health := "none"
	if info.State.Health != nil {
		health = info.State.Health.Status
	}

	report := InvestigateReport{
		HostID:    hostID,
		HostLabel: s.hostLabel(hostID),
		Diagnosis: diag,
		Inspect: map[string]interface{}{
			"id": info.ID, "name": diag.Name, "image": info.Config.Image,
			"project": info.Config.Labels["com.docker.compose.project"],
			"service": info.Config.Labels["com.docker.compose.service"],
			"networks": networks, "mounts": len(info.Mounts), "envCount": len(info.Config.Env),
			"restartPolicy": info.HostConfig.RestartPolicy.Name, "memLimit": info.HostConfig.Memory,
			"privileged": info.HostConfig.Privileged, "user": info.Config.User,
			"health": health, "ports": info.NetworkSettings.Ports,
			"startedAt": info.State.StartedAt, "finishedAt": info.State.FinishedAt,
		},
		Security: secFindings,
	}

	if s.Store != nil {
		since := time.Now().Add(-24 * time.Hour)
		report.MetricHistory, _ = s.Store.MetricsHistory(hostID, info.ID, since, 120)
		report.RestartHistory, _ = s.Store.RestartHistory(hostID, info.ID, since, 30)
	}

	writeJSON(w, report)
}

func (s *Server) containerHistory(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, map[string]interface{}{"metrics": []any{}, "restarts": []any{}, "alerts": []any{}})
		return
	}
	hostID := s.resolveHostID(r)
	id := chi.URLParam(r, "id")
	since := time.Now().Add(-7 * 24 * time.Hour)
	metrics, _ := s.Store.MetricsHistory(hostID, id, since, 500)
	restarts, _ := s.Store.RestartHistory(hostID, id, since, 100)
	alerts, _ := s.Store.RecentAlerts(50)
	writeJSON(w, map[string]interface{}{
		"metrics": metrics, "restarts": restarts, "alerts": alerts,
	})
}

func filterSecurity(ctx context.Context, cli *client.Client, hostID, hostLabel, containerID string) []insights.Finding {
	rep, err := insights.Audit(ctx, cli, hostID, hostLabel)
	if err != nil {
		return nil
	}
	var out []insights.Finding
	for _, f := range rep.Findings {
		if f.ContainerID == containerID {
			out = append(out, f)
		}
	}
	return out
}

func (s *Server) resolveHostID(r *http.Request) string {
	id := dockerclient.HostIDFromRequest(r.Header.Get("X-Dockpanel-Host"), r.URL.Query().Get("host"))
	if id == "" {
		id = s.Hosts.DefaultID()
	}
	return id
}

func (s *Server) hostLabel(id string) string {
	for _, h := range s.Hosts.List() {
		if h.ID == id {
			return h.Label
		}
	}
	return id
}

func (s *Server) listAlertHistory(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, []any{})
		return
	}
	alerts, err := s.Store.RecentAlerts(50)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, alerts)
}

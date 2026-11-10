package api

import (
	"net/http"
	"strings"
	"time"

	"dockerpilot/internal/diagnostics"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/triage"

	"github.com/docker/docker/api/types/container"
)

func (s *Server) whatBroke(w http.ResponseWriter, r *http.Request) {
	hostID := s.resolveHostID(r)
	if hostID == "" {
		hostID = "default"
	}
	ship, err := triage.LoadShipFromHost(s.dataDir(), hostID)
	if err != nil {
		ship = triage.ShipRecord{}
	}
	sig := triage.Signals{Now: time.Now()}
	s.fillTriageSignals(r, hostID, ship, &sig)
	writeJSON(w, triage.Build(ship, sig))
}

func (s *Server) fillTriageSignals(r *http.Request, hostID string, ship triage.ShipRecord, sig *triage.Signals) {
	if s.LogStore != nil {
		to := time.Now()
		from := to.Add(-6 * time.Hour)
		inc, err := s.LogStore.Incidents(from.UnixMilli(), to.UnixMilli(), 0)
		if err == nil {
			for _, it := range inc {
				sig.Incidents = append(sig.Incidents, triage.IncidentBrief{
					StartMs: it.StartMs, EndMs: it.EndMs, Containers: it.Containers,
					EntryCount: it.EntryCount, SampleLines: it.SampleLines,
				})
			}
		}
	}
	dc, err := s.dockerClient(r)
	if err != nil {
		sig.DockerNote = "Docker indisponível para este host (" + err.Error() + ")."
		return
	}
	eng := diagnostics.New(dc.CLI)
	problems, err := eng.ScanProblems(r.Context())
	if err != nil {
		sig.DockerNote = "Falha ao varrer containers (" + err.Error() + ")."
		return
	}
	if s.Resolved != nil {
		var briefs []resolved.Problem
		for _, p := range problems {
			briefs = append(briefs, resolved.Problem{ContainerID: p.ContainerID, Name: p.Name, Reason: p.Reason})
		}
		kept := map[string]struct{}{}
		for _, p := range s.Resolved.Filter(hostID, briefs) {
			kept[p.ContainerID+"|"+p.Reason] = struct{}{}
		}
		filtered := make([]diagnostics.Problem, 0, len(kept))
		for _, p := range problems {
			if _, ok := kept[p.ContainerID+"|"+p.Reason]; ok {
				filtered = append(filtered, p)
			}
		}
		problems = filtered
	}
	for _, p := range problems {
		sig.Problems = append(sig.Problems, triage.ProblemBrief{
			ContainerID: p.ContainerID, Name: p.Name, Reason: p.Reason, Severity: string(p.Severity),
		})
		if p.Severity == diagnostics.SeverityCritical {
			sig.CriticalCount++
		} else {
			sig.WarningCount++
		}
		if p.OOMKilled {
			sig.OOMKilled = true
		}
		if p.RestartCount > sig.RestartCount {
			sig.RestartCount = p.RestartCount
		}
	}
	list, err := dc.CLI.ContainerList(r.Context(), container.ListOptions{All: true})
	if err != nil {
		return
	}
	for _, c := range list {
		image := c.Image
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		match := ship.Tag != "" && strings.Contains(image, ship.Tag)
		if ship.Project != "" && strings.Contains(strings.ToLower(name), strings.ToLower(ship.Project)) {
			match = true
		}
		if !match && ship.Tag == "" {
			continue
		}
		if !match {
			continue
		}
		info, err := dc.CLI.ContainerInspect(r.Context(), c.ID)
		if err != nil {
			continue
		}
		sig.RunningTag = ship.Tag
		if sig.RunningTag == "" {
			_, sig.RunningTag = splitImageTag(image)
		}
		sig.StartedAt = info.State.StartedAt
		if info.State.Health != nil && info.State.Health.Status != "" {
			sig.Health = info.State.Health.Status
		} else if sig.Health == "" {
			sig.Health = info.State.Status
		}
		if info.RestartCount > sig.RestartCount {
			sig.RestartCount = info.RestartCount
		}
		if info.State.OOMKilled {
			sig.OOMKilled = true
		}
	}
}

func splitImageTag(image string) (string, string) {
	if i := strings.LastIndex(image, ":"); i > 0 && !strings.Contains(image[i+1:], "/") {
		return image, image[i+1:]
	}
	return image, ""
}

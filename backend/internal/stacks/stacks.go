package stacks

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"dockpanel/internal/diagnostics"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ServiceStatus struct {
	ContainerID  string                `json:"containerId"`
	Name         string                `json:"name"`
	Service      string                `json:"service"`
	Image        string                `json:"image"`
	State        string                `json:"state"`
	Severity     diagnostics.Severity  `json:"severity"`
	Reason       string                `json:"reason,omitempty"`
	RestartCount int                   `json:"restartCount"`
	AffectedBy   string                `json:"affectedBy,omitempty"`
}

type StackHealth struct {
	HostID       string          `json:"hostId"`
	HostLabel    string          `json:"hostLabel"`
	Project      string          `json:"project"`
	ComposeFile  string          `json:"composeFile,omitempty"`
	Total        int             `json:"total"`
	Running      int             `json:"running"`
	Critical     int             `json:"critical"`
	Warning      int             `json:"warning"`
	Severity     diagnostics.Severity `json:"severity"`
	Services     []ServiceStatus `json:"services"`
	CascadeNotes []string        `json:"cascadeNotes"`
}

func ListStacks(ctx context.Context, cli *client.Client, hostID, hostLabel string) ([]StackHealth, error) {
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	eng := diagnostics.New(cli)
	problems, _ := eng.ScanProblems(ctx)
	problemByID := map[string]diagnostics.Problem{}
	for _, p := range problems {
		problemByID[p.ContainerID] = p
	}

	byProject := map[string][]types.Container{}
	for _, c := range list {
		project := c.Labels["com.docker.compose.project"]
		if project == "" {
			project = "_standalone"
		}
		byProject[project] = append(byProject[project], c)
	}

	var stacks []StackHealth
	for project, containers := range byProject {
		sh := StackHealth{
			HostID:    hostID,
			HostLabel: hostLabel,
			Project:   project,
			Severity:  diagnostics.SeverityOK,
		}
		if project != "_standalone" && len(containers) > 0 {
			sh.ComposeFile = containers[0].Labels["com.docker.compose.project.config_files"]
		}

		var stoppedDBs []string
		for _, c := range containers {
			name := c.ID[:12]
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			svc := ServiceStatus{
				ContainerID: c.ID,
				Name:        name,
				Service:     c.Labels["com.docker.compose.service"],
				Image:       c.Image,
				State:       c.State,
				Severity:    diagnostics.SeverityOK,
			}
			if p, ok := problemByID[c.ID]; ok {
				svc.Severity = p.Severity
				svc.Reason = p.Reason
				svc.RestartCount = p.RestartCount
			} else if info, err := cli.ContainerInspect(ctx, c.ID); err == nil {
				svc.RestartCount = info.RestartCount
			}

			sh.Total++
			if c.State == "running" {
				sh.Running++
			}
			switch svc.Severity {
			case diagnostics.SeverityCritical:
				sh.Critical++
			case diagnostics.SeverityWarning:
				sh.Warning++
			}
			if svc.State != "running" && looksLikeDB(c.Image, name) {
				stoppedDBs = append(stoppedDBs, name)
			}
			sh.Services = append(sh.Services, svc)
		}

		// cascata: serviços na mesma stack com problema de conectividade podem ser afetados por DB parado
		for i := range sh.Services {
			svc := &sh.Services[i]
			if svc.State == "running" && svc.Severity == diagnostics.SeverityOK && len(stoppedDBs) > 0 {
				for _, db := range stoppedDBs {
					svc.AffectedBy = db
					svc.Severity = diagnostics.SeverityWarning
					svc.Reason = fmt.Sprintf("possível cascata — dependência %s está parada", db)
					sh.Warning++
					sh.CascadeNotes = append(sh.CascadeNotes,
						fmt.Sprintf("%s pode estar afetado porque %s está parado", svc.Name, db))
					break
				}
			}
		}

		if sh.Critical > 0 {
			sh.Severity = diagnostics.SeverityCritical
		} else if sh.Warning > 0 {
			sh.Severity = diagnostics.SeverityWarning
		}

		sort.Slice(sh.Services, func(i, j int) bool {
			ri, rj := severityRank(sh.Services[i].Severity), severityRank(sh.Services[j].Severity)
			if ri != rj {
				return ri < rj
			}
			return sh.Services[i].Name < sh.Services[j].Name
		})
		stacks = append(stacks, sh)
	}

	sort.Slice(stacks, func(i, j int) bool {
		ri, rj := severityRank(stacks[i].Severity), severityRank(stacks[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return stacks[i].Project < stacks[j].Project
	})
	return stacks, nil
}

func severityRank(s diagnostics.Severity) int {
	switch s {
	case diagnostics.SeverityCritical:
		return 0
	case diagnostics.SeverityWarning:
		return 1
	default:
		return 2
	}
}

func looksLikeDB(image, name string) bool {
	s := strings.ToLower(image + " " + name)
	for _, hint := range []string{"mysql", "mariadb", "postgres", "mongo", "redis", "mssql", "db"} {
		if strings.Contains(s, hint) {
			return true
		}
	}
	return false
}

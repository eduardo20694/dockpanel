package insights

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Finding struct {
	ContainerID string `json:"containerId"`
	Name        string `json:"name"`
	Project     string `json:"project,omitempty"`
	Severity    string `json:"severity"` // critical, warning, info
	Category    string `json:"category"`
	Detail      string `json:"detail"`
}

type SecurityReport struct {
	HostID         string    `json:"hostId"`
	HostLabel      string    `json:"hostLabel"`
	CriticalCount  int       `json:"criticalCount"`
	WarningCount   int       `json:"warningCount"`
	LatestTagCount int       `json:"latestTagCount"`
	Findings       []Finding `json:"findings"`
}

func Audit(ctx context.Context, cli *client.Client, hostID, hostLabel string) (*SecurityReport, error) {
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	report := &SecurityReport{HostID: hostID, HostLabel: hostLabel}
	for _, c := range list {
		if c.State != "running" {
			continue
		}
		info, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		project := c.Labels["com.docker.compose.project"]

		add := func(sev, cat, detail string) {
			f := Finding{ContainerID: c.ID, Name: name, Project: project, Severity: sev, Category: cat, Detail: detail}
			report.Findings = append(report.Findings, f)
			switch sev {
			case "critical":
				report.CriticalCount++
			case "warning":
				report.WarningCount++
			}
		}

		if info.HostConfig.Privileged {
			add("critical", "privileged", "container roda em modo privileged — acesso total ao host")
		}
		if info.HostConfig.NetworkMode == "host" {
			add("warning", "host_network", "network_mode=host — compartilha stack de rede do host")
		}
		user := info.Config.User
		if user == "" || user == "0" || user == "root" || strings.HasPrefix(user, "0:") {
			detail := fmt.Sprintf("roda como root (User=%q)", user)
			if user == "" {
				detail = "User não definido — Docker executa como root por padrão"
			}
			add("warning", "root_user", detail)
		}
		if strings.HasSuffix(strings.ToLower(c.Image), ":latest") || !strings.Contains(c.Image, ":") {
			report.LatestTagCount++
			add("warning", "floating_tag", fmt.Sprintf("imagem com tag flutuante: %s", c.Image))
		}
		for _, p := range c.Ports {
			if p.IP == "0.0.0.0" || p.IP == "" {
				if p.PublicPort > 0 {
					add("warning", "exposed_port",
						fmt.Sprintf("porta %d exposta em 0.0.0.0 (acessível externamente)", p.PublicPort))
				}
			}
		}
		if info.State.Health != nil && info.State.Health.Status == "unhealthy" {
			add("critical", "healthcheck", "healthcheck reportando unhealthy")
		} else if info.Config.Healthcheck == nil && info.State.Status == "running" {
			add("info", "no_healthcheck", "sem healthcheck definido — falhas silenciosas são mais difíceis de detectar")
		}
	}
	return report, nil
}

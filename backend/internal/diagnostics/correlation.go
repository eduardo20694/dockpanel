package diagnostics

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

var dbImageHints = []string{"mysql", "mariadb", "postgres", "postgresql", "mongo", "redis", "mssql"}

var connectivityPatterns = []string{
	"connection refused",
	"econnrefused",
	"connect econnrefused",
	"could not connect",
	"unable to connect",
	"connection timed out",
	"enotfound",
	"getaddrinfo",
	"no such host",
}

type RelatedFinding struct {
	ContainerID string `json:"containerId"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Relation    string `json:"relation"`
	Detail      string `json:"detail"`
}

func (e *Engine) correlateDependencies(ctx context.Context, info types.ContainerJSON, errorLines []string) []RelatedFinding {
	if !logsSuggestConnectivityIssue(errorLines) {
		return nil
	}
	networks := info.NetworkSettings.Networks
	if len(networks) == 0 {
		return nil
	}
	networkNames := make(map[string]struct{})
	for n := range networks {
		networkNames[n] = struct{}{}
	}

	list, err := e.CLI.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil
	}

	project := info.Config.Labels["com.docker.compose.project"]
	service := info.Config.Labels["com.docker.compose.service"]

	var related []RelatedFinding
	seen := map[string]bool{info.ID: true}

	for _, c := range list {
		if seen[c.ID] {
			continue
		}
		peerNetworks, err := e.containerNetworkNames(ctx, c.ID)
		if err != nil || !shareNetwork(networkNames, peerNetworks) {
			continue
		}

		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		relation := "mesma rede Docker"
		if project != "" && c.Labels["com.docker.compose.project"] == project {
			relation = fmt.Sprintf("mesmo compose project (%s)", project)
		}

		detail := fmt.Sprintf("imagem %s · status: %s", c.Image, c.Status)
		if c.State != "running" && looksLikeDatabase(c.Image, name) {
			detail = fmt.Sprintf("⚠ dependência provável PARADA — imagem %s · %s", c.Image, c.Status)
		} else if looksLikeDatabase(c.Image, name) {
			detail = fmt.Sprintf("dependência provável rodando — imagem %s", c.Image)
		}

		if service != "" && c.Labels["com.docker.compose.service"] != "" {
			relation += fmt.Sprintf(" · serviço compose: %s", c.Labels["com.docker.compose.service"])
		}

		related = append(related, RelatedFinding{
			ContainerID: c.ID,
			Name:        name,
			State:       c.State,
			Relation:    relation,
			Detail:      detail,
		})
		seen[c.ID] = true
	}

	return related
}

func logsSuggestConnectivityIssue(lines []string) bool {
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, p := range connectivityPatterns {
			if strings.Contains(lower, p) {
				return true
			}
		}
	}
	return false
}

func looksLikeDatabase(image, name string) bool {
	s := strings.ToLower(image + " " + name)
	for _, hint := range dbImageHints {
		if strings.Contains(s, hint) {
			return true
		}
	}
	return false
}

func (e *Engine) containerNetworkNames(ctx context.Context, id string) (map[string]struct{}, error) {
	inf, err := e.CLI.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	for n := range inf.NetworkSettings.Networks {
		out[n] = struct{}{}
	}
	return out, nil
}

func shareNetwork(a, b map[string]struct{}) bool {
	for n := range a {
		if _, ok := b[n]; ok {
			return true
		}
	}
	return false
}

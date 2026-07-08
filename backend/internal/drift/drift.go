package drift

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v3"
)

type Item struct {
	Service       string `json:"service"`
	ComposeImage  string `json:"composeImage"`
	RunningImage  string `json:"runningImage"`
	ContainerName string `json:"containerName"`
	ContainerID   string `json:"containerId"`
	State         string `json:"state"`
	Drift         bool   `json:"drift"`
	Detail        string `json:"detail"`
}

type Report struct {
	ProjectPath string `json:"projectPath"`
	ProjectName string `json:"projectName"`
	Items       []Item `json:"items"`
	DriftCount  int    `json:"driftCount"`
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image string `yaml:"image"`
}

func Check(ctx context.Context, cli *client.Client, projectPath string) (*Report, error) {
	path, composeName, services, projectName, err := loadCompose(projectPath)
	if err != nil {
		return nil, err
	}

	list, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	report := &Report{
		ProjectPath: path,
		ProjectName: projectName,
	}

	for svc, spec := range services {
		if spec.Image == "" {
			continue
		}
		want := normalizeImage(spec.Image)
		var match *types.Container
		for i := range list {
			c := list[i]
			if c.Labels["com.docker.compose.project"] == projectName &&
				c.Labels["com.docker.compose.service"] == svc {
				match = &c
				break
			}
		}
		item := Item{
			Service:      svc,
			ComposeImage: spec.Image,
		}
		if match == nil {
			item.Drift = true
			item.Detail = "serviço definido no compose mas sem container correspondente rodando/parado"
			report.DriftCount++
			report.Items = append(report.Items, item)
			continue
		}
		name := match.ID[:12]
		if len(match.Names) > 0 {
			name = strings.TrimPrefix(match.Names[0], "/")
		}
		got := normalizeImage(match.Image)
		item.RunningImage = match.Image
		item.ContainerName = name
		item.ContainerID = match.ID
		item.State = match.State
		if want != got && !imageCompatible(want, got) {
			item.Drift = true
			item.Detail = fmt.Sprintf("imagem divergente — compose pede %s, container usa %s", spec.Image, match.Image)
			report.DriftCount++
		} else {
			item.Detail = "ok — imagem alinhada com compose"
		}
		report.Items = append(report.Items, item)
	}

	_ = composeName
	return report, nil
}

func loadCompose(projectPath string) (absPath, composeFileName string, services map[string]composeService, projectName string, err error) {
	absPath, err = filepath.Abs(projectPath)
	if err != nil {
		return
	}
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		p := filepath.Join(absPath, name)
		if _, statErr := os.Stat(p); statErr == nil {
			composeFileName = name
			data, readErr := os.ReadFile(p)
			if readErr != nil {
				err = readErr
				return
			}
			var cf composeFile
			if err = yaml.Unmarshal(data, &cf); err != nil {
				return
			}
			services = cf.Services
			projectName = filepath.Base(absPath)
			return
		}
	}
	err = fmt.Errorf("nenhum compose encontrado em %s", absPath)
	return
}

func normalizeImage(img string) string {
	img = strings.TrimSpace(img)
	if idx := strings.Index(img, "@sha256:"); idx > 0 {
		img = img[:idx]
	}
	if !strings.Contains(img, ":") {
		img += ":latest"
	}
	return strings.ToLower(img)
}

func imageCompatible(want, got string) bool {
	want = normalizeImage(want)
	got = normalizeImage(got)
	if want == got {
		return true
	}
	// redecoop/redecoop:1.0.3 vs redecoop/redecoop:1.0.3 (same tag different digest display)
	wantBase, _, _ := strings.Cut(want, "@")
	gotBase, _, _ := strings.Cut(got, "@")
	return wantBase == gotBase
}

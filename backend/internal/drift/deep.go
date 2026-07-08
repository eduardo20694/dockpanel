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
	"github.com/docker/go-connections/nat"
	"gopkg.in/yaml.v3"
)

type DeepItem struct {
	Item
	Kind           string   `json:"kind"` // image, missing, orphan, env, port
	ComposePorts   []string `json:"composePorts,omitempty"`
	RunningPorts   []string `json:"runningPorts,omitempty"`
	MissingEnv     []string `json:"missingEnv,omitempty"`
	ExtraEnv       []string `json:"extraEnv,omitempty"`
}

type DeepReport struct {
	Report
	Orphans    []DeepItem `json:"orphans"`
	DeepItems  []DeepItem `json:"deepItems"`
	TotalDrift int        `json:"totalDrift"`
}

type composeServiceDeep struct {
	Image string            `yaml:"image"`
	Build interface{}       `yaml:"build"`
	Ports []interface{}     `yaml:"ports"`
	Environment []interface{} `yaml:"environment"`
}

type composeFileDeep struct {
	Services map[string]composeServiceDeep `yaml:"services"`
}

func DeepCheck(ctx context.Context, cli *client.Client, projectPath string) (*DeepReport, error) {
	base, err := Check(ctx, cli, projectPath)
	if err != nil {
		return nil, err
	}
	deep := &DeepReport{Report: *base}
	for _, it := range base.Items {
		deep.DeepItems = append(deep.DeepItems, DeepItem{Item: it, Kind: "image"})
	}
	deep.TotalDrift = base.DriftCount

	absPath, _, services, projectName, err := loadComposeDeep(projectPath)
	if err != nil {
		return deep, nil
	}
	_ = absPath

	list, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return deep, nil
	}

	matchedIDs := map[string]bool{}
	for _, c := range list {
		if c.Labels["com.docker.compose.project"] == projectName {
			matchedIDs[c.ID] = true
		}
	}

	// órfãos: containers do project no host mas serviço removido do compose
	composeServices := map[string]bool{}
	for svc := range services {
		composeServices[svc] = true
	}
	for _, c := range list {
		if c.Labels["com.docker.compose.project"] != projectName {
			continue
		}
		svc := c.Labels["com.docker.compose.service"]
		if composeServices[svc] {
			continue
		}
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		orphan := DeepItem{
			Item: Item{
				Service: svc, RunningImage: c.Image, ContainerName: name,
				ContainerID: c.ID, State: c.State, Drift: true,
				Detail: "container órfão — existe no host mas não está no compose atual",
			},
			Kind: "orphan",
		}
		deep.Orphans = append(deep.Orphans, orphan)
		deep.TotalDrift++
	}

	// env e ports por serviço
	for svc, spec := range services {
		var match *types.Container
		for i := range list {
			c := list[i]
			if c.Labels["com.docker.compose.project"] == projectName &&
				c.Labels["com.docker.compose.service"] == svc {
				match = &c
				break
			}
		}
		if match == nil {
			continue
		}
		info, err := cli.ContainerInspect(ctx, match.ID)
		if err != nil {
			continue
		}

		wantPorts := formatComposePorts(spec.Ports)
		gotPorts := formatRunningPorts(info.NetworkSettings.Ports)
		if !portsMatch(wantPorts, gotPorts) && len(wantPorts) > 0 {
			for i := range deep.DeepItems {
				if deep.DeepItems[i].Service == svc {
					deep.DeepItems[i].Kind = "port"
					deep.DeepItems[i].Drift = true
					deep.DeepItems[i].ComposePorts = wantPorts
					deep.DeepItems[i].RunningPorts = gotPorts
					deep.DeepItems[i].Detail = "mapeamento de portas diverge do compose"
					deep.TotalDrift++
					break
				}
			}
		}

		wantEnv := parseComposeEnv(spec.Environment)
		gotEnv := info.Config.Env
		missing, extra := envDiff(wantEnv, gotEnv)
		if len(missing) > 0 || len(extra) > 0 {
			for i := range deep.DeepItems {
				if deep.DeepItems[i].Service == svc {
					if deep.DeepItems[i].Kind == "image" {
						deep.DeepItems[i].Kind = "env"
					}
					if len(missing) > 0 || len(extra) > 0 {
						deep.DeepItems[i].Drift = true
						deep.DeepItems[i].MissingEnv = missing
						deep.DeepItems[i].ExtraEnv = extra
						if deep.DeepItems[i].Detail == "" || deep.DeepItems[i].Detail == "ok — imagem alinhada com compose" {
							deep.DeepItems[i].Detail = "variáveis de ambiente divergem do compose"
						}
						deep.TotalDrift++
					}
					break
				}
			}
		}

		if spec.Image == "" && spec.Build != nil {
			for i := range deep.DeepItems {
				if deep.DeepItems[i].Service == svc && !deep.DeepItems[i].Drift {
					deep.DeepItems[i].Detail = "serviço usa build: — compare imagem rodando manualmente"
				}
			}
		}
	}

	deep.DriftCount = deep.TotalDrift
	return deep, nil
}

func loadComposeDeep(projectPath string) (absPath, composeFileName string, services map[string]composeServiceDeep, projectName string, err error) {
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
			var cf composeFileDeep
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

func formatComposePorts(ports []interface{}) []string {
	var out []string
	for _, p := range ports {
		out = append(out, fmt.Sprint(p))
	}
	return out
}

func formatRunningPorts(ports nat.PortMap) []string {
	var out []string
	for priv, bindings := range ports {
		for _, b := range bindings {
			out = append(out, fmt.Sprintf("%s:%s->%s", b.HostIP, b.HostPort, priv))
		}
	}
	return out
}

func portsMatch(want, got []string) bool {
	if len(want) == 0 {
		return true
	}
	// comparação simplificada: se nenhuma porta pública mapeada quando compose pede
	if len(got) == 0 && len(want) > 0 {
		return false
	}
	return len(want) == len(got)
}

func parseComposeEnv(env []interface{}) map[string]string {
	m := map[string]string{}
	for _, e := range env {
		switch v := e.(type) {
		case string:
			k, val, ok := strings.Cut(v, "=")
			if ok {
				m[k] = val
			}
		}
	}
	return m
}

func envDiff(want map[string]string, got []string) (missing, extra []string) {
	gotMap := map[string]string{}
	for _, e := range got {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			gotMap[k] = v
		}
	}
	for k, v := range want {
		if gv, ok := gotMap[k]; !ok {
			missing = append(missing, k+"="+v)
		} else if gv != v {
			missing = append(missing, k+"="+v+" (compose) vs "+k+"="+gv+" (rodando)")
		}
	}
	for k := range gotMap {
		if _, ok := want[k]; !ok && strings.HasPrefix(k, "com.docker.") == false {
			extra = append(extra, k+"="+gotMap[k])
		}
	}
	return
}

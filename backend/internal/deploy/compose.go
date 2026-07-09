package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const composeTimeout = 15 * time.Minute

var composeFileNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

type ComposeRequest struct {
	ProjectPath string `json:"projectPath"`
	Action      string `json:"action"`
	Build       bool   `json:"build"`
	Detach      bool   `json:"detach"`
	DockerHost  string `json:"dockerHost"`
	Tail        int    `json:"tail"` // logs: linhas (padrão 100)
}

type ServiceStatus struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Ports   string `json:"ports"`
	Image   string `json:"image"`
	Health  string `json:"health,omitempty"`
}

type ComposeResult struct {
	OK       bool   `json:"ok"`
	Action   string `json:"action"`
	Path     string `json:"path"`
	Compose  string `json:"composeFile"`
	Output   string `json:"output"`
	Duration string `json:"duration"`
}

type Preset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectPath string `json:"projectPath"`
	ComposeFile string `json:"composeFile"`
}

// Presets devolve projetos compose conhecidos (path via env ou padrão).
func Presets() []Preset {
	path := os.Getenv("DOCKPANEL_COMPOSE_PATH")
	if path == "" {
		path = `c:\Github\dockpanel`
	}
	compose, _ := findComposeFile(path)
	if compose == "" {
		return nil
	}
	return []Preset{{
		ID:          "dockpanel",
		Name:        "dockpanel",
		ProjectPath: path,
		ComposeFile: compose,
	}}
}

func findComposeFile(dir string) (string, error) {
	for _, name := range composeFileNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("nenhum docker-compose.yml/compose.yml em %s", dir)
}

func RunCompose(ctx context.Context, req ComposeRequest) (*ComposeResult, error) {
	if IsSSHHost(req.DockerHost) {
		remotePath := ResolveComposePath(req.DockerHost, req.ProjectPath)
		return runComposeSSH(ctx, req.DockerHost, remotePath, req)
	}

	if req.ProjectPath == "" {
		return nil, fmt.Errorf("informe projectPath (pasta com docker-compose.yml)")
	}

	path, err := filepath.Abs(req.ProjectPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("caminho inválido: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s não é uma pasta", path)
	}

	composeName, err := findComposeFile(path)
	if err != nil {
		return nil, err
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "up"
	}

	args := []string{"compose", "-f", filepath.Join(path, composeName)}

	switch action {
	case "up":
		args = append(args, "up")
		if req.Build {
			args = append(args, "--build")
		}
		if req.Detach {
			args = append(args, "-d")
		}
	case "down":
		args = append(args, "down")
	case "build":
		args = append(args, "build")
	case "ps":
		args = append(args, "ps")
	case "pull":
		args = append(args, "pull")
	case "restart":
		args = append(args, "restart")
	case "logs":
		args = append(args, "logs", "--no-color")
		tail := req.Tail
		if tail <= 0 {
			tail = 100
		}
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	default:
		return nil, fmt.Errorf("ação inválida %q — use up, down, build, ps, pull, restart ou logs", action)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, composeTimeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = path
	env := os.Environ()
	if req.DockerHost != "" {
		env = append(env, "DOCKER_HOST="+req.DockerHost)
	}
	cmd.Env = env

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	runErr := cmd.Run()
	output := strings.TrimSpace(out.String())
	if len(output) > 64*1024 {
		output = output[len(output)-64*1024:]
		output = "…(truncado)\n" + output
	}

	res := &ComposeResult{
		OK:       runErr == nil,
		Action:   action,
		Path:     path,
		Compose:  composeName,
		Output:   output,
		Duration: time.Since(start).Round(time.Millisecond).String(),
	}
	if runErr != nil {
		if output == "" {
			output = runErr.Error()
			res.Output = output
		}
		return res, fmt.Errorf("docker compose %s falhou: %s", action, output)
	}
	return res, nil
}

func DockerHostLabel() string {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		return "local (socket/pipe padrão)"
	}
	return host
}

type psJSONRow struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Ports   string `json:"Ports"`
	Image   string `json:"Image"`
	Health  string `json:"Health"`
}

// ListServices roda `docker compose ps --format json` e devolve linhas parseadas.
func ListServices(ctx context.Context, req ComposeRequest) ([]ServiceStatus, *ComposeResult, error) {
	if req.ProjectPath == "" {
		return nil, nil, fmt.Errorf("informe projectPath")
	}
	path, err := filepath.Abs(req.ProjectPath)
	if err != nil {
		return nil, nil, err
	}
	composeName, err := findComposeFile(path)
	if err != nil {
		return nil, nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	start := time.Now()
	args := []string{"compose", "-f", filepath.Join(path, composeName), "ps", "--format", "json"}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = path
	env := os.Environ()
	if req.DockerHost != "" {
		env = append(env, "DOCKER_HOST="+req.DockerHost)
	}
	cmd.Env = env

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	output := strings.TrimSpace(out.String())

	res := &ComposeResult{
		OK:       runErr == nil,
		Action:   "ps",
		Path:     path,
		Compose:  composeName,
		Output:   output,
		Duration: time.Since(start).Round(time.Millisecond).String(),
	}
	services, parseErr := ParsePSJSON(output)
	if parseErr != nil && output != "" {
		res.Output = output + "\n(parse: " + parseErr.Error() + ")"
	}
	if runErr != nil {
		if res.Output == "" {
			res.Output = runErr.Error()
		}
		return services, res, fmt.Errorf("docker compose ps falhou: %s", res.Output)
	}
	return services, res, nil
}

func ParsePSJSON(output string) ([]ServiceStatus, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, nil
	}
	var services []ServiceStatus
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row psJSONRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("linha ps inválida: %w", err)
		}
		services = append(services, ServiceStatus{
			Name:    row.Name,
			Service: row.Service,
			State:   row.State,
			Status:  row.Status,
			Ports:   row.Ports,
			Image:   row.Image,
			Health:  row.Health,
		})
	}
	return services, nil
}

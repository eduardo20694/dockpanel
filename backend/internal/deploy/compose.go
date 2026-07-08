package deploy

import (
	"bytes"
	"context"
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
		Name:        "dockpanel (backend + frontend)",
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
	default:
		return nil, fmt.Errorf("ação inválida %q — use up, down, build, ps ou pull", action)
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

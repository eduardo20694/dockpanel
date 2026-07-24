package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// IsSSHHost indica conexão remota via ssh://user@host.
func IsSSHHost(dockerHost string) bool {
	return strings.HasPrefix(dockerHost, "ssh://")
}

// RemoteComposePath pasta do compose em host SSH (MCP / deploy remoto).
// Vazio se não configurado — o chamador deve informar project_path.
func RemoteComposePath() string {
	return strings.TrimSpace(os.Getenv("DOCKPANEL_COMPOSE_PATH_REMOTE"))
}

// ResolveComposePath escolhe pasta local (drift) ou remota (deploy SSH).
func ResolveComposePath(dockerHost, projectPath string) string {
	projectPath = strings.TrimSpace(projectPath)
	if IsSSHHost(dockerHost) {
		if projectPath != "" && !isWindowsAbsPath(projectPath) {
			return projectPath
		}
		return RemoteComposePath()
	}
	if projectPath != "" {
		return projectPath
	}
	return DefaultComposePath()
}

func isWindowsAbsPath(p string) bool {
	if len(p) < 2 {
		return false
	}
	return p[1] == ':' && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z'))
}

func sshTarget(dockerHost string) string {
	return strings.TrimPrefix(dockerHost, "ssh://")
}

func buildComposeCommand(action string, req ComposeRequest) (string, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "up"
	}
	parts := []string{"docker", "compose"}
	switch action {
	case "up":
		parts = append(parts, "up")
		if req.Build {
			parts = append(parts, "--build")
		}
		if req.Detach {
			parts = append(parts, "-d")
		}
	case "down":
		parts = append(parts, "down")
	case "build":
		parts = append(parts, "build")
	case "ps":
		parts = append(parts, "ps")
	case "pull":
		parts = append(parts, "pull")
	case "restart":
		parts = append(parts, "restart")
	case "logs":
		parts = append(parts, "logs", "--no-color")
		tail := req.Tail
		if tail <= 0 {
			tail = 100
		}
		parts = append(parts, "--tail", fmt.Sprintf("%d", tail))
	default:
		return "", fmt.Errorf("ação inválida %q — use up, down, build, ps, pull, restart ou logs", action)
	}
	return strings.Join(parts, " "), nil
}

func runComposeSSH(ctx context.Context, dockerHost, remoteDir string, req ComposeRequest) (*ComposeResult, error) {
	remoteDir = strings.TrimSpace(remoteDir)
	if remoteDir == "" {
		return nil, fmt.Errorf("pasta remota vazia")
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "up"
	}
	composeCmd, err := buildComposeCommand(action, req)
	if err != nil {
		return nil, err
	}
	remoteScript := fmt.Sprintf("cd %s && %s", shellQuote(remoteDir), composeCmd)

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, composeTimeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "ssh", sshTarget(dockerHost), remoteScript)
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
		Path:     remoteDir,
		Compose:  "docker-compose.yml (remoto)",
		Output:   output,
		Duration: time.Since(start).Round(time.Millisecond).String(),
	}
	if runErr != nil {
		if output == "" {
			output = runErr.Error()
			res.Output = output
		}
		return res, fmt.Errorf("ssh compose %s falhou: %s", action, output)
	}
	return res, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

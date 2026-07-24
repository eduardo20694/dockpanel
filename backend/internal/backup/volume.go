package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dockpanel/internal/deploy"
)

type Result struct {
	VolumeName string `json:"volumeName"`
	BackupPath string `json:"backupPath"`
	SizeBytes  int64  `json:"sizeBytes"`
	Duration   string `json:"duration"`
}

func BackupVolume(ctx context.Context, volumeName, destDir, dockerHost string) (*Result, error) {
	if volumeName == "" {
		return nil, fmt.Errorf("nome do volume vazio")
	}
	if deploy.IsSSHHost(dockerHost) {
		return backupVolumeSSH(ctx, dockerHost, volumeName)
	}
	if destDir == "" {
		if p := strings.TrimSpace(os.Getenv("DOCKPANEL_BACKUP_DIR")); p != "" {
			destDir = p
		} else {
			destDir = filepath.Join(os.TempDir(), "dockwatch-backups")
		}
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}

	ts := time.Now().Format("20060102-150405")
	safeName := strings.ReplaceAll(volumeName, "/", "_")
	fileName := fmt.Sprintf("%s-%s.tar.gz", safeName, ts)
	outPath := filepath.Join(destDir, fileName)

	start := time.Now()
	args := []string{
		"run", "--rm",
		"-v", volumeName + ":/volume:ro",
		"-v", destDir + ":/backup",
		"alpine:3.20",
		"sh", "-c", "tar czf /backup/" + fileName + " -C /volume .",
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = dockerEnv(dockerHost)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("backup falhou: %s — %s", err, strings.TrimSpace(string(out)))
	}

	info, _ := os.Stat(outPath)
	var size int64
	if info != nil {
		size = info.Size()
	}

	return &Result{
		VolumeName: volumeName,
		BackupPath: outPath,
		SizeBytes:  size,
		Duration:   time.Since(start).Round(time.Millisecond).String(),
	}, nil
}

func backupVolumeSSH(ctx context.Context, dockerHost, volumeName string) (*Result, error) {
	destDir := strings.TrimSpace(os.Getenv("DOCKPANEL_BACKUP_DIR"))
	if destDir == "" {
		destDir = "/var/lib/dockwatch/backups"
	}
	ts := time.Now().Format("20060102-150405")
	safeName := strings.ReplaceAll(volumeName, "/", "_")
	fileName := fmt.Sprintf("%s-%s.tar.gz", safeName, ts)
	remotePath := destDir + "/" + fileName

	start := time.Now()
	script := fmt.Sprintf(
		"mkdir -p %s && docker run --rm -v %s:/volume:ro -v %s:/backup alpine:3.20 sh -c 'tar czf /backup/%s -C /volume .'",
		destDir, volumeName, destDir, fileName,
	)
	cmd := exec.CommandContext(ctx, "ssh", strings.TrimPrefix(dockerHost, "ssh://"), script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("backup remoto falhou: %s — %s", err, strings.TrimSpace(string(out)))
	}

	return &Result{
		VolumeName: volumeName,
		BackupPath: remotePath + " (remoto)",
		Duration:   time.Since(start).Round(time.Millisecond).String(),
	}, nil
}

func dockerEnv(dockerHost string) []string {
	env := os.Environ()
	if dockerHost != "" {
		env = append(env, "DOCKER_HOST="+dockerHost)
	}
	return env
}

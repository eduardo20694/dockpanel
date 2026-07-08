package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	VolumeName string `json:"volumeName"`
	BackupPath string `json:"backupPath"`
	SizeBytes  int64  `json:"sizeBytes"`
	Duration   string `json:"duration"`
}

func BackupVolume(ctx context.Context, volumeName, destDir string) (*Result, error) {
	if volumeName == "" {
		return nil, fmt.Errorf("nome do volume vazio")
	}
	if destDir == "" {
		destDir = filepath.Join(os.TempDir(), "dockpanel-backups")
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}

	ts := time.Now().Format("20060102-150405")
	safeName := strings.ReplaceAll(volumeName, "/", "_")
	fileName := fmt.Sprintf("%s-%s.tar.gz", safeName, ts)
	outPath := filepath.Join(destDir, fileName)

	start := time.Now()
	// alpine cria tarball do conteúdo do volume antes de remover
	args := []string{
		"run", "--rm",
		"-v", volumeName + ":/volume:ro",
		"-v", destDir + ":/backup",
		"alpine:3.20",
		"sh", "-c", "tar czf /backup/" + fileName + " -C /volume .",
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = os.Environ()
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

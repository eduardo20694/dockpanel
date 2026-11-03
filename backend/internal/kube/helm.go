package kube

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// HelmUpgrade instala ou atualiza um chart com o binário helm e o kubeconfig do cluster.
func HelmUpgrade(ctx context.Context, kubeconfig, namespace, release, chart string) (string, error) {
	if _, err := exec.LookPath("helm"); err != nil {
		return "", fmt.Errorf("helm não está instalado")
	}
	if strings.TrimSpace(release) == "" || strings.TrimSpace(chart) == "" {
		return "", fmt.Errorf("release e chart são obrigatórios")
	}
	if namespace == "" {
		namespace = "default"
	}
	cmd := exec.CommandContext(ctx, "helm", "upgrade", "--install", release, chart, "--namespace", namespace, "--create-namespace")
	cmd.Env = os.Environ()
	if kubeconfig != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+kubeconfig)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("helm: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

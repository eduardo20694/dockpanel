package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockerpilot/internal/deploy"
)

type composeApplier struct {
	root string
}

func (a composeApplier) dir(stack string) string {
	return filepath.Join(a.root, "gitops-work", stack)
}

func (a composeApplier) write(stack string, compose []byte) (string, error) {
	dir := a.dir(stack)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), compose, 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

func (a composeApplier) Apply(ctx context.Context, stack string, compose []byte) error {
	dir, err := a.write(stack, compose)
	if err != nil {
		return err
	}
	res, err := deploy.RunCompose(ctx, deploy.ComposeRequest{ProjectPath: dir, Action: "up", Detach: true})
	if err != nil {
		return err
	}
	if res != nil && !res.OK {
		return fmt.Errorf("%s", res.Output)
	}
	return nil
}

func (a composeApplier) Rollback(ctx context.Context, stack string, previous []byte) error {
	return a.Apply(ctx, stack, previous)
}

func (a composeApplier) Healthy(ctx context.Context, stack string) (bool, string) {
	dir := a.dir(stack)
	services, _, err := deploy.ListServices(ctx, deploy.ComposeRequest{ProjectPath: dir})
	if err != nil {
		return false, err.Error()
	}
	if len(services) == 0 {
		return false, "nenhum serviço no ar"
	}
	for _, svc := range services {
		h := strings.ToLower(svc.Health)
		if strings.Contains(h, "unhealthy") || strings.Contains(strings.ToLower(svc.State), "exit") {
			return false, svc.Name + " " + svc.Health + " " + svc.State
		}
	}
	return true, "ok"
}

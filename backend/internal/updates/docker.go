package updates

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type DockerEngine struct {
	CLI *client.Client
}

func NewDockerEngine(cli *client.Client) *DockerEngine {
	if cli == nil {
		return nil
	}
	return &DockerEngine{CLI: cli}
}

func (e *DockerEngine) List(ctx context.Context) ([]Running, error) {
	list, err := e.CLI.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	var out []Running
	for _, c := range list {
		name := c.ID
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		digest := ""
		if img, _, err := e.CLI.ImageInspectWithRaw(ctx, c.ImageID); err == nil {
			for _, d := range img.RepoDigests {
				if i := strings.LastIndex(d, "@"); i >= 0 {
					digest = d[i+1:]
					break
				}
			}
		}
		out = append(out, Running{Name: name, Image: c.Image, Digest: digest})
	}
	return out, nil
}

func (e *DockerEngine) Pull(ctx context.Context, ref string) error {
	rc, err := e.CLI.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(io.Discard, rc)
	return err
}

func (e *DockerEngine) Replace(ctx context.Context, name, ref string) error {
	info, err := e.CLI.ContainerInspect(ctx, name)
	if err != nil {
		return err
	}
	prev := strings.TrimPrefix(info.Name, "/") + "-dp-prev"
	_ = e.CLI.ContainerRemove(ctx, prev, container.RemoveOptions{Force: true})
	timeout := 15
	_ = e.CLI.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout})
	if err := e.CLI.ContainerRename(ctx, name, prev); err != nil {
		return err
	}
	cfg := info.Config
	cfg.Image = ref
	var netCfg *network.NetworkingConfig
	if info.NetworkSettings != nil && len(info.NetworkSettings.Networks) > 0 {
		netCfg = &network.NetworkingConfig{EndpointsConfig: info.NetworkSettings.Networks}
	}
	created, err := e.CLI.ContainerCreate(ctx, cfg, info.HostConfig, netCfg, nil, name)
	if err != nil {
		_ = e.CLI.ContainerRename(ctx, prev, name)
		_ = e.CLI.ContainerStart(ctx, name, container.StartOptions{})
		return err
	}
	if err := e.CLI.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		_ = e.CLI.ContainerRemove(ctx, created.ID, container.RemoveOptions{Force: true})
		_ = e.CLI.ContainerRename(ctx, prev, name)
		_ = e.CLI.ContainerStart(ctx, name, container.StartOptions{})
		return err
	}
	return nil
}

func (e *DockerEngine) Rollback(ctx context.Context, name, previous string) error {
	prev := name + "-dp-prev"
	timeout := 10
	_ = e.CLI.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout})
	_ = e.CLI.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
	if err := e.CLI.ContainerRename(ctx, prev, name); err != nil {
		return err
	}
	return e.CLI.ContainerStart(ctx, name, container.StartOptions{})
}

func (e *DockerEngine) Healthy(ctx context.Context, name string) (bool, string) {
	deadline := time.Now().Add(15 * time.Second)
	var last string
	for {
		info, err := e.CLI.ContainerInspect(ctx, name)
		if err != nil {
			return false, err.Error()
		}
		if info.State != nil {
			if info.State.Health != nil {
				last = info.State.Health.Status
				if last == "healthy" {
					return true, last
				}
				if last == "unhealthy" {
					return false, last
				}
			} else if info.State.Running {
				return true, "running"
			} else {
				last = info.State.Status
			}
		}
		if time.Now().After(deadline) {
			if last == "" {
				last = "sem healthcheck"
			}
			return false, last
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err().Error()
		case <-time.After(time.Second):
		}
	}
}

// keep types import used on older compile paths
var _ = types.ContainerJSON{}

package discover

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/client"
)

// ProbeEngine lê o daemon. Um socket podman.sock já indica Podman mesmo sem resposta.
func ProbeEngine(ctx context.Context, cli *client.Client, dockerHost string) Facts {
	facts := Facts{Runtime: runtimeHint(dockerHost)}
	if cli == nil {
		facts.Err = "cliente Docker ausente"
		return facts
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	info, err := cli.Info(ctx)
	if err != nil {
		facts.Err = err.Error()
		return facts
	}
	facts.Reachable = true
	facts.SwarmState = string(info.Swarm.LocalNodeState)
	facts.Control = info.Swarm.ControlAvailable
	facts.NodeID = info.Swarm.NodeID
	facts.SwarmErr = info.Swarm.Error
	if facts.Runtime == "" {
		facts.Runtime = "docker"
	}
	ver, err := cli.ServerVersion(ctx)
	if err == nil {
		blob := strings.ToLower(ver.Platform.Name + " " + ver.Version)
		for _, c := range ver.Components {
			blob += " " + strings.ToLower(c.Name)
		}
		if strings.Contains(blob, "podman") {
			facts.Runtime = "podman"
		}
	}
	return facts
}

func runtimeHint(dockerHost string) string {
	if strings.Contains(strings.ToLower(dockerHost), "podman") {
		return "podman"
	}
	return ""
}

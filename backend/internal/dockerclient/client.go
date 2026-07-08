package dockerclient

import (
	"context"
	"os"
	"runtime"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/client"
)

// Client encapsula o cliente oficial do Docker.
type Client struct {
	CLI *client.Client
}

func New() (*Client, error) {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		return newWithHost("")
	}
	return newWithHost(host)
}

// defaultLocalHost aponta pro Docker local sem ler DOCKER_HOST do ambiente —
// evita que host "local" do pool herde um DOCKER_HOST=ssh://... do terminal.
func defaultLocalHost() string {
	if runtime.GOOS == "windows" {
		return "npipe:////./pipe/docker_engine"
	}
	return "unix:///var/run/docker.sock"
}

func newWithHost(dockerHost string) (*Client, error) {
	var opts []client.Opt
	switch {
	case dockerHost != "" && len(dockerHost) >= 6 && dockerHost[:6] == "ssh://":
		helper, err := connhelper.GetConnectionHelper(dockerHost)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHost(helper.Host), client.WithDialContext(helper.Dialer))
	case dockerHost != "":
		opts = append(opts, client.WithHost(dockerHost))
	default:
		opts = append(opts, client.WithHost(defaultLocalHost()))
	}
	opts = append(opts, client.WithAPIVersionNegotiation())
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	return &Client{CLI: cli}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.CLI.Ping(ctx)
	return err
}

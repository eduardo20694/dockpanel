// Package swarm opera um Docker Swarm pela Engine API: serviços, stacks,
// nós, secrets, configs, escala, rolling update, rollback, tasks e logs.
package swarm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"gopkg.in/yaml.v3"
)

type NodeView struct {
	ID           string `json:"id"`
	Hostname     string `json:"hostname"`
	Role         string `json:"role"`
	State        string `json:"state"`
	Availability string `json:"availability"`
	Leader       bool   `json:"leader"`
	Engine       string `json:"engine"`
}

type ServiceView struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Image     string   `json:"image"`
	Stack     string   `json:"stack,omitempty"`
	Mode      string   `json:"mode"`
	Replicas  uint64   `json:"replicas"`
	Running   uint64   `json:"running"`
	Ports     []string `json:"ports"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
}

type TaskView struct {
	ID      string `json:"id"`
	Slot    int    `json:"slot"`
	NodeID  string `json:"nodeId"`
	State   string `json:"state"`
	Desired string `json:"desired"`
	Error   string `json:"error,omitempty"`
	Image   string `json:"image,omitempty"`
}

type Named struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type StackView struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

func Active(ctx context.Context, cli *client.Client) error {
	info, err := cli.Info(ctx)
	if err != nil {
		return err
	}
	if !info.Swarm.ControlAvailable {
		return fmt.Errorf("este host não é manager de Swarm")
	}
	return nil
}

func ListNodes(ctx context.Context, cli *client.Client) ([]NodeView, error) {
	nodes, err := cli.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NodeView, 0, len(nodes))
	for _, n := range nodes {
		v := NodeView{
			ID: n.ID, Hostname: n.Description.Hostname, Role: string(n.Spec.Role),
			State: string(n.Status.State), Availability: string(n.Spec.Availability),
			Engine: n.Description.Engine.EngineVersion,
		}
		if n.ManagerStatus != nil {
			v.Leader = n.ManagerStatus.Leader
		}
		out = append(out, v)
	}
	return out, nil
}

func ListServices(ctx context.Context, cli *client.Client) ([]ServiceView, error) {
	list, err := cli.ServiceList(ctx, types.ServiceListOptions{Status: true})
	if err != nil {
		return nil, err
	}
	out := make([]ServiceView, 0, len(list))
	for _, s := range list {
		out = append(out, serviceView(s))
	}
	return out, nil
}

func serviceView(s swarm.Service) ServiceView {
	v := ServiceView{
		ID: s.ID, Name: s.Spec.Name, UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339),
		Stack: s.Spec.Labels["com.docker.stack.namespace"],
	}
	if s.Spec.TaskTemplate.ContainerSpec != nil {
		v.Image = s.Spec.TaskTemplate.ContainerSpec.Image
	}
	if s.Spec.Mode.Replicated != nil && s.Spec.Mode.Replicated.Replicas != nil {
		v.Mode = "replicated"
		v.Replicas = *s.Spec.Mode.Replicated.Replicas
	} else if s.Spec.Mode.Global != nil {
		v.Mode = "global"
	} else {
		v.Mode = "job"
	}
	if s.ServiceStatus != nil {
		v.Running = s.ServiceStatus.RunningTasks
		if s.ServiceStatus.DesiredTasks > 0 {
			v.Replicas = s.ServiceStatus.DesiredTasks
		}
	}
	if s.Endpoint.Spec.Ports != nil {
		for _, p := range s.Endpoint.Spec.Ports {
			v.Ports = append(v.Ports, fmt.Sprintf("%d->%d/%s", p.PublishedPort, p.TargetPort, p.Protocol))
		}
	}
	if v.Ports == nil {
		v.Ports = []string{}
	}
	return v
}

func inspect(ctx context.Context, cli *client.Client, name string) (swarm.Service, error) {
	svc, _, err := cli.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})
	return svc, err
}

func Scale(ctx context.Context, cli *client.Client, name string, replicas uint64) error {
	svc, err := inspect(ctx, cli, name)
	if err != nil {
		return err
	}
	if svc.Spec.Mode.Replicated == nil {
		return fmt.Errorf("serviço %s não é replicated", name)
	}
	svc.Spec.Mode.Replicated.Replicas = &replicas
	return update(ctx, cli, svc, types.ServiceUpdateOptions{})
}

func UpdateImage(ctx context.Context, cli *client.Client, name, image string) error {
	svc, err := inspect(ctx, cli, name)
	if err != nil {
		return err
	}
	if svc.Spec.TaskTemplate.ContainerSpec == nil {
		return fmt.Errorf("serviço sem container")
	}
	svc.Spec.TaskTemplate.ContainerSpec.Image = image
	svc.Spec.TaskTemplate.ForceUpdate++
	if svc.Spec.UpdateConfig == nil {
		svc.Spec.UpdateConfig = &swarm.UpdateConfig{Parallelism: 1, Order: "start-first"}
	}
	if svc.Spec.RollbackConfig == nil {
		svc.Spec.RollbackConfig = &swarm.UpdateConfig{Parallelism: 1, Order: "stop-first"}
	}
	return update(ctx, cli, svc, types.ServiceUpdateOptions{QueryRegistry: true})
}

func Rollback(ctx context.Context, cli *client.Client, name string) error {
	svc, err := inspect(ctx, cli, name)
	if err != nil {
		return err
	}
	return update(ctx, cli, svc, types.ServiceUpdateOptions{Rollback: "previous"})
}

func update(ctx context.Context, cli *client.Client, svc swarm.Service, opt types.ServiceUpdateOptions) error {
	resp, err := cli.ServiceUpdate(ctx, svc.ID, svc.Version, svc.Spec, opt)
	if err != nil {
		return err
	}
	if len(resp.Warnings) > 0 {
		return fmt.Errorf("%s", strings.Join(resp.Warnings, "; "))
	}
	return nil
}

func Tasks(ctx context.Context, cli *client.Client, name string) ([]TaskView, error) {
	f := filters.NewArgs()
	f.Add("service", name)
	list, err := cli.TaskList(ctx, types.TaskListOptions{Filters: f})
	if err != nil {
		return nil, err
	}
	out := make([]TaskView, 0, len(list))
	for _, t := range list {
		v := TaskView{ID: t.ID, Slot: t.Slot, NodeID: t.NodeID, State: string(t.Status.State), Desired: string(t.DesiredState), Error: t.Status.Err}
		if t.Spec.ContainerSpec != nil {
			v.Image = t.Spec.ContainerSpec.Image
		}
		out = append(out, v)
	}
	return out, nil
}

func Logs(ctx context.Context, cli *client.Client, name, tail string) (string, error) {
	if tail == "" {
		tail = "100"
	}
	rc, err := cli.ServiceLogs(ctx, name, container.LogsOptions{ShowStdout: true, ShowStderr: true, Tail: tail, Timestamps: true})
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, rc); err != nil && err != io.EOF {
		raw, _ := io.ReadAll(rc)
		if buf.Len() == 0 && len(raw) > 0 {
			return string(raw), nil
		}
	}
	return buf.String(), nil
}

func ListSecrets(ctx context.Context, cli *client.Client) ([]Named, error) {
	list, err := cli.SecretList(ctx, types.SecretListOptions{})
	if err != nil {
		return nil, err
	}
	return namedSecrets(list), nil
}

func CreateSecret(ctx context.Context, cli *client.Client, name string, data []byte) (string, error) {
	res, err := cli.SecretCreate(ctx, swarm.SecretSpec{Annotations: swarm.Annotations{Name: name}, Data: data})
	if err != nil {
		return "", err
	}
	return res.ID, nil
}

func RemoveSecret(ctx context.Context, cli *client.Client, id string) error {
	return cli.SecretRemove(ctx, id)
}

func ListConfigs(ctx context.Context, cli *client.Client) ([]Named, error) {
	list, err := cli.ConfigList(ctx, types.ConfigListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Named, 0, len(list))
	for _, c := range list {
		out = append(out, Named{ID: c.ID, Name: c.Spec.Name, CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339)})
	}
	return out, nil
}

func CreateConfig(ctx context.Context, cli *client.Client, name string, data []byte) (string, error) {
	res, err := cli.ConfigCreate(ctx, swarm.ConfigSpec{Annotations: swarm.Annotations{Name: name}, Data: data})
	if err != nil {
		return "", err
	}
	return res.ID, nil
}

func RemoveConfig(ctx context.Context, cli *client.Client, id string) error {
	return cli.ConfigRemove(ctx, id)
}

func namedSecrets(list []swarm.Secret) []Named {
	out := make([]Named, 0, len(list))
	for _, s := range list {
		out = append(out, Named{ID: s.ID, Name: s.Spec.Name, CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339)})
	}
	return out
}

func ListStacks(ctx context.Context, cli *client.Client) ([]StackView, error) {
	svcs, err := ListServices(ctx, cli)
	if err != nil {
		return nil, err
	}
	by := map[string][]string{}
	for _, s := range svcs {
		if s.Stack == "" {
			continue
		}
		by[s.Stack] = append(by[s.Stack], s.Name)
	}
	out := make([]StackView, 0, len(by))
	for name, services := range by {
		out = append(out, StackView{Name: name, Services: services})
	}
	return out, nil
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image  string        `yaml:"image"`
	Ports  []string      `yaml:"ports"`
	Deploy composeDeploy `yaml:"deploy"`
}

type composeDeploy struct {
	Replicas int `yaml:"replicas"`
}

// DeployStack sobe um compose no Swarm. Usa `docker stack deploy` quando o
// CLI existe; senão cria os serviços pela API com o rótulo de stack.
func DeployStack(ctx context.Context, cli *client.Client, dockerHost, name string, compose []byte) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("nome da stack obrigatório")
	}
	if _, err := exec.LookPath("docker"); err == nil {
		dir, err := os.MkdirTemp("", "dockpilot-stack-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "compose.yml")
		if err := os.WriteFile(path, compose, 0o644); err != nil {
			return "", err
		}
		cmd := exec.CommandContext(ctx, "docker", "stack", "deploy", "-c", path, name)
		if dockerHost != "" {
			cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("docker stack deploy: %s", strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}
	return deployStackAPI(ctx, cli, name, compose)
}

func deployStackAPI(ctx context.Context, cli *client.Client, name string, compose []byte) (string, error) {
	var file composeFile
	if err := yaml.Unmarshal(compose, &file); err != nil {
		return "", err
	}
	if len(file.Services) == 0 {
		return "", fmt.Errorf("compose sem services")
	}
	var created []string
	for svcName, svc := range file.Services {
		if svc.Image == "" {
			return "", fmt.Errorf("serviço %s sem image", svcName)
		}
		replicas := uint64(1)
		if svc.Deploy.Replicas > 0 {
			replicas = uint64(svc.Deploy.Replicas)
		}
		spec := swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name:   name + "_" + svcName,
				Labels: map[string]string{"com.docker.stack.namespace": name},
			},
			TaskTemplate: swarm.TaskSpec{ContainerSpec: &swarm.ContainerSpec{Image: svc.Image}},
			Mode:         swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: &replicas}},
			EndpointSpec: portsOf(svc.Ports),
		}
		res, err := cli.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
		if err != nil {
			return "", err
		}
		created = append(created, res.ID)
	}
	return "criados " + strings.Join(created, ","), nil
}

func portsOf(raw []string) *swarm.EndpointSpec {
	if len(raw) == 0 {
		return nil
	}
	var ports []swarm.PortConfig
	for _, p := range raw {
		pub, tgt, ok := splitPort(p)
		if !ok {
			continue
		}
		ports = append(ports, swarm.PortConfig{Protocol: "tcp", PublishedPort: pub, TargetPort: tgt, PublishMode: "ingress"})
	}
	if len(ports) == 0 {
		return nil
	}
	return &swarm.EndpointSpec{Ports: ports}
}

func splitPort(raw string) (uint32, uint32, bool) {
	raw = strings.Split(raw, "/")[0]
	parts := strings.Split(raw, ":")
	if len(parts) == 1 {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, false
		}
		return uint32(n), uint32(n), true
	}
	pub, err1 := strconv.Atoi(parts[len(parts)-2])
	tgt, err2 := strconv.Atoi(parts[len(parts)-1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return uint32(pub), uint32(tgt), true
}

func RemoveStack(ctx context.Context, cli *client.Client, dockerHost, name string) error {
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.CommandContext(ctx, "docker", "stack", "rm", name)
		if dockerHost != "" {
			cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)
		}
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		if !strings.Contains(string(out), "not found") && !strings.Contains(strings.ToLower(string(out)), "nothing found") {
			// fall through to API if the CLI failed for another reason only when no services exist
			_ = out
		}
	}
	svcs, err := ListServices(ctx, cli)
	if err != nil {
		return err
	}
	for _, s := range svcs {
		if s.Stack == name || strings.HasPrefix(s.Name, name+"_") {
			if err := cli.ServiceRemove(ctx, s.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateService cria um serviço replicated solto, fora de uma stack.
func CreateService(ctx context.Context, cli *client.Client, name, image string, replicas uint64) (string, error) {
	if replicas == 0 {
		replicas = 1
	}
	res, err := cli.ServiceCreate(ctx, swarm.ServiceSpec{
		Annotations:    swarm.Annotations{Name: name},
		TaskTemplate:   swarm.TaskSpec{ContainerSpec: &swarm.ContainerSpec{Image: image}},
		Mode:           swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: &replicas}},
		UpdateConfig:   &swarm.UpdateConfig{Parallelism: 1, Order: "start-first"},
		RollbackConfig: &swarm.UpdateConfig{Parallelism: 1, Order: "stop-first"},
	}, types.ServiceCreateOptions{})
	if err != nil {
		return "", err
	}
	return res.ID, nil
}

func RemoveService(ctx context.Context, cli *client.Client, name string) error {
	return cli.ServiceRemove(ctx, name)
}

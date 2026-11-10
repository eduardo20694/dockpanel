package main

import (
	"context"
	"encoding/json"

	"fmt"

	"dockerpilot/internal/discover"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/kube"
	"dockerpilot/internal/swarm"

	"github.com/docker/docker/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerOrchestratorTools(s *server.MCPServer, pool *dockerclient.Pool, clusters *kube.Set) {
	text := func(v any) *mcp.CallToolResult {
		raw, _ := json.MarshalIndent(v, "", "  ")
		return mcp.NewToolResultText(string(raw))
	}
	cliOf := func(ctx context.Context, req mcp.CallToolRequest) (*client.Client, error) {
		id := req.GetString("host_id", "")
		dc, err := hostClient(pool, id)
		if err != nil {
			return nil, err
		}
		dockerHost := ""
		kind := "auto"
		if pool != nil {
			for _, h := range pool.List() {
				if h.ID == id || (id == "" && h.ID == pool.DefaultID()) {
					dockerHost = h.DockerHost
					kind = h.Kind
				}
			}
		}
		view := discover.Classify(kind, discover.ProbeEngine(ctx, dc.CLI, dockerHost))
		if !view.ManageSwarm() {
			msg := view.Detail
			if msg == "" {
				msg = "a gestão do Swarm só está disponível num manager"
			}
			return nil, fmt.Errorf("%s", msg)
		}
		return dc.CLI, nil
	}
	clusterOf := func(req mcp.CallToolRequest) (*kube.Cluster, error) {
		if clusters == nil {
			return nil, errNoCluster
		}
		return clusters.Get(req.GetString("host_id", ""))
	}

	add := func(name, desc string, fn server.ToolHandlerFunc, opts ...mcp.ToolOption) {
		opts = append([]mcp.ToolOption{mcp.WithDescription(desc), mcp.WithString("host_id", mcp.Description("ambiente (list_environments)"))}, opts...)
		s.AddTool(mcp.NewTool(name, opts...), fn)
	}

	add("swarm_nodes", "Lista os nós do Swarm no ambiente.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := swarm.ListNodes(ctx, cli)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	})
	add("swarm_services", "Lista serviços do Swarm, com imagem, réplicas e stack.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := swarm.ListServices(ctx, cli)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	})
	add("swarm_tasks", "Lista as tasks de um serviço Swarm.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := swarm.Tasks(ctx, cli, req.GetString("service", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	}, mcp.WithString("service", mcp.Required()))
	add("swarm_logs", "Lê o log de um serviço Swarm.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		logs, err := swarm.Logs(ctx, cli, req.GetString("service", ""), req.GetString("tail", "100"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(logs), nil
	}, mcp.WithString("service", mcp.Required()), mcp.WithString("tail"))
	add("swarm_scale", "Escala um serviço replicated.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := swarm.Scale(ctx, cli, req.GetString("service", ""), uint64(atoi(req.GetString("replicas", "1")))); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(map[string]bool{"ok": true}), nil
	}, mcp.WithString("service", mcp.Required()), mcp.WithString("replicas", mcp.Required()))
	add("swarm_update", "Rolling update da imagem de um serviço. O rollback fica disponível em swarm_rollback.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := swarm.UpdateImage(ctx, cli, req.GetString("service", ""), req.GetString("image", "")); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(map[string]bool{"ok": true}), nil
	}, mcp.WithString("service", mcp.Required()), mcp.WithString("image", mcp.Required()))
	add("swarm_rollback", "Volta o serviço Swarm para a spec anterior.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := swarm.Rollback(ctx, cli, req.GetString("service", "")); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(map[string]bool{"ok": true}), nil
	}, mcp.WithString("service", mcp.Required()))
	add("swarm_secrets", "Lista secrets do Swarm. O conteúdo não é devolvido.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := swarm.ListSecrets(ctx, cli)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	})
	add("swarm_configs", "Lista configs do Swarm.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := swarm.ListConfigs(ctx, cli)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	})
	add("swarm_stacks", "Lista stacks pelo rótulo com.docker.stack.namespace.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := swarm.ListStacks(ctx, cli)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	})
	add("swarm_stack_deploy", "Sobe uma stack a partir do compose.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		cli, err := cliOf(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := swarm.DeployStack(ctx, cli, "", req.GetString("name", ""), []byte(req.GetString("compose", "")))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(out), nil
	}, mcp.WithString("name", mcp.Required()), mcp.WithString("compose", mcp.Required()))
	add("k8s_namespaces", "Lista namespaces do cluster Kubernetes selecionado.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := c.Namespaces(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	})
	add("k8s_workloads", "Lista Deployments, StatefulSets e DaemonSets.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := c.Workloads(ctx, req.GetString("namespace", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	}, mcp.WithString("namespace"))
	add("k8s_pods", "Lista pods do namespace.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := c.Pods(ctx, req.GetString("namespace", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	}, mcp.WithString("namespace"))
	add("k8s_pod_logs", "Lê o log de um pod.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		logs, err := c.PodLogs(ctx, req.GetString("namespace", "default"), req.GetString("pod", ""), req.GetString("container", ""), 200)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(logs), nil
	}, mcp.WithString("namespace"), mcp.WithString("pod", mcp.Required()), mcp.WithString("container"))
	add("k8s_what_broke", "Aponta CrashLoopBackOff, ImagePullBackOff, OOMKilled e pods Pending.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := c.WhatBroke(ctx, req.GetString("namespace", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	}, mcp.WithString("namespace"))
	add("k8s_events", "Lista eventos do namespace.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		list, err := c.Events(ctx, req.GetString("namespace", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(list), nil
	}, mcp.WithString("namespace"))
	add("k8s_apply", "Aplica um manifesto YAML (Namespace, Deployment, StatefulSet, DaemonSet, Service, Ingress, ConfigMap).", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		applied, err := c.ApplyYAML(ctx, []byte(req.GetString("yaml", "")), req.GetString("namespace", "default"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(applied), nil
	}, mcp.WithString("yaml", mcp.Required()), mcp.WithString("namespace"))
	add("k8s_scale", "Escala um Deployment ou StatefulSet.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := c.Scale(ctx, req.GetString("kind", "Deployment"), req.GetString("namespace", "default"), req.GetString("name", ""), int32(atoi(req.GetString("replicas", "1")))); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(map[string]bool{"ok": true}), nil
	}, mcp.WithString("name", mcp.Required()), mcp.WithString("namespace"), mcp.WithString("kind"), mcp.WithString("replicas", mcp.Required()))
	add("k8s_rollout", "Reinicia o rollout de um Deployment, StatefulSet ou DaemonSet.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := c.RolloutRestart(ctx, req.GetString("kind", "Deployment"), req.GetString("namespace", "default"), req.GetString("name", "")); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(map[string]bool{"ok": true}), nil
	}, mcp.WithString("name", mcp.Required()), mcp.WithString("namespace"), mcp.WithString("kind"))
	add("k8s_undo", "Desfaz o Deployment para a ReplicaSet anterior.", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := ensureWritable(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		c, err := clusterOf(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := c.Undo(ctx, req.GetString("namespace", "default"), req.GetString("name", "")); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return text(map[string]bool{"ok": true}), nil
	}, mcp.WithString("name", mcp.Required()), mcp.WithString("namespace"))
}

var errNoCluster = errString("nenhum cluster Kubernetes configurado")

type errString string

func (e errString) Error() string { return string(e) }

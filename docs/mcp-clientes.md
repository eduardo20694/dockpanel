# Conectar o MCP do Dockpilot

O binário `dockerpilot-mcp` fala stdio (Claude Desktop, Claude Code e Cursor na máquina) e HTTP streamable em `/mcp`, com SSE em `/sse` para clientes mais antigos. No HTTP cada pessoa usa a própria chave (`Authorization: Bearer dp_live_...`). A chave carrega papel e organização, pode expirar, ser revogada, e entra na auditoria.

Suba o transporte remoto com `DOCKERPILOT_MCP_ADDR=:8090`.

## Claude Desktop

Arquivo `claude_desktop_config.json`.

Stdio, no host:

```json
{
  "mcpServers": {
    "dockpilot": {
      "command": "dockerpilot-mcp",
      "env": {
        "DOCKERPILOT_HOSTS": "[{\"id\":\"local\",\"label\":\"Docker\",\"dockerHost\":\"\"}]"
      }
    }
  }
}
```

Remoto, com chave:

```json
{
  "mcpServers": {
    "dockpilot": {
      "url": "https://dockpilot.exemplo/mcp",
      "headers": {
        "Authorization": "Bearer dp_live_COLE_A_CHAVE"
      }
    }
  }
}
```

## Claude Code

```bash
claude mcp add --transport http dockpilot https://dockpilot.exemplo/mcp \
  --header "Authorization: Bearer dp_live_COLE_A_CHAVE"
```

Stdio local:

```bash
claude mcp add dockpilot -- dockerpilot-mcp
```

## Cursor

`.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "dockpilot": {
      "url": "http://127.0.0.1:8090/mcp",
      "headers": {
        "Authorization": "Bearer dp_live_COLE_A_CHAVE"
      }
    }
  }
}
```

Stdio no Cursor:

```json
{
  "mcpServers": {
    "dockpilot": {
      "command": "dockerpilot-mcp",
      "env": {
        "DOCKERPILOT_HOSTS": "[{\"id\":\"local\",\"label\":\"Docker\",\"dockerHost\":\"\"}]"
      }
    }
  }
}
```

## GitOps, cofre, imagens e atendimento

O mesmo servidor MCP expõe `gitops_status`, `gitops_configure`, `gitops_sync`, `gitops_approve`, `secrets_list`, `secrets_put`, `secrets_scan`, `image_updates`, `apply_image_update`, `list_templates`, `deploy_template`, `create_guest_access`, `revoke_guest_access`, `session_timeline`, `render_report` e `container_exec`. Papel, escopo da chave e `DOCKERPILOT_READONLY` continuam valendo.

```env
DOCKERPILOT_SECRETS_KEY=uma-frase-ou-32-bytes
DOCKERPILOT_REGISTRY_USER=
DOCKERPILOT_REGISTRY_PASSWORD=
DOCKERPILOT_REGISTRY_TOKEN=
```

O webhook de GitOps é `POST /api/gitops/webhook`, com `X-Hub-Signature-256` (GitHub) ou `X-Gitlab-Signature-256` / `X-Gitlab-Token` (GitLab).

## Swarm e Kubernetes

`list_environments` devolve o tipo detectado (`kind`, `detectedKind`, `role`, `status`, `detail`, `capabilities`). `kind` vazio ou `auto` em `DOCKERPILOT_HOSTS` segue a detecção; um kind explícito permanece. Swarm só é gerível quando `role` é `manager`. `host_id` escolhe o ambiente. Swarm: `swarm_nodes`, `swarm_services`, `swarm_tasks`, `swarm_logs`, `swarm_scale`, `swarm_update`, `swarm_rollback`, `swarm_secrets`, `swarm_configs`, `swarm_stacks`, `swarm_stack_deploy`. Kubernetes: `k8s_namespaces`, `k8s_workloads`, `k8s_pods`, `k8s_pod_logs`, `k8s_what_broke`, `k8s_events`, `k8s_apply`, `k8s_scale`, `k8s_rollout`, `k8s_undo`. O cluster entra por `DOCKERPILOT_KUBECONFIG` ou por um host `kind: kubernetes` em `DOCKERPILOT_HOSTS`.


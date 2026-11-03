# Dockpilot

Painel e servidor MCP para operar Docker em organizações. Roda no host, fala com o socket ou com `ssh://`, e não precisa de um Portainer no meio.

O módulo Go é `dockerpilot`. O binário do painel é `dockerpilot` e o do MCP é `dockerpilot-mcp`. Variáveis `DOCKERPILOT_*` são as oficiais. `DOCKPANEL_*` continua valendo quando a nova não está definida, e o header `X-Dockpanel-Host` ainda é aceito ao lado de `X-Dockerpilot-Host`.

## Por que não o Portainer MCP

O [Portainer MCP](https://github.com/portainer/portainer-mcp) expõe a API do Portainer. Ele é forte se a organização já vive dentro do Portainer (Edge, Helm, GitOps, centenas de rotas geradas do OpenAPI). Ele também pede uma instância Portainer, uma API key por pessoa e, por padrão, devolve um guia na primeira chamada antes de executar a ferramenta.

O Dockpilot cobre as mesmas áreas que importam para operar containers — vários ambientes, usuários e times, RBAC, stack com atualização e rollback, modo somente leitura, proxy Docker e Kubernetes quando faz sentido — e acrescenta o que o Portainer MCP não faz: diagnóstico, “algo quebrou”, auditoria de segurança, CVE com Trivy, drift de compose, Log Center, alertas, safe prune, backup de volume e trilha de auditoria. As descrições das ferramentas são escritas para o modelo, sem bloquear a primeira chamada.

Não reimplementamos Edge, Helm nem o catálogo inteiro de ~380 rotas do Portainer. Quem precisa administrar o produto Portainer continua no Portainer MCP. Quem precisa operar o Docker da organização usa o Dockpilot.

## Comparação

| Capacidade | Portainer MCP | Dockpilot |
|---|---|---|
| Onde roda | Na frente de um Portainer | No host, socket ou `ssh://`, sem Portainer |
| Vários ambientes | Endpoints do Portainer | `DOCKERPILOT_HOSTS` (local e SSH) |
| Usuários, times, RBAC | Perfil ADMIN, via API do Portainer | Diretório local: admin, operator, auditor, viewer |
| Organizações | O próprio Portainer | Multi-tenant em `orgs.json` |
| Stack criar / atualizar | Sim, pela API do Portainer | Sim, com revisão do compose |
| Rollback de stack | Depende do fluxo GitOps | Revisão local + ship registrado |
| Somente leitura | `PORTAINER_READ_ONLY` | `DOCKERPILOT_READONLY` (API e MCP) |
| Proxy Docker | Sim, pelo endpoint Portainer | Sim, direto na Engine API, com guarda de path |
| Proxy Kubernetes | Sim | Sim, se `DOCKERPILOT_K8S_API` existir; senão explica que não está configurado |
| SSO | API key do Portainer | OIDC (issuer, client id, HMAC ou gateway) |
| Docker standalone | Pela API do Portainer | Socket ou `ssh://`, containers, compose, logs, terminal |
| Docker Swarm | Pela API do Portainer | Nativo: nós, serviços, tasks, logs, secrets, configs, escala, rolling update, rollback e stacks |
| Kubernetes | Proxy da API | Nativo: kubeconfig ou service account, vários clusters, workloads, pods, services, ingress, configmaps, secrets (só nomes), escala, rollout, undo, eventos, YAML, Helm e “o que quebrou” |

## Frente a Arcane, Dockhand e Portainer

| | Arcane | Dockhand | Portainer | Dockpilot |
|---|---|---|---|---|
| Foco | Painel Docker moderno | Painel Docker leve | Plataforma com Edge e Swarm/K8s pela UI | Operação e MCP no host |
| Swarm | Parcial, quando exposto pela API | Não é o centro | Completo na UI | Completo na API, na UI e no MCP |
| Kubernetes | Não é o produto | Não é o produto | UI e proxy | Cliente nativo, vários clusters, diagnóstico |
| MCP | Não | Não | Portainer MCP, na frente do Portainer | MCP direto na Engine, no Swarm e no Kubernetes |
| GitOps, cofre, chaves por pessoa | Não nesse formato | Não | Mais no Business | Sim, no mesmo binário |
| “O que quebrou” | Não | `/broke` + `what_broke`: último ship, restarts, Log Center |
| Auditoria de segurança | Não é o foco | Privileged, `:latest`, portas, root |
| CVE | Não | Trivy |
| Drift de compose | Não | Compose local contra o que está rodando |
| Log Center | Não | Busca, incidentes, retenção |
| Alertas | Não | Telegram / Discord |
| Safe prune | Não | Relatório sem apagar sozinho |
| Backup | Backup do servidor Portainer | Backup de volume |
| Trilha de auditoria | Não | JSONL de toda escrita da API |
| Custo | Não | Estimativa USD/dia por reserva de CPU e memória |
| Descrições para IA | Geradas do OpenAPI; a primeira chamada pode devolver só o guia | Escritas para o modelo; `dockpilot_guidance` é opcional e não bloqueia |

## O que o painel faz

| Área | O que você vê |
|---|---|
| Painel | Saúde do host, problemas, segurança |
| Algo quebrou | Relato do último ship e a sugestão: rollback, restart ou nada a fazer |
| Erros | Crash loop, OOM, exit code; dá para marcar como resolvido |
| Containers | Lista, ações, logs, terminal, investigação |
| Stacks | Projetos compose e revisão / rollback |
| Segurança | Score e findings |
| Log Center | Busca, incidentes, retenção em `/api/logs/incidents/retention` |
| Organização | Pessoas, times, SSO, custo, auditoria |
| Deploy | `docker compose` e drift |
| MCP | As mesmas capacidades para Cursor, Claude e outros clientes stdio |

## Arquitetura

```
Browser  →  nginx + dockerpilot (API Go)
                ├─ socket Docker ou ssh://
                ├─ /data  (last-ship.json, resolved_problems.json, audit.jsonl, orgs.json, logs)
                └─ dockerpilot-mcp  (stdio, mesmo código de domínio)
```

Estado operacional continua no Docker. O disco do Dockpilot guarda histórico, ships, revisões, diretório e auditoria. Não há Postgres.

## Subir

```bash
docker build -t redecoop/dockpilot:1.1.0 .
```

Variáveis mínimas (o compose deste repositório ainda aceita os nomes `DOCKPANEL_*`):

```env
DOCKERPILOT_ADMIN_EMAIL=voce@org.dev
DOCKERPILOT_ADMIN_PASSWORD=senha-forte
DOCKERPILOT_JWT_SECRET=<segredo-com-32-ou-mais>
DOCKERPILOT_HOSTS=[{"id":"local","label":"Docker","dockerHost":""}]
```

Opcionais: `DOCKERPILOT_READONLY=1`, `DOCKERPILOT_OIDC_ISSUER`, `DOCKERPILOT_OIDC_CLIENT_ID`, `DOCKERPILOT_OIDC_HMAC_SECRET`, `DOCKERPILOT_OIDC_ADMIN_GROUP`, `DOCKERPILOT_K8S_API`, `DOCKERPILOT_K8S_TOKEN`, `DOCKERPILOT_CPU_USD_HOUR`, `DOCKERPILOT_MEM_GB_USD_HOUR`, `DOCKERPILOT_DATA_DIR`, `DOCKERPILOT_COMPOSE_PATH`, `DOCKERPILOT_LOG_RETENTION_DAYS`.

Papéis: `admin` administra pessoas e times; `operator` opera containers e stacks; `auditor` só lê, inclusive a trilha; `viewer` lê e não vê a trilha.

## Desenvolvimento

Go 1.25+ e Node 20+.

```bash
cp .env.example .env
cd backend && go test ./... && go run ./cmd/server
cd frontend && npm ci && npm test && npm run dev
```

| Serviço | URL |
|---|---|
| UI | http://localhost:5173 |
| API | http://localhost:8080 |

Testes de ponta a ponta da interface:

```bash
cd frontend && npx playwright install chromium && npm run test:e2e
```

O MCP:

```bash
cd backend && go build -o dockerpilot-mcp ./cmd/mcp
```

Ferramentas novas em relação ao painel antigo: `list_environments`, `list_users`, `create_user`, `list_teams`, `create_team`, `add_team_member`, `create_stack`, `update_stack`, `rollback_stack`, `docker_proxy`, `kubernetes_proxy`, `what_broke`, `resolve_problem`, `list_audit`, `cost_insights`, `dockpilot_guidance`. As ferramentas de diagnóstico, Trivy, drift, prune, backup e logs continuam.

## GitOps, cofre e atendimento

Uma stack pode apontar para um repositório (URL, branch, caminho do compose e deploy key). O sync entra por webhook HMAC (GitHub ou GitLab) ou por polling, mostra o diff e aplica em modo automático ou com aprovação. Healthcheck ruim dispara rollback. O histórico cai na auditoria.

Segredos ficam cifrados com AES-GCM. A chave mestra é `DOCKERPILOT_SECRETS_KEY`. O valor não volta na API. No deploy ele vira env ou arquivo. Compose com senha em texto ganha um aviso para migrar ao cofre.

Imagens são conferidas no Docker Hub, GHCR ou registry privado. A atualização pode esperar aprovação ou seguir política automática, com rollback e notificação.

Consultor entra com chave de poucas horas, escopo de stack e host, revogação imediata e linha do tempo. O terminal web (xterm.js) respeita papel e somente leitura e grava os comandos. O relatório de atendimento sai em Markdown e PDF. Templates (Postgres, Redis, Nginx+SSL, n8n, MySQL, MongoDB, Traefik, Grafana+Prometheus) sobem em um clique e mandam a senha para o cofre.

As mesmas operações existem como tools MCP.

## Licença de uso

Self-hosted. Segredos ficam no ambiente, nunca no compose commitado.

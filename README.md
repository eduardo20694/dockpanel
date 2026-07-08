# dockpanel

Painel Docker com **diagnóstico inteligente** — complementa o Portainer onde ele é fraco: erros, segurança, stacks, métricas e investigação profunda. Inclui servidor **MCP** para usar com Claude Desktop / Cursor.

---

## O que é

| Camada | Função |
|--------|--------|
| **Dashboard** | UI React com visão executiva, erros, containers, imagens, volumes, redes, deploy e tema claro/escuro |
| **Backend** | API REST + WebSocket (Go) sobre o Docker API |
| **MCP** | Ferramentas para agentes de IA diagnosticarem a infra via stdio |

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│  Browser    │────▶│  Go API      │────▶│  Docker         │
│  :5173 dev  │     │  :8080       │     │  local ou SSH   │
│  :9090 prod │     └──────────────┘     └─────────────────┘
└─────────────┘            │
                           ▼
                    JSONL store (métricas, alertas)

┌─────────────┐     ┌──────────────┐
│ Claude /    │────▶│ dockpanel-mcp│──▶ mesmo motor interno
│ Cursor      │     │ (stdio)      │
└─────────────┘     └──────────────┘
```

---

## Funcionalidades

### Dashboard

| Página | Descrição |
|--------|-----------|
| **Executivo** | Health score, alertas, problemas recentes, snapshot de segurança |
| **Erros** | Crash loops, OOM, exit codes — diagnóstico por container |
| **Stacks** | Saúde por projeto Docker Compose |
| **Segurança** | Root, privileged, portas expostas, tags `:latest` |
| **Containers** | CPU/RAM ao vivo, logs, terminal, menu de ações |
| **Imagens** | Lista, scan Trivy, remoção |
| **Volumes** | Backup tarball, remoção com backup opcional |
| **Redes** | Detalhes (subnet, containers, IPs, labels) |
| **Deploy** | `docker compose` up/down/build, drift compose vs running |
| **Investigação** | Deep dive por container com histórico de métricas |

Dados atualizam automaticamente a cada **15 segundos**. Tema **dia/noite** com persistência.

### Backend

- Coletor de métricas (CPU/RAM) a cada 30s → `backend/data/`
- Alertas proativos Telegram/Discord (containers critical)
- WebSocket para terminal interativo (xterm.js)
- Multi-host via `DOCKPANEL_HOSTS` (ex.: VPS por SSH)

### MCP — 18 ferramentas

| Ferramenta | O que faz |
|------------|-----------|
| `list_hosts` | Hosts Docker configurados |
| `list_containers` | Lista containers |
| `container_action` | start / stop / restart |
| `container_logs` | Últimas linhas de log |
| `diagnose_container` | Diagnóstico completo (exit, OOM, logs, eventos) |
| `scan_problems` | Varredura de todos os problemas |
| `investigate_container` | Investigação profunda + histórico |
| `executive_summary` | Resumo executivo multi-host |
| `security_audit` | Auditoria de segurança |
| `stack_health` | Saúde das stacks Compose |
| `safe_prune_report` | O que dá pra limpar (sem remover) |
| `remove_resource` | Remove container/imagem/volume |
| `system_overview` | Info do daemon Docker |
| `deploy_compose` | Roda docker compose no host |
| `check_compose_drift` | Drift superficial |
| `deep_compose_drift` | Drift imagem compose vs running |
| `scan_image_vulnerabilities` | Scan Trivy (requer `trivy` no PATH) |
| `backup_volume` | Tarball de backup de volume |

---

## Requisitos

- **Go 1.25+**
- **Node.js 18+** (frontend)
- **Docker** acessível (socket local ou `ssh://user@host`)
- Opcional: **Trivy** (scan de imagens), **docker compose** (deploy/drift)

---

## Desenvolvimento local

### 1. Backend

```powershell
cd backend
go mod tidy
go build -o dockpanel-server-v3.exe ./cmd/server

$env:DOCKPANEL_HOSTS = '[{"id":"vps","label":"VPS","dockerHost":"ssh://root@SEU_IP"}]'
.\dockpanel-server-v3.exe
```

API em **http://localhost:8080**

### 2. Frontend

```powershell
cd frontend
npm install
npm run dev
```

Dashboard em **http://localhost:5173** (proxy `/api` → `:8080`)

### 3. MCP (Cursor / Claude Desktop)

```powershell
cd backend
go build -o dockpanel-mcp.exe ./cmd/mcp
```

Copie `.cursor/mcp.json.example` → `.cursor/mcp.json` e ajuste caminhos e host SSH.

---

## Variáveis de ambiente

| Variável | Descrição |
|----------|-----------|
| `DOCKPANEL_HOSTS` | JSON com hosts `[{id, label, dockerHost}]` |
| `DOCKER_HOST` | Host único (fallback) |
| `PORT` | Porta do backend (padrão `8080`) |
| `DOCKPANEL_DATA_DIR` | Pasta do store JSONL (padrão `data/`) |
| `DOCKPANEL_BACKUP_DIR` | Destino dos backups de volume |
| `DOCKPANEL_COMPOSE_PATH` | Pasta padrão para presets de deploy |
| `ALERT_TELEGRAM_BOT_TOKEN` | Token do bot Telegram |
| `ALERT_TELEGRAM_CHAT_ID` | Chat ID para alertas |
| `ALERT_DISCORD_WEBHOOK` | Webhook Discord para alertas |

### Exemplo VPS-only

```powershell
$env:DOCKPANEL_HOSTS = '[{"id":"vps","label":"Minha VPS","dockerHost":"ssh://user@SEU_IP"}]'
```

> **Git:** não commite IP, usuário SSH, tokens ou `.cursor/mcp.json`. Use `.env.example` e `.cursor/mcp.json.example` como modelos.

---

## Produção (Docker Compose)

```bash
docker compose up -d --build
```

| Serviço | Porta | Descrição |
|---------|-------|-----------|
| `dockpanel-frontend` | **9090** | Nginx + UI estática |
| `dockpanel-backend` | interna | API + socket do host |

> **Segurança:** acesso ao Docker socket = root no host. Não exponha a porta 9090 na internet sem autenticação (Caddy, Traefik, Cloudflare Access, etc.). O MCP roda local via stdio de propósito.

---

## Estrutura do projeto

```
dockpanel/
├── backend/
│   ├── cmd/server/          # API REST + WebSocket
│   ├── cmd/mcp/             # Servidor MCP (stdio)
│   └── internal/
│       ├── api/             # Handlers HTTP
│       ├── diagnostics/     # Motor de diagnóstico
│       ├── insights/        # Auditoria de segurança
│       ├── stacks/          # Health por stack
│       ├── drift/           # Compose drift
│       ├── deploy/          # Docker compose
│       ├── backup/          # Backup de volumes
│       ├── scan/            # Trivy
│       ├── alerts/          # Telegram / Discord
│       ├── collector/       # Métricas periódicas
│       ├── store/           # Persistência JSONL
│       └── dockerclient/    # Pool multi-host
├── frontend/
│   └── src/
│       ├── pages/           # 10 páginas do dashboard
│       ├── components/      # UI, terminal, modais
│       ├── context/         # Host, tema
│       └── lib/             # Polling, formatação
├── docker-compose.yml
└── .cursor/mcp.json.example
```

---

## Stack técnica

| Parte | Tecnologias |
|-------|-------------|
| Frontend | React 18, TypeScript, Vite 5, Tailwind 3, react-router 6, xterm.js |
| Backend | Go 1.25, chi, gorilla/websocket, Docker SDK |
| MCP | mcp-go |
| Deploy | Docker Compose, nginx |

---

## Licença

Uso interno / projeto pessoal. Ajuste conforme necessário.

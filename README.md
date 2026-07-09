# dockpanel

Painel Docker com **diagnóstico inteligente**, **login**, **deploy** e **MCP para agentes de IA** — complementa o Portainer onde ele é fraco: erros, segurança, stacks, métricas e investigação profunda.

---

## Visão geral

```
┌─────────────────────────────────────────────────────────────┐
│  Browser  →  :8083 (nginx)  →  UI React                     │
│                    ↓ /api/*                                 │
│              dockpanel API (:8080 interno)                  │
│                    ↓                                        │
│         Docker socket + PostgreSQL (auth)                   │
└─────────────────────────────────────────────────────────────┘

┌──────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ Cursor /     │────▶│ dockpanel-mcp   │────▶│ Docker (local   │
│ Claude       │     │ 18 tools stdio  │     │ ou SSH na VPS)  │
└──────────────┘     └─────────────────┘     └─────────────────┘
```

| Camada | Função |
|--------|--------|
| **Frontend** | React + Vite — dashboard, login, terminal, deploy |
| **Backend** | API REST + WebSocket (Go) sobre Docker API |
| **Auth** | JWT + PostgreSQL (bcrypt) |
| **MCP** | 18 ferramentas para Cursor / Claude Desktop |
| **Imagem** | UI + API + Trivy num container só |

---

## Funcionalidades

### Dashboard

| Página | Descrição |
|--------|-----------|
| **Executivo** | Health score, alertas, problemas recentes |
| **Erros** | Crash loops, OOM, exit codes |
| **Stacks** | Saúde por projeto Docker Compose |
| **Segurança** | Root, privileged, portas expostas, `:latest` |
| **Containers** | CPU/RAM ao vivo, logs, terminal, ações |
| **Imagens** | Lista, scan Trivy, remoção |
| **Volumes** | Backup tarball, remoção |
| **Redes** | Subnet, containers, IPs |
| **Deploy** | compose up/down/build, drift, logs |
| **Investigação** | Deep dive por container + histórico |

Dados atualizam a cada **15s**. Tema claro/escuro.

### Backend

- Coletor de métricas (30s) → JSONL em `/data`
- Alertas Telegram / Discord
- WebSocket para terminal (xterm.js)
- Multi-host via `DOCKPANEL_HOSTS` (local ou `ssh://`)
- Scan Trivy embutido na imagem de produção

### Autenticação

- Login com email + senha (PostgreSQL)
- JWT em cookie httpOnly
- Middleware protege `/api/*` quando auth está ativo
- Bootstrap do admin via env ou insert manual no banco

---

## MCP — 18 ferramentas

Todas registradas em `backend/cmd/mcp/`. Rebuild após mudanças:

```powershell
cd backend
go build -o dockpanel-mcp.exe ./cmd/mcp
```

| # | Ferramenta | O que faz |
|---|------------|-----------|
| 1 | `list_hosts` | Hosts Docker configurados (`DOCKPANEL_HOSTS`) |
| 2 | `list_containers` | Lista containers do host default |
| 3 | `container_action` | start / stop / restart |
| 4 | `container_logs` | Últimas linhas de log |
| 5 | `diagnose_container` | Diagnóstico (exit, OOM, logs, eventos) |
| 6 | `scan_problems` | Varredura de problemas em todos containers |
| 7 | `investigate_container` | Investigação profunda + histórico |
| 8 | `executive_summary` | Resumo executivo multi-host |
| 9 | `security_audit` | Auditoria de segurança |
| 10 | `stack_health` | Saúde das stacks Compose |
| 11 | `safe_prune_report` | O que dá pra limpar (sem remover) |
| 12 | `remove_resource` | Remove container / imagem / volume |
| 13 | `system_overview` | Info do daemon Docker |
| 14 | `deploy_compose` | docker compose (usa `DOCKPANEL_HOSTS` + `host_id`) |
| 15 | `check_compose_drift` | Drift superficial compose vs running |
| 16 | `deep_compose_drift` | Drift de imagem compose vs running |
| 17 | `scan_image_vulnerabilities` | Scan Trivy (precisa `trivy` no PATH do MCP) |
| 18 | `backup_volume` | Tarball de backup de volume |

### Configurar no Cursor

1. Build do MCP (comando acima)
2. Copie `.cursor/mcp.json.example` → `.cursor/mcp.json`
3. Ajuste `DOCKPANEL_HOSTS` (SSH da VPS) e caminho do `.exe`
4. Reinicie o MCP no Cursor (Settings → MCP → Reload)

```json
{
  "mcpServers": {
    "dockpanel-vps": {
      "command": "c:\\Github\\dockpanel\\backend\\dockpanel-mcp.exe",
      "env": {
        "DOCKPANEL_HOSTS": "[{\"id\":\"vps\",\"label\":\"VPS\",\"dockerHost\":\"ssh://root@SEU_IP\"}]",
        "DOCKPANEL_COMPOSE_PATH": "c:\\Github\\dockpanel",
        "DOCKPANEL_COMPOSE_PATH_REMOTE": "/root/dockpanel",
        "PATH": "C:\\Users\\SEU_USUARIO\\AppData\\Local\\Programs\\trivy;..."
      }
    }
  }
}
```

| Variável MCP | Uso |
|--------------|-----|
| `DOCKPANEL_HOSTS` | Conexão SSH com a VPS (`ssh://root@IP`) |
| `DOCKPANEL_COMPOSE_PATH` | Pasta **local** do repo (drift, ler compose) |
| `DOCKPANEL_COMPOSE_PATH_REMOTE` | Pasta **na VPS** para `deploy_compose` (`/root/dockpanel`) |

> **Todas as 18 tools** aceitam `host_id` (padrão: primeiro do `DOCKPANEL_HOSTS`). Com só a VPS configurada, tudo opera no servidor remoto.

> **Deploy remoto:** `deploy_compose` executa `ssh root@VPS 'cd /root/dockpanel && docker compose ...'` — não usa Docker local.

> **Drift:** compara o `docker-compose.yml` **local** com containers **remotos** na VPS.

---

## Requisitos

| Ambiente | Requisitos |
|----------|------------|
| **Dev** | Go 1.25+, Node 18+, Docker |
| **Prod** | Docker, PostgreSQL no host, Portainer (opcional) |
| **MCP** | Binário `dockpanel-mcp`, Trivy no PATH (scan) |

---

## Desenvolvimento local

### 1. Variáveis

```powershell
cp .env.example .env
# Edite .env — veja seção abaixo
```

**Dev com Postgres na VPS** — abra o túnel antes do backend:

```powershell
ssh -L 5433:127.0.0.1:5432 root@SEU_IP
```

No `.env`:

```env
DOCKPANEL_HOSTS=[{"id":"vps","label":"VPS","dockerHost":"ssh://root@SEU_IP"}]
POSTGRES_HOST=127.0.0.1
POSTGRES_PORT=5433
POSTGRES_USER=dockpanel
POSTGRES_PASSWORD=sua-senha
POSTGRES_DB=dockpanel
DOCKPANEL_JWT_SECRET=string-longa-aleatoria
```

### 2. Backend

```powershell
cd backend
go mod tidy
go build -o dockpanel-server.exe ./cmd/server
.\dockpanel-server.exe
```

API em **http://localhost:8080**

### 3. Frontend

```powershell
cd frontend
npm install
npm run dev
```

UI em **http://localhost:5173** (proxy `/api` → `:8080`)

### 4. Testes

```powershell
cd backend
go test ./...
```

---

## Produção

### Build da imagem

Um comando, uma imagem (UI + API + Trivy):

```bash
docker build -t redecoop/dockpanel:0.0.1 .
```

### Docker Compose / Portainer

| Item | Valor |
|------|-------|
| Imagem | `redecoop/dockpanel:0.0.1` |
| Porta host | **8083** → container `80` |
| CPU | 0.5 core |
| RAM | 512 MB |

```bash
docker compose up -d
```

No **Portainer**: cole `docker-compose.yml` e as variáveis de `.env.vps.example` em **Environment variables** (não use `env_file`).

### PostgreSQL no host

O container acessa o Postgres via `host.docker.internal`. Configure:

**pg_hba.conf:**
```
host    dockpanel    dockpanel    172.16.0.0/12    scram-sha-256
```

**UFW:**
```bash
ufw allow from 172.16.0.0/12 to any port 5432 comment 'PostgreSQL Docker'
```

**postgresql.conf:** `listen_addresses = '*'`

Script de setup: `backend/scripts/init-dockpanel-db.sql`  
Hash de senha: `go run backend/scripts/hashpassword.go "sua-senha"`

### Deploy na VPS

```bash
bash scripts/deploy-vps.sh
```

---

## Variáveis de ambiente

| Variável | Onde | Descrição |
|----------|------|-----------|
| `DOCKPANEL_HOSTS` | app + MCP | JSON `[{id, label, dockerHost}]` |
| `DOCKPANEL_COMPOSE_PATH` | MCP | Pasta local do compose (drift) |
| `DOCKPANEL_COMPOSE_PATH_REMOTE` | MCP | Pasta do compose na VPS (deploy) |
| `PORT` | container | API interna (padrão `8080`) |
| `DOCKPANEL_DATA_DIR` | container | Store JSONL (padrão `/data`) |
| `POSTGRES_*` | container | Conexão PostgreSQL |
| `DOCKPANEL_JWT_SECRET` | container | Assinatura JWT (obrigatório em prod) |
| `DOCKPANEL_ADMIN_*` | container | Bootstrap do primeiro admin |
| `ALERT_TELEGRAM_*` | container | Alertas Telegram |
| `ALERT_DISCORD_WEBHOOK` | container | Alertas Discord |

Arquivos de exemplo:

- `.env.example` — desenvolvimento local
- `.env.vps.example` — produção / Portainer

> **Nunca commite:** `.env`, `produção.env`, `.cursor/mcp.json`

---

## Estrutura do projeto

```
dockpanel/
├── Dockerfile                 # Build único (imagem dockpanel)
├── docker-compose.yml         # Produção (porta 8083, limites CPU/RAM)
├── docker/
│   ├── entrypoint.sh          # nginx + API no mesmo container
│   └── nginx.conf             # Proxy /api → :8080 interno
├── scripts/
│   └── deploy-vps.sh          # Deploy na VPS
├── backend/
│   ├── cmd/server/            # API REST + WebSocket
│   ├── cmd/mcp/               # Servidor MCP (18 tools)
│   ├── scripts/
│   │   ├── init-dockpanel-db.sql
│   │   └── hashpassword.go
│   └── internal/
│       ├── api/               # Handlers HTTP + auth
│       ├── auth/              # JWT, PostgreSQL, bcrypt
│       ├── diagnostics/       # Motor de diagnóstico
│       ├── deploy/            # docker compose
│       ├── drift/             # Compose drift
│       ├── scan/              # Trivy
│       └── ...
└── frontend/
    └── src/
        ├── pages/             # Dashboard, Login, Deploy, ...
        └── context/           # Auth, Host, Theme
```

---

## Stack técnica

| Parte | Tecnologias |
|-------|-------------|
| Frontend | React 18, TypeScript, Vite 5, Tailwind 3, xterm.js |
| Backend | Go 1.25, chi, gorilla/websocket, pgx, Docker SDK |
| Auth | JWT, bcrypt, PostgreSQL |
| MCP | mcp-go |
| Deploy | Docker, nginx, Trivy |

---

## Segurança

- Acesso ao **Docker socket** = root no host — proteja a porta **8083** (VPN, Cloudflare, IP allowlist)
- **Não exponha** PostgreSQL (`5432`) para `Anywhere` na internet
- Use senhas fortes e `DOCKPANEL_JWT_SECRET` longo em produção
- MCP roda **local** via stdio — não expõe API na rede

---

## Licença

Uso interno / projeto pessoal. Ajuste conforme necessário.

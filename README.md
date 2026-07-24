# Dockwatch

**Painel de operações Docker self-hosted.** Um container na VPS, socket montado, login único — você vê e controla a infraestrutura sem Portainer genérico, sem agent e sem banco.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](./backend)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](./frontend)
[![Docker](https://img.shields.io/badge/Docker-socket-2496ED?logo=docker&logoColor=white)](./docker-compose.yml)
[![License](https://img.shields.io/badge/uso-self--hosted-informational)](#)

---

## Por que existe

Operar Docker numa VPS costuma significar SSH + `docker ps` + compose na mão, ou um Portainer cheio de opções que você não precisa. O Dockwatch é o meio-termo: **painel focado em diagnóstico, segurança e deploy**, rodando **no mesmo host** que o Docker Engine.

Feito para um operador (você), não para SaaS multi-tenant.

---

## O que ele faz

| Área | Capacidades |
|------|-------------|
| **Visão** | Overview com saúde do host, problemas critical/warning, stacks e segurança |
| **Containers** | Lista, start/stop/restart/remove, logs ao vivo, terminal, investigação com métricas |
| **Erros** | Scan de crash loops, OOM, exit codes e padrões em logs |
| **Stacks** | Projetos compose detectados pelos labels dos containers |
| **Segurança** | Auditoria de configuração (privileged, :latest, mounts sensíveis, etc.) |
| **Imagens** | Registry local + scan CVE via Trivy (binário ou fallback Docker) |
| **Volumes / redes** | Listagem, backup de volume, inspect de rede |
| **Deploy** | `docker compose` up/down/build/pull/logs + drift compose ↔ running |
| **Limpeza** | Relatório safe-prune (dangling, volumes órfãos, exited) sem destruir nada sozinho |
| **Métricas / alertas** | Histórico JSONL local; Telegram/Discord opcional |
| **MCP** | Ferramentas stdio para Cursor / Claude gerenciarem o host via SSH ou socket |

---

## Arquitetura

```
┌─────────────┐     :8083      ┌──────────────────────────────┐
│  Browser    │ ─────────────► │  dockwatch (nginx + API Go)  │
└─────────────┘                └──────────────┬───────────────┘
                                              │
                         ┌────────────────────┼────────────────────┐
                         ▼                    ▼                    ▼
              /var/run/docker.sock      volume /data         /opt/dockpanel
              (Docker Engine)          (JSONL métricas)     (compose Deploy)
```

- **Sem Postgres / Redis** — estado operacional vive no Docker; histórico leve em arquivos.
- **Sem agent** — o painel é o cliente do socket (ou `ssh://` para outro host, avançado).
- **Segredos fora do YAML** — Portainer Environment / `.env`; o compose só referencia `${VAR}`.

---

## Quick start (produção)

### 1. Build & push (no seu PC)

```powershell
docker build -t redecoop/dockwatch:1.0.0 .
docker push redecoop/dockwatch:1.0.0
```

### 2. Stack na VPS / Portainer

Use o [`docker-compose.yml`](./docker-compose.yml) e cole as variáveis (modelo: [`.env.vps.example`](./.env.vps.example)):

```env
DOCKPANEL_ADMIN_EMAIL=voce@email.com
DOCKPANEL_ADMIN_PASSWORD=senha-forte
DOCKPANEL_ADMIN_NAME=Seu Nome
DOCKPANEL_JWT_SECRET=<gere-um-segredo-longo-aleatorio>
DOCKPANEL_HOSTS=[{"id":"local","label":"Docker","dockerHost":""}]
DOCKPANEL_PROJECT_DIR=.
DOCKPANEL_COMPOSE_PATH=/opt/dockpanel
```

```bash
docker compose pull
docker compose up -d
```

Abra **http://IP_DA_VPS:8083** → login com o admin do Environment.

> `dockerHost: ""` = usa o socket montado. Não configure `ssh://` para o mesmo host onde o painel já roda.

---

## Desenvolvimento local

Pré-requisitos: Go 1.25+, Node 20+, Docker Desktop (ou daemon local).

```powershell
cp .env.example .env
# ajuste DOCKPANEL_ADMIN_* e JWT

# terminal 1
cd backend
go run ./cmd/server

# terminal 2
cd frontend
npm install
npm run dev
```

| Serviço | URL |
|---------|-----|
| UI | http://localhost:5173 |
| API | http://localhost:8080 |

```powershell
cd backend;  go test ./...
cd frontend; npm run build
```

---

## Variáveis de ambiente

| Variável | Obrig. | Descrição |
|----------|:------:|-----------|
| `DOCKPANEL_ADMIN_EMAIL` | ✓ | Login |
| `DOCKPANEL_ADMIN_PASSWORD` | ✓ | Senha |
| `DOCKPANEL_JWT_SECRET` | ✓ | Segredo JWT (≥ 32 caracteres) |
| `DOCKPANEL_ADMIN_NAME` | | Nome exibido no painel |
| `DOCKPANEL_HOSTS` | ✓* | JSON de hosts; `dockerHost` vazio = socket |
| `DOCKPANEL_DATA_DIR` | | Pasta JSONL (prod: `/data`) |
| `DOCKPANEL_COMPOSE_PATH` | | Pasta com `docker-compose.yml` (tela Deploy) |
| `DOCKPANEL_PROJECT_DIR` | | Path no host montado no container |
| `DOCKPANEL_BACKUP_DIR` | | Destino de backup de volumes |
| `DOCKPANEL_COMPOSE_PATH_REMOTE` | | Compose em host `ssh://` (MCP / avançado) |
| `ALERT_TELEGRAM_BOT_TOKEN` / `CHAT_ID` | | Alertas Telegram |
| `ALERT_DISCORD_WEBHOOK` | | Alertas Discord |

\*Se omitido, a API sobe um host local padrão a partir de `DOCKER_HOST` / socket.

**Nunca commite** `.env` nem `.cursor/mcp.json`.

---

## MCP (Cursor / Claude)

Gerencie a VPS a partir do laptop sem abrir o painel:

```powershell
cd backend
go build -o dockwatch-mcp.exe ./cmd/mcp
copy .cursor\mcp.json.example .cursor\mcp.json
# edite SEU_IP e o caminho do .exe
```

| Server no example | Uso |
|-------------------|-----|
| `dockwatch-vps` | `ssh://root@SEU_IP` |
| `dockwatch-local` | Socket Docker local |

Ferramentas típicas: listar containers, logs, diagnóstico, drift de compose, prune report, scan Trivy, backup de volume, deploy compose.

---

## Estrutura do repositório

```
dockpanel/
├── backend/
│   ├── cmd/server/     # API HTTP
│   ├── cmd/mcp/        # MCP stdio
│   └── internal/       # dockerclient, diagnostics, deploy, scan, auth…
├── frontend/
│   └── src/pages/      # login + painel
├── docker/
│   ├── nginx.conf
│   └── entrypoint.sh
├── docker-compose.yml  # produção (pull image + socket)
├── Dockerfile
└── .env.vps.example
```

O módulo Go continua `dockpanel` (nome interno); o produto e a imagem são **Dockwatch**.

---

## Decisões de design (resumo)

1. **Socket first** — um container na VPS é o caminho feliz; SSH é para o MCP no laptop ou segundo host.
2. **Auth env-only** — um admin, JWT em cookie httpOnly; zero users table.
3. **JSONL em vez de DB** — métricas/alertas sem operar Postgres na VPS.
4. **Compose sem segredos** — Portainer injeta Environment; YAML versionável com segurança.
5. **Trivy pragmático** — binário se existir; senão `docker run aquasec/trivy`.

---

## Roadmap / limites honestos

- Terminal e logs são por container (não há “log center” global sem store extra).
- Alertas externos só disparam se `ALERT_*` estiver configurado; o histórico local roda sempre.
- Multi-host avançado existe via `DOCKPANEL_HOSTS`, mas o produto é otimizado para **um host**.

---

## Licença

Uso self-hosted. Ajuste a licença formal se for publicar o repositório como open source.

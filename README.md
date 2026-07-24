# Dockwatch

Painel Docker self-hosted: login único, containers, stacks, segurança, deploy e MCP.

Sem SaaS, sem Postgres, sem agent. Na VPS o container fala **direto** com o Docker Engine via socket.

---

## Visão geral

```
Browser → UI (:5173 dev / :8083 prod)
              ↓ /api/*
         dockpanel API
              ↓
    /var/run/docker.sock  (ou ssh:// outro host)
    + /data  (métricas/alertas JSONL)
```

| Camada | Função |
|--------|--------|
| **Frontend** | Login + painel Docker |
| **Backend** | API REST + WebSocket (Go) |
| **Auth** | JWT cookie; admin via `DOCKPANEL_ADMIN_*` |
| **MCP** | Ferramentas stdio (Cursor / Claude) |

---

## Produção (build → push → pull na VPS)

No PC (raiz do repo):

```powershell
docker build -t redecoop/dockwatch:1.0.0 .
docker push redecoop/dockwatch:1.0.0
```

No Portainer / VPS: Environment com admin/JWT (sem VERSION — a tag está fixa no compose).

```bash
docker compose pull
docker compose up -d
```

Abra **http://SEU_IP:8083** → login com o admin do Environment.

O compose monta `/var/run/docker.sock` (Docker do host) + volume `dockpanel-data`.

---

## Desenvolvimento local

```powershell
cp .env.example .env
# ajuste admin + DOCKPANEL_HOSTS se precisar

cd backend
go run ./cmd/server

cd frontend
npm install
npm run dev
```

- API: http://localhost:8080  
- UI: http://localhost:5173 → `/login`

```powershell
cd backend
go test ./...
```

---

## Variáveis

| Variável | Descrição |
|----------|-----------|
| `DOCKPANEL_HOSTS` | JSON hosts; `dockerHost` vazio = socket local |
| `DOCKPANEL_JWT_SECRET` | Segredo JWT (obrigatório) |
| `DOCKPANEL_ADMIN_*` | Login único |
| `DOCKPANEL_DATA_DIR` | JSONL métricas/alertas |
| `DOCKPANEL_COMPOSE_PATH` | Pasta com `docker-compose.yml` (Deploy) |
| `DOCKPANEL_BACKUP_DIR` | Destino de backup de volumes |

No Portainer: cole o conteúdo do `.env` em **Environment** da stack (não coloque senha/JWT no YAML do compose).

> **Nunca commite:** `.env`, `.cursor/mcp.json`

---

## Estrutura

```
backend/cmd/server/    # API
backend/cmd/mcp/       # MCP
frontend/src/pages/    # login + painel
docker-compose.yml     # produção (socket)
```

---

## MCP

```powershell
cd backend
go build -o dockpanel-mcp.exe ./cmd/mcp
```

Copie `.cursor/mcp.json.example` → `.cursor/mcp.json` e ajuste `DOCKPANEL_HOSTS`.

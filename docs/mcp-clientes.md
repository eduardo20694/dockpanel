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

## Stripe (teste)

```env
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
STRIPE_PRICE_INDIVIDUAL=price_...
STRIPE_PRICE_SEAT=price_...
```

O webhook é `POST /api/billing/webhook`. Sem essas variáveis o checkout responde que a cobrança não está configurada; outro provedor entra no lugar da interface `billing.Provider`.

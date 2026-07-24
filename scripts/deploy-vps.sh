#!/usr/bin/env bash
# Uso: ./scripts/deploy-vps.sh   (na VPS, depois do push da imagem)
# Expecta .env com VERSION=x.x.x e admin/JWT.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "Crie .env (veja .env.vps.example) ou cole as vars no Portainer."
  exit 1
fi

# shellcheck disable=SC1091
set -a && source .env && set +a

echo "==> Pull redecoop/dockwatch:1.0.0"
docker compose pull

echo "==> Up"
docker compose up -d

echo ""
docker compose ps
IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
echo "Painel: http://${IP:-localhost}:8083"
echo "Logs:  docker compose logs -f dockpanel"

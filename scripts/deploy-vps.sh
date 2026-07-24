#!/usr/bin/env bash
# Pull + up na VPS (imagem já publicada: redecoop/dockwatch:1.0.0).
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "Crie .env a partir de .env.vps.example (ou cole vars no Portainer)."
  exit 1
fi

echo "==> Pull redecoop/dockwatch:1.0.0"
docker compose pull

echo "==> Up"
docker compose up -d

echo ""
docker compose ps
IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
echo "Painel: http://${IP:-localhost}:8083"
echo "Logs:  docker compose logs -f dockwatch"

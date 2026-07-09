#!/usr/bin/env bash
# Deploy dockpanel na VPS (rode na pasta do projeto: /root/dockpanel)
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "Erro: crie .env a partir de .env.vps.example"
  exit 1
fi

echo "==> Build e subida dos containers"
docker compose pull 2>/dev/null || true
docker compose up -d --build

echo ""
echo "==> Status"
docker compose ps

echo ""
echo "Painel: http://$(hostname -I | awk '{print $1}'):8083"
echo "Logs: docker compose logs -f dockpanel"

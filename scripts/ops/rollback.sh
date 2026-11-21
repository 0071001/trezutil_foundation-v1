#!/usr/bin/env bash
# Bring back the image that ran before the last scripts/ops/deploy.sh.
#
# Usage, from the repository root on the server:
#   scripts/ops/rollback.sh
#
# The code goes back; the database does not. Migrations are forward-only and additive in this repository, so the
# previous image normally runs on the migrated schema. If it does not, restore the backup deploy.sh took just before
# migrating (backups/trezutil-<timestamp>.dump) — a deliberate, separate step described in
# docs/deployment/VPS_DEPLOYMENT.md, never done automatically here.
set -euo pipefail

cd "$(dirname "$0")/../.."
ENV_FILE="${ENV_FILE:-deployments/docker/app.env}"
COMPOSE=(docker compose -p trezutil -f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml --env-file "$ENV_FILE")
port=$(grep -E '^GATEWAY_LOCAL_PORT=' "$ENV_FILE" | tail -1 | cut -d= -f2- || true); port=${port:-18080}

docker image inspect trezutil:previous >/dev/null 2>&1 || { echo "ERROR: no trezutil:previous image: nothing to roll back to." >&2; exit 1; }

read -r -p "Replace the running Trezutil image with the previous one? [y/N] " answer
[ "$answer" = "y" ] || [ "$answer" = "Y" ] || { echo "Cancelled."; exit 0; }

docker image tag trezutil:latest trezutil:failed 2>/dev/null || true
docker image tag trezutil:previous trezutil:latest
"${COMPOSE[@]}" up -d --no-build --remove-orphans

for _ in $(seq 1 40); do
  if curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
    echo "Rolled back: the previous image is running (the faulty one is kept as trezutil:failed for diagnosis)."
    exit 0
  fi
  sleep 3
done
echo "ERROR: the previous image is not healthy either. Check: ${COMPOSE[*]} logs --tail 100" >&2
exit 1

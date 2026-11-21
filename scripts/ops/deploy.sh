#!/usr/bin/env bash
# Deploy (or update) Trezutil on a VPS shared with other software (Odoo), behind the host's nginx.
#
# Usage, from the repository root on the server:
#   scripts/ops/deploy.sh            # checks, backup, build, migrate, start, health check
#   scripts/ops/deploy.sh --check    # the checks only: changes nothing
#
# What it never does: touch Odoo, its database, its ports (8069, 8072) or the host's PostgreSQL (5432); bind 80/443
# (nginx keeps them, see deployments/nginx/trezutil.conf); drop or roll back data. Migrations are forward-only and run
# after a verified backup; scripts/ops/rollback.sh brings the previous image back.
set -euo pipefail

cd "$(dirname "$0")/../.."
ENV_FILE="${ENV_FILE:-deployments/docker/app.env}"
PROJECT="trezutil"
COMPOSE=(docker compose -p "$PROJECT" -f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml --env-file "$ENV_FILE")
CHECK_ONLY=0
[ "${1:-}" = "--check" ] && CHECK_ONLY=1

fail() { echo "ERROR: $*" >&2; exit 1; }
warn() { echo "WARNING: $*" >&2; }
envget() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2- ; }

echo "== Pre-flight checks"
command -v docker >/dev/null || fail "docker is not installed"
docker compose version >/dev/null || fail "the docker compose plugin is missing"
[ -f "$ENV_FILE" ] || fail "$ENV_FILE is missing (copy deployments/docker/app.env.example and fill in the secrets)"
perm=$(stat -c %a "$ENV_FILE" 2>/dev/null || echo 600)
[ "${perm: -1}" = "0" ] || warn "$ENV_FILE is readable by other users (chmod 600 $ENV_FILE)"

app_env=$(envget APP_ENV || true)
[ "$app_env" = "production" ] || [ "$app_env" = "staging" ] || fail "APP_ENV must be production or staging (found '${app_env:-unset}'); development logs sign-in codes"
for key in POSTGRES_PASSWORD JWT_SECRET OTP_CODE_SECRET CORS_ALLOWED_ORIGINS; do
  [ -n "$(envget "$key" || true)" ] || fail "$key is empty in $ENV_FILE"
done
[ "$(envget TRUSTED_PROXY_HOPS || true)" = "1" ] || warn "TRUSTED_PROXY_HOPS should be 1 behind nginx (per-IP limits would see nginx, not the client)"
case "$(envget CORS_ALLOWED_ORIGINS)" in *localhost*|*127.0.0.1*) warn "CORS_ALLOWED_ORIGINS still lists a local address";; esac

port=$(envget GATEWAY_LOCAL_PORT || true); port=${port:-18080}
for reserved in 80 443 5432 8069 8072; do
  [ "$port" != "$reserved" ] || fail "GATEWAY_LOCAL_PORT=$port is reserved (nginx, PostgreSQL or Odoo)"
done
if command -v ss >/dev/null; then
  holder=$(ss -ltnp "sport = :$port" 2>/dev/null | tail -n +2 || true)
  if [ -n "$holder" ] && ! echo "$holder" | grep -q docker; then fail "port $port is already used by another program: $holder"; fi
fi

if command -v free >/dev/null; then
  avail=$(free -m | awk '/^Mem:/ {print $7}')
  [ "${avail:-0}" -ge 1500 ] || warn "only ${avail} MB of memory available; Trezutil needs about 1.2 GB idle (caps: 3.6 GB)"
fi
disk=$(df -Pm . | awk 'NR==2 {print $4}')
[ "${disk:-0}" -ge 5000 ] || fail "only ${disk} MB free on disk; keep at least 5 GB for images, backups and the database"

"${COMPOSE[@]}" config -q || fail "the compose configuration is invalid"
echo "Checks passed (gateway on 127.0.0.1:$port)."
[ "$CHECK_ONLY" = "1" ] && exit 0

echo "== Backup before migrating"
if docker ps --format '{{.Names}}' | grep -qx "${PROJECT}-postgres-1"; then
  PG_CONTAINER="${PROJECT}-postgres-1" \
  POSTGRES_USER="$(envget POSTGRES_USER || echo trezutil)" POSTGRES_DB="$(envget POSTGRES_DB || echo trezutil)" \
  BACKUP_DIR="${BACKUP_DIR:-./backups}" scripts/ops/backup.sh
else
  echo "No running database yet (first deployment): nothing to back up."
fi

echo "== Keep the running image for a rollback"
if docker image inspect trezutil:latest >/dev/null 2>&1; then
  docker image tag trezutil:latest trezutil:previous
  echo "trezutil:latest saved as trezutil:previous"
fi

echo "== Build"
"${COMPOSE[@]}" build

echo "== Migrate and start (the migrate service runs before the others)"
"${COMPOSE[@]}" up -d --remove-orphans

echo "== Health check"
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null 2>&1; then
    echo "Trezutil is up on 127.0.0.1:$port. Reload nginx if this is the first deployment: nginx -t && systemctl reload nginx"
    "${COMPOSE[@]}" ps
    exit 0
  fi
  sleep 3
done
"${COMPOSE[@]}" ps
"${COMPOSE[@]}" logs --tail 80 migrate api-gateway conversation-service identity-service || true
fail "the gateway did not become healthy in 3 minutes. Read the logs above; scripts/ops/rollback.sh restores the previous image."

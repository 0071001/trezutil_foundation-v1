#!/usr/bin/env bash
# Replace the Anthropic (Claude) API key of the running deployment, safely, and turn the AI on if it was off.
#
# The key is read from standard input (never from the command line, so it is not in any shell history or process
# list). It is checked with Anthropic first, against the model in AI_MODEL; a refused key changes nothing. Then
# app.env is updated (ANTHROPIC_API_KEY, and AI_PROVIDER=anthropic when AI_PROVIDER is empty) and only the services
# that read them (conversation-service, and identity-service for the console's "AI configured" flag) are recreated.
# From a Windows PC with the trezutil-vps SSH alias:
#
#   $s = Read-Host "Clé API Claude" -AsSecureString
#   [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s)) | ssh trezutil-vps bash trezutil/scripts/ops/set-anthropic-key.sh
set -euo pipefail

cd "$(dirname "$0")/../.."
ENV_FILE="${ENV_FILE:-deployments/docker/app.env}"
COMPOSE=(docker compose -p trezutil -f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml --env-file "$ENV_FILE")
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE is missing" >&2; exit 1; }

IFS= read -r key || true
key="${key//$'\r'/}"
key="${key//[[:space:]]/}"
[ -n "$key" ] || { echo "ERROR: no key received on standard input; nothing changed." >&2; exit 1; }

get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2-; }
model="$(get AI_MODEL)"; model="${model:-claude-opus-5}"

echo "== Checking the key with Anthropic (model $model)"
code="$(curl -s -o /tmp/anthropic-check.$$ -w '%{http_code}' --max-time 20 \
  -H "x-api-key: $key" -H "anthropic-version: 2023-06-01" "https://api.anthropic.com/v1/models/$model" || echo 000)"
answer="$(cat /tmp/anthropic-check.$$ 2>/dev/null || true)"; rm -f /tmp/anthropic-check.$$
case "$code" in
  200) echo "Accepted: the key can use $model.";;
  404) echo "ERROR: the key works but the model '$model' (AI_MODEL) is unknown to it; nothing changed." >&2; exit 1;;
  401|403) echo "ERROR: Anthropic refused this key (HTTP $code); nothing changed." >&2
           printf '%s\n' "$answer" | head -c 300 >&2; echo >&2; exit 1;;
  *) echo "ERROR: Anthropic could not be reached or answered HTTP $code; nothing changed." >&2
     printf '%s\n' "$answer" | head -c 300 >&2; echo >&2; exit 1;;
esac

echo "== Updating $ENV_FILE"
provider="$(get AI_PROVIDER)"
tmp="$(mktemp)"
grep -vE '^(ANTHROPIC_API_KEY|AI_PROVIDER)=' "$ENV_FILE" > "$tmp"
printf 'ANTHROPIC_API_KEY=%s\n' "$key" >> "$tmp"
if [ -z "$provider" ]; then
  printf 'AI_PROVIDER=anthropic\n' >> "$tmp"; echo "AI_PROVIDER was empty: set to anthropic."
else
  printf 'AI_PROVIDER=%s\n' "$provider" >> "$tmp"; echo "AI_PROVIDER kept: $provider"
fi
cat "$tmp" > "$ENV_FILE"
rm -f "$tmp"
chmod 600 "$ENV_FILE"
unset key answer

echo "== Recreating conversation-service and identity-service (nothing else restarts)"
"${COMPOSE[@]}" up -d --no-build --no-deps conversation-service identity-service
port="$(get GATEWAY_LOCAL_PORT)"; port="${port:-18080}"
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null 2>&1; then echo "Done: the new key is in use."; exit 0; fi
  sleep 2
done
echo "WARNING: the services were recreated but /readyz is not answering yet; check: ${COMPOSE[*]} logs --tail 50 conversation-service" >&2
exit 1

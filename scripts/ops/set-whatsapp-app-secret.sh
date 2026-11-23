#!/usr/bin/env bash
# Install the Meta App Secret of the WhatsApp app, safely. Trezutil uses it to check that every webhook call really
# comes from Meta (X-Hub-Signature-256); without it, every incoming WhatsApp message is refused (401).
#
# The secret is read from standard input (never from the command line, so it is not in any shell history or process
# list). It is checked with Meta first, against the app of the configured WhatsApp token; a wrong secret changes
# nothing. Then app.env is updated and only conversation-service (which receives the webhook) is recreated.
# From a Windows PC with the trezutil-vps SSH alias:
#
#   $s = Read-Host "App Secret Meta" -AsSecureString
#   [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s)) | ssh trezutil-vps bash trezutil/scripts/ops/set-whatsapp-app-secret.sh
set -euo pipefail

cd "$(dirname "$0")/../.."
ENV_FILE="${ENV_FILE:-deployments/docker/app.env}"
COMPOSE=(docker compose -p trezutil -f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml --env-file "$ENV_FILE")
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE is missing" >&2; exit 1; }

IFS= read -r secret || true
secret="${secret//$'\r'/}"
secret="${secret//[[:space:]]/}"
[ -n "$secret" ] || { echo "ERROR: no secret received on standard input; nothing changed." >&2; exit 1; }
[[ "$secret" =~ ^[0-9a-fA-F]{32}$ ]] || { echo "ERROR: a Meta App Secret is 32 hexadecimal characters; this is not one. Nothing changed." >&2; exit 1; }

get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2-; }
base="$(get WHATSAPP_API_BASE_URL)"; base="${base:-https://graph.facebook.com/v20.0}"
token="$(get WHATSAPP_ACCESS_TOKEN)"
[ -n "$token" ] || { echo "ERROR: WHATSAPP_ACCESS_TOKEN is empty; install the token first. Nothing changed." >&2; exit 1; }

echo "== Checking the secret with Meta"
app_id="$(curl -s --max-time 20 -H "Authorization: Bearer $token" "$base/debug_token?input_token=$token" | grep -oE '"app_id":"[0-9]+"' | grep -oE '[0-9]+' | head -1 || true)"
[ -n "$app_id" ] || { echo "ERROR: could not read the app of the WhatsApp token from Meta; nothing changed." >&2; exit 1; }
answer="$(curl -s --max-time 20 "$base/debug_token?input_token=$token&access_token=$app_id%7C$secret")"
if ! printf '%s' "$answer" | grep -q '"is_valid":true'; then
  echo "ERROR: Meta refused this secret for the app of the WhatsApp token; nothing changed." >&2
  printf '%s\n' "$answer" | sed -E 's/"fbtrace_id":"[^"]*"//' | head -c 300 >&2; echo >&2
  exit 1
fi
echo "Accepted: it is the secret of the app that owns the WhatsApp token."

echo "== Updating $ENV_FILE"
tmp="$(mktemp)"
grep -v '^WHATSAPP_APP_SECRET=' "$ENV_FILE" > "$tmp"
printf 'WHATSAPP_APP_SECRET=%s\n' "$secret" >> "$tmp"
cat "$tmp" > "$ENV_FILE"
rm -f "$tmp"
chmod 600 "$ENV_FILE"
unset secret token answer

echo "== Recreating conversation-service (nothing else restarts)"
"${COMPOSE[@]}" up -d --no-build --no-deps conversation-service
port="$(get GATEWAY_LOCAL_PORT)"; port="${port:-18080}"
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null 2>&1; then echo "Done: incoming WhatsApp messages are now checked with this secret."; exit 0; fi
  sleep 2
done
echo "WARNING: conversation-service was recreated but /readyz is not answering yet; check: ${COMPOSE[*]} logs --tail 50 conversation-service" >&2
exit 1

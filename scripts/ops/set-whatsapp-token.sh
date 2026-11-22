#!/usr/bin/env bash
# Replace the WhatsApp access token of the running deployment, safely.
#
# The token is read from standard input (never from the command line, so it is not in any shell history or process
# list). It is checked with Meta first; an invalid token changes nothing. Then app.env is updated and only the two
# services that use it (identity, conversation) are recreated. From a Windows PC with the trezutil-vps SSH alias:
#
#   $s = Read-Host "Token WhatsApp" -AsSecureString
#   [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s)) | ssh trezutil-vps bash trezutil/scripts/ops/set-whatsapp-token.sh
set -euo pipefail

cd "$(dirname "$0")/../.."
ENV_FILE="${ENV_FILE:-deployments/docker/app.env}"
COMPOSE=(docker compose -p trezutil -f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml --env-file "$ENV_FILE")
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE is missing" >&2; exit 1; }

IFS= read -r token || true
token="${token//$'\r'/}"
token="${token//[[:space:]]/}"
[ -n "$token" ] || { echo "ERROR: no token received on standard input; nothing changed." >&2; exit 1; }

get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2-; }
base="$(get WHATSAPP_API_BASE_URL)"; base="${base:-https://graph.facebook.com/v20.0}"
phone_id="$(get WHATSAPP_PHONE_NUMBER_ID)"
[ -n "$phone_id" ] || { echo "ERROR: WHATSAPP_PHONE_NUMBER_ID is empty in $ENV_FILE; nothing changed." >&2; exit 1; }

echo "== Checking the token with Meta"
auth=(-H "Authorization: Bearer $token")
answer="$(curl -s --max-time 20 "${auth[@]}" "$base/$phone_id?fields=display_phone_number,verified_name")"
if ! printf '%s' "$answer" | grep -q '"display_phone_number"'; then
  echo "ERROR: Meta refused this token for phone number id $phone_id; nothing changed." >&2
  printf '%s\n' "$answer" | sed -E 's/"fbtrace_id":"[^"]*"//' | head -c 400 >&2; echo >&2
  exit 1
fi
echo "Accepted for: $(printf '%s' "$answer" | grep -o '"display_phone_number":"[^"]*"' | cut -d'"' -f4) ($(printf '%s' "$answer" | grep -o '"verified_name":"[^"]*"' | cut -d'"' -f4))"
expiry="$(curl -s --max-time 20 "$base/debug_token?input_token=$token&access_token=$token" | grep -o '"expires_at":[0-9]*' | cut -d: -f2 || true)"
case "$expiry" in
  "") echo "Expiry: unknown (Meta did not say)";;
  0)  echo "Expiry: never (permanent token)";;
  *)  echo "Expiry: $(date -d "@$expiry" '+%Y-%m-%d %H:%M %Z' 2>/dev/null || echo "$expiry") — a temporary token: it will stop working then";;
esac

echo "== Updating $ENV_FILE"
tmp="$(mktemp)"
grep -v '^WHATSAPP_ACCESS_TOKEN=' "$ENV_FILE" > "$tmp"
printf 'WHATSAPP_ACCESS_TOKEN=%s\n' "$token" >> "$tmp"
cat "$tmp" > "$ENV_FILE"
rm -f "$tmp"
chmod 600 "$ENV_FILE"
unset token answer

echo "== Recreating identity-service and conversation-service (nothing else restarts)"
"${COMPOSE[@]}" up -d --no-build --no-deps identity-service conversation-service
port="$(get GATEWAY_LOCAL_PORT)"; port="${port:-18080}"
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null 2>&1; then echo "Done: the new token is in use."; exit 0; fi
  sleep 2
done
echo "WARNING: the services were recreated but /readyz is not answering yet; check: ${COMPOSE[*]} logs --tail 50 identity-service" >&2
exit 1

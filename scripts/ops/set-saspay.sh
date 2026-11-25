#!/usr/bin/env bash
# Install Trezutil's SasPay account (mobile money in DR Congo, cards in USD), safely.
#
#   set-saspay.sh key              the secret API key (sk_live_… or sk_test_…), from the SasPay dashboard, Développeur
#   set-saspay.sh webhook-secret   the signing secret shown ONCE when the webhook is created in the SasPay dashboard
#
# The secret is read from standard input (never from the command line, so it is not in any shell history or process
# list). The key is checked with SasPay first: a key SasPay refuses changes nothing. Then app.env is updated (a copy
# is kept), the Trezutil platform organisation and its 'saspay' providers row are created if missing (scope
# "platform": it collects for every organisation without a payment provider of its own), and only payment-service is
# recreated. From a Windows PC with the trezutil-vps SSH alias:
#
#   $s = Read-Host "Cle SasPay" -AsSecureString
#   [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s)) | ssh trezutil-vps bash trezutil/scripts/ops/set-saspay.sh key
set -euo pipefail

cd "$(dirname "$0")/../.."
ENV_FILE="${ENV_FILE:-deployments/docker/app.env}"
COMPOSE=(docker compose -p trezutil -f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml --env-file "$ENV_FILE")
mode="${1:-}"
[ "$mode" = key ] || [ "$mode" = webhook-secret ] || { echo "usage: $0 key|webhook-secret  (secret on standard input)" >&2; exit 2; }
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE is missing" >&2; exit 1; }

IFS= read -r secret || true
secret="${secret//$'\r'/}"
secret="${secret//[[:space:]]/}"
[ -n "$secret" ] || { echo "ERROR: nothing received on standard input; nothing changed." >&2; exit 1; }

get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2- || true; }
psql_t() { docker exec -i trezutil-postgres-1 psql -U "${POSTGRES_USER:-trezutil}" -d "${POSTGRES_DB:-trezutil}" -v ON_ERROR_STOP=1 -tA "$@"; }

if [ "$mode" = key ]; then
  case "$secret" in
    sk_live_*) base="https://api.saspay.me/api/v1" ;;
    sk_test_*) base="https://sandbox-api.saspay.me/api/v1" ;;
    *) echo "ERROR: a SasPay secret key starts with sk_live_ or sk_test_; this is not one. Nothing changed." >&2; exit 1 ;;
  esac
  echo "== Checking the key with SasPay ($base)"
  # Reading a payment that does not exist: 404 means SasPay accepted the key, 401/403 means it refused it.
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 -H "Authorization: Bearer $secret" "$base/payments/00000000-0000-0000-0000-000000000000/verify/" || true)"
  case "$code" in
    404) echo "Accepted by SasPay." ;;
    401|403) echo "ERROR: SasPay refused this key (HTTP $code; check it and its scope PAYIN or BOTH). Nothing changed." >&2; exit 1 ;;
    *) echo "ERROR: unexpected answer from SasPay (HTTP $code); nothing changed. Try again later." >&2; exit 1 ;;
  esac
  var=SASPAY_SECRET_KEY
else
  [ "${#secret}" -ge 16 ] || { echo "ERROR: this does not look like a webhook signing secret; nothing changed." >&2; exit 1; }
  var=SASPAY_WEBHOOK_SECRET
fi

echo "== Updating $ENV_FILE (copy: $ENV_FILE.bak-saspay-$(date +%Y%m%d%H%M%S))"
cp -p "$ENV_FILE" "$ENV_FILE.bak-saspay-$(date +%Y%m%d%H%M%S)"
tmp="$(mktemp)"
grep -v "^$var=" "$ENV_FILE" > "$tmp" || true
printf '%s=%s\n' "$var" "$secret" >> "$tmp"
cat "$tmp" > "$ENV_FILE"
rm -f "$tmp"
chmod 600 "$ENV_FILE"
unset secret

if [ "$mode" = key ]; then
  echo "== Trezutil platform organisation and its SasPay provider (created only if missing)"
  [ "$base" = "https://api.saspay.me/api/v1" ] && base_setting="" || base_setting=",\"base_url\":\"$base\""
  psql_t <<SQL
INSERT INTO organizations (type, legal_name, display_name, phone, country, status, verification_status)
SELECT 'company', 'Trezutil', 'Trezutil', '', 'CD', 'active', 'verified'
WHERE NOT EXISTS (SELECT 1 FROM providers WHERE code = 'saspay')
  AND NOT EXISTS (SELECT 1 FROM organizations WHERE legal_name = 'Trezutil' AND display_name = 'Trezutil');
INSERT INTO providers (organization_id, code, name, type, status, capabilities, configuration, secret_ref, priority, health_status)
SELECT o.id, 'saspay', 'SasPay (Trezutil)', 'payment', 'ACTIVE', '{PAYMENT}',
       '{"settings":{"scope":"platform","country":"CD","return_url":"https://trezutil.com/","webhook_secret_ref":"env:SASPAY_WEBHOOK_SECRET"$base_setting}}',
       'env:SASPAY_SECRET_KEY', 10, 'UNKNOWN'
FROM organizations o
WHERE o.legal_name = 'Trezutil' AND o.display_name = 'Trezutil'
  AND NOT EXISTS (SELECT 1 FROM providers WHERE code = 'saspay')
ORDER BY o.created_at LIMIT 1;
SQL
  org="$(psql_t -c "SELECT organization_id FROM providers WHERE code = 'saspay' ORDER BY created_at LIMIT 1")"
  echo "Platform organisation: $org"
  echo "Webhook URL to enter in the SasPay dashboard (Développeur > Webhooks):"
  echo "  https://trezutil.com/api/v1/webhooks/payments/saspay?organization_id=$org"
fi

echo "== Recreating payment-service (nothing else restarts)"
"${COMPOSE[@]}" up -d --no-build --no-deps payment-service
for _ in $(seq 1 30); do
  if docker exec trezutil-payment-service-1 wget -qO- http://127.0.0.1:8083/readyz >/dev/null 2>&1 || \
     "${COMPOSE[@]}" ps payment-service 2>/dev/null | grep -q "Up"; then
    sleep 3
    if "${COMPOSE[@]}" logs --since 1m payment-service 2>/dev/null | grep -q "SasPay payment providers registered"; then
      echo "Done: SasPay is registered in payment-service."
    else
      echo "payment-service is up; SasPay registration not seen in its logs yet (it reloads providers every minute)."
    fi
    exit 0
  fi
  sleep 2
done
echo "WARNING: payment-service was recreated but is not up yet; check: ${COMPOSE[*]} logs --tail 50 payment-service" >&2
exit 1

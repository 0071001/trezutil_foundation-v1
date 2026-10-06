# Trezutil — Operations runbook

*Written 2026-10-05.* What to do to run Trezutil in production, day after day. The first deployment is in [PRODUCTION_DEPLOYMENT.md](PRODUCTION_DEPLOYMENT.md); the alerts and their first actions are in [HARDENING_PASS_1.md](HARDENING_PASS_1.md) §4. This page is the part in between: routine, backups, incidents, secrets.

## 1. What runs, and what matters

| Piece | Holds | If it is lost |
|---|---|---|
| **PostgreSQL** (`postgres_data`) | everything: accounts, orders, bookings, payments, conversations, site content | **this is the only thing to back up** |
| OpenSearch (`opensearch_data`) | the search index, rebuilt from PostgreSQL | delete the index and restart `search-service`: it rebuilds (minutes) |
| Caddy (`caddy_data`) | the TLS certificates | it asks Let's Encrypt for new ones (rate-limited: keep the volume) |
| Prometheus, Alertmanager | metrics history, alert state | nothing customer-facing |
| The image (`trezutil:<version>`) | the services, migrations and web app | rebuild it from the tag |

The **audit trail** (`audit_events`) is a table in PostgreSQL: it is in the nightly backup and nothing prunes it. Decide a retention before it grows (at one event per order or booking step it is small; archive with a dated `COPY … TO` rather than `DELETE`, which a trigger refuses).

The stack, from the repository root:

```sh
# everything: application + TLS + monitoring
docker compose -f deployments/docker/docker-compose.app.yml \
               -f deployments/docker/docker-compose.proxy.yml \
               -f deployments/docker/docker-compose.monitoring.yml \
               --env-file deployments/docker/app.env up -d
```

Only Caddy publishes ports (80 and 443). The gateway, the services, PostgreSQL, OpenSearch, Prometheus and Alertmanager are on the private network: reach them from the host with `docker exec` or an SSH tunnel.

## 2. Deploying a new version

1. **Build** the image from the tag: `docker build -f deployments/docker/Dockerfile -t trezutil:<version> .` (pass the `VITE_*` build arguments of [PRODUCTION_DEPLOYMENT.md](PRODUCTION_DEPLOYMENT.md) §2: the WhatsApp number, the store pages; leave `VITE_CHANNEL_PLACEHOLDERS` empty).
2. **Back up first**: `scripts/ops/backup.sh` (§3). A migration is forward-only.
3. **Migrate**: `docker compose … run --rm migrate`. It is idempotent: running it twice does nothing the second time.
4. **Roll the services** (`up -d` with the new image). The gateway's `/readyz` answers 200 when every upstream is up.
5. **Check**: `https://<host>/readyz` is `ready`; sign in; open a listing; the alert list is quiet for ten minutes; the logs show no `csp_violation` you did not expect.
6. **Roll back** by deploying the previous image tag. Migrations are additive, so the old image runs on the new schema. If a migration itself must be undone, restore the backup of step 2 (§3).

## 3. Backups

PostgreSQL is backed up with `scripts/ops/backup.sh`: a compressed custom-format dump, read back with `pg_restore --list` (so a truncated archive fails at once), checksummed, and old ones pruned (`BACKUP_KEEP_DAYS`, default 14).

```sh
# on the host, from cron (every night at 02:30), with the database container or DATABASE_URL set:
30 2 * * *  cd /opt/trezutil && PG_CONTAINER=trezutil-postgres-1 BACKUP_DIR=/var/backups/trezutil sh scripts/ops/backup.sh
```

**A backup that stays on the same disk is not a backup.** Copy `BACKUP_DIR` off the host after every run (object storage with versioning, or another machine). Decide the **recovery point** you accept: with a nightly backup, up to a day of orders can be lost. If that is too much, use the provider's point-in-time recovery on a managed database, or run the script more often.

### Rehearse the restore (at least every quarter, and before launch)

```sh
sh scripts/ops/restore-check.sh /var/backups/trezutil/trezutil-<stamp>.dump --expect-equal
```

It restores the dump into a scratch database next to the live one (never touching it), compares the row counts of `users`, `organizations`, `listings`, `orders`, `reservations`, `conversations` and `schema_migrations` and the booking exclusion constraint, then drops the scratch database. It fails on a changed checksum, a corrupt archive, or a live database that holds fewer rows than the backup.

*Rehearsed on 2026-10-05 on the staging stack:* a 0.55 MB dump restored cleanly and matched on every table; a truncated archive was rejected (exit 1) and an altered one failed its checksum (exit 3); the scratch database was removed each time.

### Restoring for real

1. Stop the writers: `docker compose … stop api-gateway order-service payment-service conversation-service identity-service search-service`.
2. Restore into the live database (or a new one): `pg_restore --clean --if-exists --no-owner -d <database> <dump>` (inside the container with `docker exec -i`).
3. Start the services; `migrate` first if the dump is older than the image.
4. Rebuild the index: delete the OpenSearch index and restart `search-service` (it rebuilds on start, then hourly).
5. Check `/readyz`, sign in, open an order and a booking.

## 4. Alerts

Prometheus evaluates the rules of `infrastructure/prometheus/alerts.yml`; Alertmanager sends them (`infrastructure/prometheus/alertmanager.yml`):

| Severity | Goes to | Repeats |
|---|---|---|
| `page` (5xx rate, payment failures, webhook failures, not ready, service down) | `ALERT_WEBHOOK_PAGE_URL`: whoever is on call | every hour until resolved |
| `ticket` (reservation errors, provider failures) | `ALERT_WEBHOOK_TICKET_URL`: the team channel | every 12 hours |

The destinations are web addresses you choose (a chat bridge, an incident service). The routing is tested: a `page` and a `ticket` alert fired into a real Alertmanager reached their own address and no other. When a service is down, the 5xx, not-ready and provider alerts it causes are held back.

**Know that monitoring itself is up.** Prometheus cannot alert about its own death. Add an external check that calls `https://<host>/readyz` every minute (an uptime service) and pages when it fails: that is the one alert that does not depend on this stack.

## 5. Incidents

| Symptom | First look | Usual cause and fix |
|---|---|---|
| Nobody can sign in | identity-service logs: `otp delivery failed` | **the WhatsApp access token expired** (Meta error 190; the staging token did on 2026-09-30) or the approved template was disabled: renew the token in the Meta console, update `WHATSAPP_ACCESS_TOKEN`, restart `identity-service` |
| Payments fail | `event=payment_failed` (stage, reason) | wrong or rotated `STRIPE_SECRET_KEY`, or the provider row is inactive; check Stripe's dashboard |
| Orders stay unpaid after a payment | `event=webhook_failed`, `WEBHOOK_SIGNATURE_INVALID` | the webhook secret differs from Stripe's: fix `STRIPE_WEBHOOK_SECRET`; replay the missed events from Stripe's dashboard (processing is idempotent) |
| Search returns nothing | `search-service` logs; `curl opensearch:9200/_cluster/health` | OpenSearch down or the index lost: restart it, then restart `search-service` to resync |
| The site loads blank or a payment form does not appear | the browser console, and `event=csp_violation` in the gateway logs | a new third-party script or frame the policy does not allow: add it to `contentSecurityPolicy` in `internal/gateway/gateway.go` (or set `CSP_MODE=report-only` while you fix it) |
| `/admin` answers 403 | Caddy | the address is not in `ADMIN_ALLOW_IPS`: add it (a VPN or office address), restart Caddy |
| The Audit page is empty though people are working | `event=audit_store_failed` in the order, conversation or identity logs | the audit table could not be written (database unreachable, migration `000025` not applied): the actions themselves succeeded and each still has its `event=audit` log line; fix the database, then the page fills from new actions (events missed in between exist only in the logs) |
| A seller or customer must be locked out now | the console, *Users* | an administrator suspends the account with a reason (it cannot sign in or refresh; a session already open lasts up to 15 minutes, `JWT_ACCESS_TTL`). Staff accounts are not suspended there: use `trezutil-admin`, or change `JWT_SECRET` to end every session |

## 6. Secrets

Never commit them; inject them from the host's secret store. Rotating:

| Secret | Effect of changing it | How |
|---|---|---|
| `JWT_SECRET` | every session ends: people sign in again | change it everywhere at once, restart all services |
| `OTP_CODE_SECRET` | codes in flight stop working (they last minutes) | change, restart `identity-service` |
| `STRIPE_*`, `WHATSAPP_*`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY` | none beyond the provider | change, restart the service that reads it |
| `METRICS_TOKEN` | Prometheus loses the gateway until the overlay is recreated | change, `up -d` the monitoring overlay |
| Database password | all services reconnect | change in PostgreSQL and in `app.env`, restart everything |
| Staff passwords | that person signs in again | `trezutil-admin set-password -email …` (the person chooses it) |

## 7. Security hygiene

- The **administration console and the staff sign-in** are reachable only from `ADMIN_ALLOW_IPS` at the proxy. Do not widen it to the whole internet.
- Change the **shared administrator password** that was used during development, and give each staff member their own account.
- The gateway sends a **Content-Security-Policy** (`CSP_MODE`: `report-only` by default, `enforce` on staging). Move production to `enforce` after a week with no unexpected `csp_violation` and one real payment made under it (the Stripe form is not exercised by the automated checks, which have no Stripe account).
- `/readyz` is public but says only `ready` or `degraded`; the detail needs `METRICS_TOKEN`.

## 8. Before opening to customers: the operations part of the checklist

- [ ] The domain's DNS points at the host; `https://<host>` shows a valid certificate; `http://` redirects.
- [ ] `ADMIN_ALLOW_IPS` is set; a staff member signs in from an allowed address and is refused from another.
- [ ] A backup ran from cron, was copied off the host, and `restore-check.sh --expect-equal` passed on it.
- [ ] Alertmanager's two addresses receive a test alert (`amtool alert add …` or the routing test in [HARDENING_PASS_1.md](HARDENING_PASS_1.md)).
- [ ] An external uptime check calls `/readyz`.
- [ ] `VITE_CHANNEL_PLACEHOLDERS` is empty; the WhatsApp number and the store pages are the real ones.
- [ ] `docs/legal/LEGAL_CHECKLIST.md` is done and `LEGAL.reviewed` is `true`.

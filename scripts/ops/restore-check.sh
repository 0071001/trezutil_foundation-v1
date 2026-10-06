#!/usr/bin/env sh
# Rehearse a restore: load a backup into a scratch database and compare it with the live one.
#
# A backup nobody has restored is a hope, not a backup. This restores the dump into a throwaway
# database next to the real one (never touching it), compares the row counts of the tables that
# matter, and drops the scratch database.
#
# Usage:   scripts/ops/restore-check.sh backups/trezutil-20261005T120000Z.dump [--expect-equal]
#
#   --expect-equal   for a rehearsal right after a backup: every count must be the same
#                    (otherwise the live database may only hold MORE rows than the backup, as it
#                    keeps taking orders; fewer rows than the backup means data was lost)
#
# Where the database is: the same variables as backup.sh (DATABASE_URL, or PG_CONTAINER with
# POSTGRES_USER / POSTGRES_DB). With DATABASE_URL the user must be allowed to create a database.
set -eu

dump="${1:?usage: restore-check.sh <backup.dump> [--expect-equal]}"
expect_equal=0
[ "${2:-}" = "--expect-equal" ] && expect_equal=1
[ -s "$dump" ] || { echo "no such backup: $dump" >&2; exit 2; }

# the checksum written by backup.sh, when it is there
if [ -f "$dump.sha256" ]; then
  ( cd "$(dirname "$dump")" && if command -v sha256sum >/dev/null 2>&1; then sha256sum -c "$(basename "$dump").sha256"; else shasum -a 256 -c "$(basename "$dump").sha256"; fi ) >/dev/null \
    || { echo "checksum mismatch: the backup file changed since it was written" >&2; exit 3; }
fi

CONTAINER="${PG_CONTAINER:-trezutil-postgres-1}"
PGUSER_NAME="${POSTGRES_USER:-trezutil}"
PGDB_NAME="${POSTGRES_DB:-trezutil}"
SCRATCH="trezutil_restore_check"
TABLES="users organizations listings orders reservations conversations schema_migrations"

if [ -n "${DATABASE_URL:-}" ]; then
  base="${DATABASE_URL%%\?*}"; query=""; [ "$base" != "$DATABASE_URL" ] && query="?${DATABASE_URL#*\?}"
  scratch_url="${base%/*}/$SCRATCH$query"
  live() { psql -At "$DATABASE_URL" -c "$1"; }
  scratch() { psql -At "$scratch_url" -c "$1"; }
  restore() { pg_restore --no-owner --dbname "$scratch_url" "$dump"; }
else
  live() { MSYS_NO_PATHCONV=1 docker exec "$CONTAINER" psql -U "$PGUSER_NAME" -d "$PGDB_NAME" -At -c "$1"; }
  scratch() { MSYS_NO_PATHCONV=1 docker exec "$CONTAINER" psql -U "$PGUSER_NAME" -d "$SCRATCH" -At -c "$1"; }
  restore() { MSYS_NO_PATHCONV=1 docker exec -i "$CONTAINER" pg_restore -U "$PGUSER_NAME" --no-owner -d "$SCRATCH" <"$dump"; }
fi

cleanup() { live "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

live "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null
live "CREATE DATABASE $SCRATCH" >/dev/null
restore

fail=0
printf '%-20s %12s %12s\n' table restored live
for table in $TABLES; do
  r="$(scratch "SELECT count(*) FROM $table")"
  l="$(live "SELECT count(*) FROM $table")"
  status="ok"
  if [ "$expect_equal" -eq 1 ] && [ "$r" != "$l" ]; then status="DIFFERENT"; fail=1; fi
  if [ "$r" -gt "$l" ]; then status="LIVE HAS FEWER ROWS THAN THE BACKUP"; fail=1; fi
  printf '%-20s %12s %12s  %s\n' "$table" "$r" "$l" "$status"
done

# the schema is restored too: the same migrations, and the constraint that stops double bookings
rm_=$(scratch "SELECT count(*) FROM pg_constraint WHERE contype = 'x'")
lm_=$(live "SELECT count(*) FROM pg_constraint WHERE contype = 'x'")
printf '%-20s %12s %12s  %s\n' "exclusion constraints" "$rm_" "$lm_" "$([ "$rm_" = "$lm_" ] && echo ok || echo DIFFERENT)"
[ "$rm_" = "$lm_" ] || fail=1

if [ "$fail" -eq 0 ]; then echo "restore rehearsal passed: $dump restores into a clean database and matches the live one"; else echo "restore rehearsal FAILED" >&2; fi
exit "$fail"

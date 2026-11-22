#!/usr/bin/env sh
# Trezutil database backup: a PostgreSQL custom-format dump, checked, checksummed and pruned.
#
# PostgreSQL is the only data that matters: OpenSearch is a projection that the search service
# rebuilds from it, and the web app, the services and the migrations are in the image.
#
# Usage:   scripts/ops/backup.sh
#
# Where the database is (one of):
#   DATABASE_URL     a managed or remote database: pg_dump runs on this host
#   PG_CONTAINER     the compose container (default trezutil-postgres-1) with POSTGRES_USER and
#                    POSTGRES_DB (defaults trezutil / trezutil): pg_dump runs inside it
# Where to write, and for how long:
#   BACKUP_DIR       default ./backups
#   BACKUP_KEEP_DAYS default 14 (older dumps and their checksums are deleted)
#
# The dump is written to a temporary name, listed back with pg_restore (a truncated or corrupt
# archive fails here, not on the day you need it), then renamed and checksummed. Copy the result
# off the host (object storage, another machine): a backup on the same disk is not a backup.
# Rehearse the restore with scripts/ops/restore-check.sh.
set -eu

BACKUP_DIR="${BACKUP_DIR:-./backups}"
KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"
CONTAINER="${PG_CONTAINER:-trezutil-postgres-1}"
PGUSER_NAME="${POSTGRES_USER:-trezutil}"
PGDB_NAME="${POSTGRES_DB:-trezutil}"

mkdir -p "$BACKUP_DIR"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="$BACKUP_DIR/trezutil-$stamp.dump"
tmp="$file.partial"
trap 'rm -f "$tmp"' EXIT INT TERM

if [ -n "${DATABASE_URL:-}" ]; then
  pg_dump --format=custom --no-owner --file "$tmp" "$DATABASE_URL"
  pg_restore --list "$tmp" >/dev/null
else
  # MSYS_NO_PATHCONV: keep Git Bash on Windows from rewriting paths given to docker
  MSYS_NO_PATHCONV=1 docker exec "$CONTAINER" pg_dump -U "$PGUSER_NAME" -d "$PGDB_NAME" --format=custom --no-owner >"$tmp"
  MSYS_NO_PATHCONV=1 docker exec -i "$CONTAINER" pg_restore --list <"$tmp" >/dev/null
fi

size="$(wc -c <"$tmp" | tr -d ' ')"
if [ "$size" -lt 1024 ]; then
  echo "backup is only $size bytes: refusing to keep it" >&2
  exit 1
fi
mv "$tmp" "$file"
trap - EXIT INT TERM

( cd "$BACKUP_DIR" && if command -v sha256sum >/dev/null 2>&1; then sha256sum "$(basename "$file")"; else shasum -a 256 "$(basename "$file")"; fi >"$(basename "$file").sha256" )

# prune what is older than KEEP_DAYS (the newest dump is never deleted)
find "$BACKUP_DIR" -maxdepth 1 -name 'trezutil-*.dump' -mtime "+$KEEP_DAYS" ! -name "$(basename "$file")" -print -delete 2>/dev/null | while read -r old; do rm -f "$old.sha256"; done || true

echo "backup written: $file ($size bytes)"
echo "$file"

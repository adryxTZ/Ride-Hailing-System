#!/bin/bash
# Restores one encrypted dump into a database. Used for the restore drill (into a
# scratch database) and for disaster recovery (into a fresh managed database).
#
#   restore.sh <object> <target-database-url>
#     <object>  path under the backup prefix, e.g. daily/db-20261023T020000Z.dump.age
#
# Needs AGE_IDENTITY_FILE: the private age key, mounted read-only for this run only.
# Never restore over the live production database: restore into a new database,
# verify, then repoint NEXA_DATABASE_URL.
set -euo pipefail

OBJECT="${1:?object name, e.g. daily/db-<stamp>.dump.age}"
TARGET="${2:?target database url}"
: "${AGE_IDENTITY_FILE:?mount the private key and set AGE_IDENTITY_FILE}" "${BACKUP_BUCKET:?}"
PREFIX="${BACKUP_PREFIX:-platform}/${BACKUP_ENV:-unknown}"
PARALLEL_JOBS=4
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "fetching $OBJECT"
rclone copyto "store:$BACKUP_BUCKET/$PREFIX/$OBJECT" "$WORK/dump.age"
age -d -i "$AGE_IDENTITY_FILE" "$WORK/dump.age" > "$WORK/dump"
# PostGIS must exist before the objects that use it.
psql "$TARGET" -v ON_ERROR_STOP=1 -qc 'CREATE EXTENSION IF NOT EXISTS postgis'
pg_restore --no-owner --no-privileges --exit-on-error --jobs="$PARALLEL_JOBS" -d "$TARGET" "$WORK/dump"
echo "restored into target; tables:"
psql "$TARGET" -qtAc "select count(*) from information_schema.tables where table_schema='public'"

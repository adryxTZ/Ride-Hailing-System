#!/bin/bash
# Encrypted logical backup of the Platform database plus the uploads and signing-key
# volumes, pushed to S3-compatible storage, with retention.
#
#   docker compose -f compose.yml -f compose.<env>.yml --profile ops run --rm backup
#
# Environment (backup.env): BACKUP_DATABASE_URL, BACKUP_AGE_RECIPIENT (public key),
# BACKUP_BUCKET, BACKUP_PREFIX, RCLONE_CONFIG_STORE_* (the "store:" remote),
# BACKUP_RETENTION_DAYS, BACKUP_WEEKLY_RETENTION_DAYS, BACKUP_ENV.
# The private age key never touches the server: a stolen server cannot read old dumps.
# Managed Postgres point-in-time recovery stays the first line; this dump is the
# off-provider copy used by the quarterly restore drill.
set -euo pipefail

: "${BACKUP_DATABASE_URL:?}" "${BACKUP_AGE_RECIPIENT:?}" "${BACKUP_BUCKET:?}"
PREFIX="${BACKUP_PREFIX:-platform}/${BACKUP_ENV:-unknown}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
WEEKLY_RETENTION_DAYS="${BACKUP_WEEKLY_RETENTION_DAYS:-90}"
SUNDAY=7                                  # date +%u
TEXTFILE_DIR="${TEXTFILE_DIR:-/textfile}" # node-exporter textfile collector (Alloy)
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log() { echo "$(date -u +%H:%M:%S) backup: $*"; }

# write_metric records the outcome for the BackupStale / BackupFailed alerts.
write_metric() {
  local ok="$1" size="${2:-0}"
  [ -d "$TEXTFILE_DIR" ] || return 0
  cat > "$TEXTFILE_DIR/backup.prom.tmp" <<PROM
# HELP platform_backup_last_run_timestamp_seconds Last backup attempt.
# TYPE platform_backup_last_run_timestamp_seconds gauge
platform_backup_last_run_timestamp_seconds $(date +%s)
# HELP platform_backup_ok 1 when the last backup uploaded successfully.
# TYPE platform_backup_ok gauge
platform_backup_ok $ok
# HELP platform_backup_size_bytes Size of the last encrypted database dump.
# TYPE platform_backup_size_bytes gauge
platform_backup_size_bytes $size
PROM
  mv "$TEXTFILE_DIR/backup.prom.tmp" "$TEXTFILE_DIR/backup.prom"
}
trap 'write_metric 0' ERR

# 1) Database: custom format (compressed, selective restore), streamed into age.
log "dumping database"
pg_dump --format=custom --no-owner --no-privileges "$BACKUP_DATABASE_URL" \
  | age -r "$BACKUP_AGE_RECIPIENT" > "$WORK/db-$STAMP.dump.age"

# 2) Files: uploaded documents and the JWT signing keys (losing keys signs everyone out).
log "archiving uploads and keys"
tar -C /data -czf - uploads keys | age -r "$BACKUP_AGE_RECIPIENT" > "$WORK/files-$STAMP.tar.gz.age"

# 3) Upload, then a Sunday copy to weekly/.
DEST="store:$BACKUP_BUCKET/$PREFIX"
log "uploading to $DEST/daily/"
rclone copy --no-traverse "$WORK" "$DEST/daily/"
if [ "$(date -u +%u)" = "$SUNDAY" ]; then
  rclone copy --no-traverse "$WORK" "$DEST/weekly/"
fi

# 4) Retention (mkdir is a no-op on object stores; it makes a first run's prune safe).
rclone mkdir "$DEST/weekly/"
rclone delete --min-age "${RETENTION_DAYS}d" "$DEST/daily/"
rclone delete --min-age "${WEEKLY_RETENTION_DAYS}d" "$DEST/weekly/"

SIZE="$(stat -c %s "$WORK/db-$STAMP.dump.age")"
write_metric 1 "$SIZE"
log "done: db-$STAMP.dump.age ($SIZE bytes), files-$STAMP.tar.gz.age"

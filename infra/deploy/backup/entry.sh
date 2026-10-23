#!/bin/bash
# Dispatches the toolbox's two jobs: "backup" (default) and "restore <object>".
set -euo pipefail
case "${1:-backup}" in
  backup)  shift || true; exec backup.sh "$@" ;;
  restore) shift; exec restore.sh "$@" ;;
  *) echo "usage: backup | restore <dump-object-name> <target-database-url>" >&2; exit 2 ;;
esac

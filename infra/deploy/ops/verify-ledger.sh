#!/bin/bash
# Daily ledger hash-chain verification (D11). Runs `nexa verify-ledger` in a one-off
# container and records the outcome for the LedgerVerifyFailed/Stale alerts through
# the node-exporter textfile collector that Alloy reads. Cron: see ops/crontab.example.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
# shellcheck disable=SC1091
source .env
TEXTFILE_DIR="${METRICS_TEXTFILE_DIR:-/var/lib/platform/metrics}"
COMPOSE=(docker compose -f compose.yml -f "compose.${DEPLOY_ENV}.yml")

if "${COMPOSE[@]}" --profile ops run --rm verify-ledger; then ok=1; else ok=0; fi
mkdir -p "$TEXTFILE_DIR"
cat > "$TEXTFILE_DIR/ledger.prom.tmp" <<PROM
# HELP platform_ledger_verify_ok 1 when the last ledger hash-chain verification passed.
# TYPE platform_ledger_verify_ok gauge
platform_ledger_verify_ok $ok
# HELP platform_ledger_verify_last_run_timestamp_seconds Last verification run.
# TYPE platform_ledger_verify_last_run_timestamp_seconds gauge
platform_ledger_verify_last_run_timestamp_seconds $(date +%s)
PROM
mv "$TEXTFILE_DIR/ledger.prom.tmp" "$TEXTFILE_DIR/ledger.prom"
[ "$ok" = 1 ]

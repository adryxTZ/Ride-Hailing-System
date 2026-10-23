#!/bin/bash
# Deploys one immutable release to this host (UAT or production). Run by the CI deploy
# job over SSH from /opt/platform, or by hand:
#
#   ./ops/deploy.sh <api-image> <admin-image>
#
# Order (D8/D11): pull -> migrate (one-off container, before any new code serves)
# -> seed reference data -> up -d --wait (healthchecks gate) -> smoke. No watchtower.
# Rollback = run this again with the previous tags (migrations are forward-only and
# must stay backward compatible with the previous release: expand, then contract).
set -euo pipefail
cd "$(dirname "$0")/.."

API_IMAGE_NEW="${1:?api image, e.g. ghcr.io/<owner>/<repo>/nexa:<sha>}"
ADMIN_IMAGE_NEW="${2:?admin image, e.g. ghcr.io/<owner>/<repo>/admin:<sha>-uat}"
WAIT_TIMEOUT_S=180

# shellcheck disable=SC1091
source .env
OVERLAY="compose.${DEPLOY_ENV:?DEPLOY_ENV missing from .env}.yml"
COMPOSE=(docker compose -f compose.yml -f "$OVERLAY")

log() { echo "$(date -u +%H:%M:%S) deploy[$DEPLOY_ENV]: $*"; }

# set_env rewrites one KEY=value line of .env (the release record on this host).
set_env() {
  local key="$1" value="$2"
  if grep -q "^${key}=" .env; then
    sed -i.bak "s#^${key}=.*#${key}=${value}#" .env && rm -f .env.bak
  else
    echo "${key}=${value}" >> .env
  fi
}

# Concurrent deploys are prevented by the CI concurrency group per environment;
# flock guards against a manual run at the same time.
exec 9>/tmp/platform-deploy.lock
if command -v flock >/dev/null; then flock -n 9 || { log "another deploy is running"; exit 1; }; fi

PREVIOUS_API="${API_IMAGE:-none}"
# The shell environment overrides .env for compose; .env is rewritten only after the
# smoke test passes, so a failed deploy leaves the last good release on record.
export API_IMAGE="$API_IMAGE_NEW" ADMIN_IMAGE="$ADMIN_IMAGE_NEW"
log "release $API_IMAGE_NEW (previous $PREVIOUS_API)"

# SKIP_PULL=1 deploys images already on this host (rehearsals, offline rollback).
if [ "${SKIP_PULL:-0}" != 1 ]; then
  log "pulling images"
  "${COMPOSE[@]}" pull api admin
fi

log "redis up (migrations do not need it, the seed may bump cache versions)"
"${COMPOSE[@]}" up -d --wait redis

log "migrating"
"${COMPOSE[@]}" --profile ops run --rm migrate
log "seeding reference data (idempotent)"
"${COMPOSE[@]}" --profile ops run --rm seed

log "starting services"
"${COMPOSE[@]}" up -d --wait --wait-timeout "$WAIT_TIMEOUT_S" --remove-orphans

log "smoke test"
PORT_SUFFIX=""
if [ "${PUBLIC_HTTPS_PORT:-443}" != 443 ]; then PORT_SUFFIX=":${PUBLIC_HTTPS_PORT}"; fi
./ops/smoke.sh "https://${API_DOMAIN}${PORT_SUFFIX}/v1" "https://${ADMIN_DOMAIN}${PORT_SUFFIX}"
set_env API_IMAGE "$API_IMAGE_NEW"
set_env ADMIN_IMAGE "$ADMIN_IMAGE_NEW"
docker image prune -f --filter "until=168h" >/dev/null
log "done"

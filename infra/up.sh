#!/bin/bash
# Builds and runs the whole platform in Docker: Postgres, Redis, migrations + seed,
# API (:8080), admin dashboard (:3000) and the mobile bundlers (:8081-8084). Phones on
# the same Wi-Fi reach the bundlers and the API through this machine's LAN address,
# found automatically.
#
#   ./infra/up.sh            build and start everything
#   ./infra/up.sh core       database, Redis, API and dashboard only (no mobile bundlers)
#   ./infra/up.sh down       stop (data kept)
#   ./infra/up.sh reset      stop and delete data
#   ./infra/up.sh staff ...  create a dashboard account, e.g.
#       ./infra/up.sh staff add 0712345678 --name "Ops" --roles "Super Admin" --email you@example.com --password '...'
#
# Ports and the compose project are variables, so a second copy can run beside the
# dev stack:  COMPOSE_PROJECT_NAME=nexa-test API_PORT=18080 ADMIN_PORT=13000 \
#             PG_PORT=15433 REDIS_PORT=16380 ./infra/up.sh core
set -euo pipefail
cd "$(dirname "$0")"

# Defaults match the developer stack documented in infra/README.md.
export API_PORT="${API_PORT:-8080}"
export ADMIN_PORT="${ADMIN_PORT:-3000}"
COMPOSE=(docker compose -f compose.yml)

# lan_ip prints this machine's Wi-Fi/LAN address (macOS first, then Linux).
lan_ip() {
  ipconfig getifaddr en0 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || true
}

# wait_http polls a URL until it answers 2xx/3xx or the attempts run out.
wait_http() {
  local url="$1" attempts="${2:-60}"
  for _ in $(seq "$attempts"); do
    curl -fsS -o /dev/null -m 2 "$url" && return 0
    sleep 1
  done
  echo "no answer from $url" >&2
  return 1
}

case "${1:-up}" in
  down)  "${COMPOSE[@]}" down; exit 0 ;;
  reset) "${COMPOSE[@]}" down -v; exit 0 ;;
  staff) shift; "${COMPOSE[@]}" run --rm --no-deps api staff "$@"; exit 0 ;;
  up|core) ;;
  *) sed -n '2,16p' "$0"; exit 2 ;;
esac

HOST_IP="${HOST_IP:-$(lan_ip)}"
export HOST_IP="${HOST_IP:-localhost}"
export PUBLIC_API_URL="${PUBLIC_API_URL:-http://localhost:${API_PORT}/v1}"
export PHONE_API_URL="${PHONE_API_URL:-http://${HOST_IP}:${API_PORT}/v1}"

"${COMPOSE[@]}" up -d --build --wait api admin
wait_http "http://localhost:${API_PORT}/v1/health"
wait_http "http://localhost:${ADMIN_PORT}/login"
if [ "${1:-up}" = "up" ]; then
  "${COMPOSE[@]}" up -d --build apps
fi

cat <<MSG

API        http://localhost:${API_PORT}/v1/health
Dashboard  http://localhost:${ADMIN_PORT}   (staff email + password; create one with: ./infra/up.sh staff add 07XXXXXXXX --roles "Super Admin" --email you@example.com --password ...)
MSG
if [ "${1:-up}" = "up" ]; then
  cat <<MSG
Rider      web http://localhost:8083   phone: open Expo Go and enter exp://${HOST_IP}:8081
Driver     web http://localhost:8084   phone: open Expo Go and enter exp://${HOST_IP}:8082
MSG
fi
echo "Logs       docker compose -f infra/compose.yml logs -f api admin"

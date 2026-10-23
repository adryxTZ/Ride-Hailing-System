#!/bin/bash
# Creates the synthetic, approved drivers the load test signs in as (one vehicle each,
# spread over the ride types). Idempotent: re-running re-provisions the same numbers.
#
#   COUNT=600 ./infra/loadtest/provision-drivers.sh
#   NEXA="docker compose -f compose.yml -f compose.uat.yml --profile ops run --rm -T seed" \
#     COUNT=600 ./provision-drivers.sh          (on the UAT host, from /opt/platform; see README)
#
# NEXA is the command that runs the backend binary: default the local build
# (backend/bin/nexa with the developer database); on UAT the API image via compose.
set -euo pipefail
COUNT="${COUNT:-600}"
PREFIX="${DRIVER_PREFIX:-+2557119}"
# NEXA may be a multi-word command; the default is one path that may contain spaces.
if [ -n "${NEXA:-}" ]; then
  read -r -a NEXA_CMD <<< "$NEXA"
else
  NEXA_CMD=("$(cd "$(dirname "$0")/../../backend" && pwd)/bin/nexa")
fi
# Vehicle classes in rotation (taxi ride types, ADR 0009).
CLASSES=(boda bajaji standard xl)

for i in $(seq 1 "$COUNT"); do
  phone="$(printf '%s%05d' "$PREFIX" "$i")"
  class="${CLASSES[$(( (i - 1) % ${#CLASSES[@]} ))]}"
  plate="$(printf 'LT%05d' "$i")"
  "${NEXA_CMD[@]}" driver provision "$phone" --service taxi --class "$class" --plate "$plate" --name "Load Driver $i" >/dev/null
  if (( i % 50 == 0 )); then echo "provisioned $i/$COUNT"; fi
done
echo "provisioned $COUNT drivers (${PREFIX}00001 ..)"

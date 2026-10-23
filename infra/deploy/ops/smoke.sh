#!/bin/bash
# Post-deploy smoke test: the public surface answers and reports the expected build.
#   ./ops/smoke.sh https://api.<domain>/v1 https://admin.<domain> [expected-version]
# Exit non-zero on the first failure, so the deploy job fails visibly.
set -euo pipefail
API="${1:?api base url ending in /v1}"
ADMIN="${2:?admin url}"
EXPECT_VERSION="${3:-}"
ATTEMPTS=30
DELAY_S=2
# SMOKE_INSECURE=1 accepts a private CA (rehearsals with Caddy's internal certificates).
CURL=(curl -fsS -m 5)
if [ "${SMOKE_INSECURE:-0}" = 1 ]; then CURL+=(-k); fi

# get_ok fetches a URL until it answers 2xx (TLS issuance can take a few seconds).
get_ok() {
  local url="$1"
  for _ in $(seq "$ATTEMPTS"); do
    if body="$("${CURL[@]}" "$url")"; then echo "$body"; return 0; fi
    sleep "$DELAY_S"
  done
  echo "smoke: $url did not answer" >&2
  return 1
}

health="$(get_ok "$API/health")";   echo "health  $health"
get_ok "$API/ready" >/dev/null;     echo "ready   ok"
config="$(get_ok "$API/app/config")"
echo "config  $(echo "$config" | head -c 120)..."
get_ok "$ADMIN/login" >/dev/null;   echo "admin   ok"
if [ -n "$EXPECT_VERSION" ] && ! echo "$health" | grep -q "\"version\":\"$EXPECT_VERSION\""; then
  echo "smoke: expected version $EXPECT_VERSION, got $health" >&2
  exit 1
fi
echo "smoke: passed"

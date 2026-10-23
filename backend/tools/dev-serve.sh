#!/bin/zsh
# Development API: rebuild, migrate, seed and (re)start `nexa serve` on 0.0.0.0:8080
# against the local compose Postgres (:5433) and Redis (:6380). Logs: backend/tmp/nexa.log.
# Payments: the fake gateway settles after 2 s (NEXA_PAYMENT_GATEWAY=selcom needs credentials).
# OTP: code 0000 for the listed test numbers; other numbers get the code echoed in the response.
set -e
cd "${0:A:h}/.."
export NEXA_BRAND=${NEXA_BRAND:-nexa}
export NEXA_DATABASE_URL=${NEXA_DATABASE_URL:-'postgres://nexa:nexa@localhost:5433/nexa?sslmode=disable'}
export NEXA_REDIS_URL=${NEXA_REDIS_URL:-redis://localhost:6380/0}
go build -o bin/nexa ./cmd/nexa
./bin/nexa migrate up 2>&1 | tail -n 1
./bin/nexa seed all 2>&1 | tail -n 1
pkill -f 'bin/nexa serve' || true
sleep 1
mkdir -p tmp
NEXA_PAYMENT_GATEWAY=${NEXA_PAYMENT_GATEWAY:-fake} NEXA_FAKE_GATEWAY_DELAY=${NEXA_FAKE_GATEWAY_DELAY:-2s} \
NEXA_OTP_DEV_ECHO=true NEXA_OTP_TEST_CODE=0000 NEXA_OTP_TEST_NUMBERS=${NEXA_OTP_TEST_NUMBERS:-255626512332,255715123702} \
NEXA_CORS_ORIGINS='*' NEXA_HTTP_ADDR=0.0.0.0:8080 NEXA_JWT_KEYS_DIR=./.keys NEXA_LOG_LEVEL=warn \
  nohup ./bin/nexa serve > tmp/nexa.log 2>&1 &
for i in {1..30}; do sleep 1; curl -s -m 1 -o /dev/null localhost:8080/v1/health && { echo "api up on :8080"; exit 0; }; done
echo "api did not start; see backend/tmp/nexa.log" >&2; exit 1

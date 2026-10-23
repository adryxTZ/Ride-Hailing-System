BIN        := bin/nexa
VERSION    ?= $(shell git -C .. describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X main.Version=$(VERSION)
export NEXA_BRAND      ?= nexa
export NEXA_BRANDS_DIR ?= ./brands
export NEXA_SEED_DIR   ?= ./seed
export NEXA_DATABASE_URL ?= postgres://nexa:nexa@localhost:5433/nexa?sslmode=disable
export NEXA_REDIS_URL    ?= redis://localhost:6380/0

.PHONY: build run test lint tidy dev-up dev-down migrate brand-check

build:
	go build -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/nexa

run: build
	./$(BIN) serve --roles=api,realtime,worker,scheduler

test:
	go test ./... -count=1

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

dev-up:
	docker compose -f ../infra/compose.local.yml up -d --wait

dev-down:
	docker compose -f ../infra/compose.local.yml down

migrate:
	go tool goose -dir db/migrations postgres "$(NEXA_DATABASE_URL)" up

# Nothing under internal/ may contain a brand name (ADR 0005).
brand-check:
	@! grep -rniE 'songa|rider ?& ?hauler' internal/ cmd/ || (echo "brand name found under internal/ or cmd/" && exit 1)
export NEXA_OTP_TEST_CODE    ?= 0000
export NEXA_OTP_TEST_NUMBERS ?= 255626512332,255715123702
export NEXA_OTP_DEV_ECHO     ?= true
export NEXA_CORS_ORIGINS     ?= *

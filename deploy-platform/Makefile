GO ?= go

ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: build test lint vet infra api worker web-dev web-build demo-e2e fault-e2e

build:
	$(GO) build ./...

test:
	$(GO) test -mod=readonly ./...

lint: vet
	test -z "$$(gofmt -l .)"

vet:
	$(GO) vet ./...

infra:
	docker compose up -d --wait

api:
	$(GO) run ./cmd/api

worker:
	$(GO) run ./cmd/worker

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

demo-e2e:
	bash scripts/demo-e2e.sh

fault-e2e:
	GANTRY_FAULT_E2E=1 bash scripts/demo-e2e.sh

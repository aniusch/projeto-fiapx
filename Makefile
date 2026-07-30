# FIAP X — developer tasks. Run `make` or `make help` to list targets.

SHELL := /bin/bash
COMPOSE := docker compose

.DEFAULT_GOAL := help

## help: show this help
help:
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -E 's/^## /  /'

# --- Build & test ---------------------------------------------------------

## build: compile all packages
build:
	go build ./...

## test: run unit tests (no external services)
test:
	go test ./...

## test-race: unit tests with the race detector
test-race:
	go test -race ./...

## cover: unit tests with a total coverage summary (writes coverage.out)
cover:
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

## test-integration: integration tests — needs `make up` first
test-integration:
	go test -tags=integration ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: gofmt the code in place
fmt:
	gofmt -w cmd internal

## fmt-check: fail if any file isn't gofmt-ed
fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || { echo "not gofmt-ed:"; gofmt -l cmd internal; exit 1; }

## tidy: go mod tidy
tidy:
	go mod tidy

## check: what CI runs — fmt-check, vet, build, unit tests
check: fmt-check vet build test

# --- Local stack (docker compose) -----------------------------------------

## up: start the full local stack (infra + services + monitoring)
up:
	$(COMPOSE) up -d

## down: stop the stack (use `make down ARGS=-v` to also wipe data volumes)
down:
	$(COMPOSE) down $(ARGS)

## logs: follow logs from all services
logs:
	$(COMPOSE) logs -f

## images: build the service container images
images:
	$(COMPOSE) build

# --- Run a single service on the host -------------------------------------

## run-gateway: run the gateway on the host (:8080)
run-gateway:
	go run ./cmd/gateway

## run-worker: run the worker on the host (requires ffmpeg)
run-worker:
	go run ./cmd/worker

## run-notifier: run the notifier on the host
run-notifier:
	go run ./cmd/notifier

# --- Housekeeping ---------------------------------------------------------

## clean: remove build/coverage artifacts
clean:
	rm -f coverage.out
	go clean

.PHONY: help build test test-race cover test-integration vet fmt fmt-check tidy check \
	up down logs images run-gateway run-worker run-notifier clean

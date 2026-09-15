SHELL := /bin/bash

BIN ?= bin/dashboard
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
EMBED_DIST := internal/web/dist
MASTER_PLAN := master-plan-v5.md
SEED_JSON := internal/seed/seed.json
SEED_YEAR := 2026

.DEFAULT_GOAL := help

.PHONY: help setup doctor dev build clean-build seed-gen seed-check plan-check fmt fmt-check \
	tidy-check vet lint lint-go lint-web typecheck test test-go test-race test-web test-embed \
	smoke check check-ci verify

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: ## Install frontend dependencies from the lockfile
	cd web && pnpm install --frozen-lockfile

doctor: ## Check tool versions and installed dependencies
	@scripts/doctor.sh

dev: ## Run the Go API and Vite dev server together
	@set -euo pipefail; \
	trap 'trap - INT TERM EXIT; kill 0 2>/dev/null || true' INT TERM EXIT; \
	prefix() { while IFS= read -r line; do printf '[%s] %s\n' "$$1" "$$line"; done; }; \
	(go run ./cmd/server 2>&1 | prefix go) & \
	(cd web && pnpm dev 2>&1 | prefix vite) & \
	wait

build: ## Build bin/dashboard with the frontend embedded
	cd web && pnpm build
	rm -rf $(EMBED_DIST)
	mkdir -p $(EMBED_DIST)
	cp -R web/dist/. $(EMBED_DIST)/
	test -f $(EMBED_DIST)/index.html
	test -n "$$(find $(EMBED_DIST) -type f -print -quit)"
	CGO_ENABLED=0 go build -tags embed_frontend -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/server

clean-build: ## Remove build outputs
	rm -rf $(BIN) $(EMBED_DIST) web/dist

# Regenerate the committed default seed from the master plan. Runtime custom plans are loaded
# with cmd/server -plan.
seed-gen: ## Regenerate the committed seed from the master plan
	go run ./cmd/seedgen -year $(SEED_YEAR) -out $(SEED_JSON) $(MASTER_PLAN)

# Fails when the committed seed is not what seed-gen would write now: the master plan or the
# parser changed and nobody regenerated.
seed-check: ## Check the committed seed matches the master plan
	@set -euo pipefail; \
	generated="$$(mktemp)"; trap 'rm -f "$$generated"' EXIT; \
	go run ./cmd/seedgen -year $(SEED_YEAR) -out - $(MASTER_PLAN) >"$$generated"; \
	if ! cmp -s "$$generated" $(SEED_JSON); then \
		echo "$(SEED_JSON) is stale: run make seed-gen and commit the result"; \
		diff -u $(SEED_JSON) "$$generated" | head -40 || true; \
		exit 1; \
	fi

plan-check: ## Check the master plan parses
	go run ./cmd/server plan validate $(MASTER_PLAN)

fmt: ## Format Go and frontend code in place
	gofmt -w cmd internal
	cd web && pnpm format

# Verify formatting instead of applying it. CI must not rewrite files: `fmt` would silently
# reformat and then pass, so unformatted code would never fail a build.
fmt-check: ## Check formatting without changing files
	@unformatted="$$(gofmt -l cmd internal)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi
	cd web && pnpm format:check

tidy-check: ## Check go.mod and go.sum are tidy
	go mod tidy -diff

vet: ## Run go vet
	go vet ./...

lint: lint-go lint-web ## Run golangci-lint and eslint

lint-go: ## Run golangci-lint
	golangci-lint run ./...

lint-web: ## Run eslint
	cd web && pnpm lint

typecheck: ## Typecheck the frontend
	cd web && pnpm typecheck

test: test-go test-web ## Run Go and frontend tests

test-go: ## Run Go tests
	go test ./...

test-race: ## Run Go tests with the race detector
	go test -race ./...

test-web: ## Run frontend tests
	cd web && pnpm test

# Tests behind the embed_frontend build tag never run under plain `go test`. They need the
# frontend that `make build` copies into $(EMBED_DIST).
test-embed: ## Run Go tests against the embedded frontend (needs make build)
	@test -f $(EMBED_DIST)/index.html || { echo "$(EMBED_DIST) is empty: run make build first"; exit 1; }
	go test -tags embed_frontend ./internal/web/... ./cmd/server/...

smoke: ## Boot the built binary and check its responses (needs make build)
	@test -x $(BIN) || { echo "$(BIN) not found: run make build first"; exit 1; }
	scripts/smoke.sh $(BIN) $(VERSION)

check: fmt vet lint typecheck test ## Format, then vet, lint, typecheck and test

# What CI runs. Same checks, but it verifies formatting rather than fixing it.
check-ci: fmt-check vet lint typecheck test ## What CI runs: check without rewriting files

# The one command for "is everything working?": source checks, generated files, the production
# build, and the built binary serving requests. Keeps going past failures and ends with a
# summary; see scripts/verify.sh.
verify: ## Run every check, build, and smoke test, with a summary
	@MAKE="$(MAKE)" scripts/verify.sh

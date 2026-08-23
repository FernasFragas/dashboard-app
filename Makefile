SHELL := /bin/bash

BIN ?= bin/dashboard
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
EMBED_DIST := internal/web/dist

.PHONY: dev build clean-build seed-gen fmt fmt-check vet lint typecheck test check check-ci

dev:
	@set -euo pipefail; \
	trap 'trap - INT TERM EXIT; kill 0 2>/dev/null || true' INT TERM EXIT; \
	prefix() { while IFS= read -r line; do printf '[%s] %s\n' "$$1" "$$line"; done; }; \
	(go run ./cmd/server 2>&1 | prefix go) & \
	(cd web && pnpm dev 2>&1 | prefix vite) & \
	wait

build:
	cd web && pnpm build
	rm -rf $(EMBED_DIST)
	mkdir -p $(EMBED_DIST)
	cp -R web/dist/. $(EMBED_DIST)/
	test -f $(EMBED_DIST)/index.html
	test -n "$$(find $(EMBED_DIST) -type f -print -quit)"
	CGO_ENABLED=0 go build -tags embed_frontend -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/server

clean-build:
	rm -rf $(BIN) $(EMBED_DIST) web/dist

# Regenerate the committed default seed from the master plan. Runtime custom plans are loaded
# with cmd/server -plan.
seed-gen:
	go run ./cmd/seedgen -year 2026 -out internal/seed/seed.json master-plan-v5.md

fmt:
	gofmt -w cmd internal
	cd web && pnpm format

# Verify formatting instead of applying it. CI must not rewrite files: `fmt` would silently
# reformat and then pass, so unformatted code would never fail a build.
fmt-check:
	@unformatted="$$(gofmt -l cmd internal)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi
	cd web && pnpm format:check

vet:
	go vet ./...

lint:
	golangci-lint run ./...
	cd web && pnpm lint

typecheck:
	cd web && pnpm typecheck

test:
	go test ./...
	cd web && pnpm test

check: fmt vet lint typecheck test

# What CI runs. Same checks, but it verifies formatting rather than fixing it.
check-ci: fmt-check vet lint typecheck test

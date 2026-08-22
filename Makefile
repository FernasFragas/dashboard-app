SHELL := /bin/bash

.PHONY: dev seed-gen fmt vet lint typecheck test check

dev:
	@set -euo pipefail; \
	trap 'trap - INT TERM EXIT; kill 0 2>/dev/null || true' INT TERM EXIT; \
	prefix() { while IFS= read -r line; do printf '[%s] %s\n' "$$1" "$$line"; done; }; \
	(go run ./cmd/server 2>&1 | prefix go) & \
	(cd web && pnpm dev 2>&1 | prefix vite) & \
	wait

# Regenerate the committed seed from the master plan. The server never reads the markdown, so
# a parser bug can never block a boot - this is the only place the two meet.
seed-gen:
	go run ./cmd/seedgen -year 2026 -out internal/seed/seed.json master-plan-v5.md

fmt:
	gofmt -w cmd internal
	cd web && pnpm format

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

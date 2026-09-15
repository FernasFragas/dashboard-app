#!/usr/bin/env bash
# Checks the local toolchain against what CI uses and prints one actionable line per problem.
# Usage: scripts/doctor.sh   (or: make doctor)
set -uo pipefail

cd "$(dirname "$0")/.." || exit 2

failures=0
warnings=0

ok() { printf '  ok    %s\n' "$1"; }
fail() {
  printf '  FAIL  %s\n        fix: %s\n' "$1" "$2"
  failures=$((failures + 1))
}
warn() {
  printf '  warn  %s\n        fix: %s\n' "$1" "$2"
  warnings=$((warnings + 1))
}

# First x.y[.z] in a string.
version_of() { grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' <<<"$1" | head -n1; }

# at_least HAVE WANT, both x.y[.z]
at_least() {
  local have_major have_minor want_major want_minor
  IFS=. read -r have_major have_minor _ <<<"$1"
  IFS=. read -r want_major want_minor _ <<<"$2"
  ((have_major > want_major || (have_major == want_major && have_minor >= want_minor)))
}

port_in_use() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

echo "Toolchain"

if command -v go >/dev/null; then
  v="$(version_of "$(go version)")"
  if at_least "$v" 1.26; then ok "go $v"; else fail "go $v is older than go.mod's 1.26" "install Go 1.26.x from https://go.dev/dl/"; fi
else
  fail "go not found" "install Go 1.26.x from https://go.dev/dl/"
fi

want_node="$(tr -d '[:space:]v' <.nvmrc)"
if command -v node >/dev/null; then
  v="$(version_of "$(node --version)")"
  if [[ "${v%%.*}" == "$want_node" ]]; then ok "node $v"; else fail "node $v, but .nvmrc pins $want_node" "nvm install && nvm use"; fi
else
  fail "node not found" "install Node $want_node (nvm install)"
fi

want_pnpm="$(grep -oE '"packageManager": *"pnpm@[0-9]+' web/package.json | grep -oE '[0-9]+$')"
if command -v pnpm >/dev/null; then
  v="$(version_of "$(pnpm --version)")"
  if [[ "${v%%.*}" == "$want_pnpm" ]]; then ok "pnpm $v"; else fail "pnpm $v, but web/package.json pins pnpm $want_pnpm" "npm install -g pnpm@$want_pnpm"; fi
else
  fail "pnpm not found" "npm install -g pnpm@$want_pnpm"
fi

lint_fix="go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0 (builds it with your Go 1.26, as CI does)"
if command -v golangci-lint >/dev/null; then
  out="$(golangci-lint version 2>&1)"
  v="$(version_of "$out")"
  built="$(grep -oE 'go[0-9]+\.[0-9]+' <<<"$out" | head -n1)"
  built="${built#go}"
  if [[ "${v%%.*}" != "2" ]]; then
    warn "golangci-lint $v is not 2.x; make check will fail" "$lint_fix"
  elif [[ -n "$built" ]] && ! at_least "$built" 1.26; then
    warn "golangci-lint was built with go$built and refuses a go 1.26 module" "$lint_fix"
  else
    ok "golangci-lint $v (go$built)"
  fi
else
  warn "golangci-lint not found; needed only for make check / make check-ci" "$lint_fix"
fi

command -v jq >/dev/null && ok "jq (used by .claude/hooks)" || warn "jq not found; Claude Code hooks in .claude/hooks need it" "brew install jq"

echo "Project"

if [[ -d web/node_modules ]]; then ok "web/node_modules installed"; else warn "web/node_modules missing" "make setup"; fi

for port in 8484 5173; do
  if port_in_use "$port"; then
    warn "port $port is in use, so make dev cannot start" "lsof -ti:$port | xargs kill — or run scripts/dev-instance.sh, which picks free ports"
  else
    ok "port $port free"
  fi
done

echo
if ((failures > 0)); then
  echo "doctor: $failures problem(s), $warnings warning(s)"
  exit 1
fi
echo "doctor: ready ($warnings warning(s))"

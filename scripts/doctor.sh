#!/usr/bin/env bash
# Checks that the tools the Makefile calls are installed at the versions this repo pins (go.mod,
# .nvmrc, web/package.json) and that frontend dependencies are installed. It reports every
# problem rather than stopping at the first, so one run shows the whole setup gap.
set -uo pipefail
cd "$(dirname "$0")/.."

problems=0
ok() { printf '  ok    %s\n' "$*"; }
bad() {
  printf '  FAIL  %s\n' "$*"
  problems=$((problems + 1))
}
have() { command -v "$1" >/dev/null 2>&1 || { bad "$1: not found on PATH"; return 1; }; }

# version_at_least HAVE WANT, both "major.minor".
version_at_least() {
  local have_major="${1%%.*}" have_minor="${1#*.}" want_major="${2%%.*}" want_minor="${2#*.}"
  ((have_major > want_major || (have_major == want_major && have_minor >= want_minor)))
}

go_want="$(sed -n 's/^go \([0-9]*\.[0-9]*\).*/\1/p' go.mod)"
if have go; then
  go_have="$(go env GOVERSION | sed 's/^go\([0-9]*\.[0-9]*\).*/\1/')"
  if version_at_least "$go_have" "$go_want"; then
    ok "go $go_have"
  else
    bad "go $go_have: go.mod requires $go_want"
  fi
fi

if have golangci-lint; then
  lint_have="$(golangci-lint version --short 2>/dev/null)"
  # golangci-lint refuses to lint a module whose go directive is newer than the Go it was built
  # with, which fails with a confusing message. See the note in .github/workflows/ci.yml.
  lint_go="$(golangci-lint version 2>/dev/null | sed -n 's/.*built with go\([0-9]*\.[0-9]*\).*/\1/p')"
  if [[ "${lint_have%%.*}" != 2 ]]; then
    bad "golangci-lint $lint_have: .golangci.yml needs 2.x"
  elif [[ -n "$lint_go" ]] && ! version_at_least "$lint_go" "$go_want"; then
    bad "golangci-lint $lint_have is built with go$lint_go, older than go.mod's $go_want;" \
      "reinstall with: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$lint_have"
  else
    ok "golangci-lint $lint_have"
  fi
fi

node_want="$(tr -d '[:space:]' <.nvmrc)"
if have node; then
  node_have="$(node --version | sed 's/^v//')"
  if [[ "${node_have%%.*}" == "$node_want" ]]; then
    ok "node $node_have"
  else
    bad "node $node_have: .nvmrc pins $node_want"
  fi
fi

pnpm_want="$(sed -n 's/.*"packageManager": *"pnpm@\([0-9.]*\)".*/\1/p' web/package.json)"
if have pnpm; then
  # Run inside web/ so pnpm switches to the packageManager version, as every make target does.
  pnpm_have="$(cd web && pnpm --version 2>/dev/null)"
  if [[ "${pnpm_have%%.*}" == "${pnpm_want%%.*}" ]]; then
    ok "pnpm $pnpm_have"
  else
    bad "pnpm $pnpm_have: web/package.json wants $pnpm_want"
  fi
fi

have curl && ok "curl"

if [[ -f web/node_modules/.modules.yaml ]]; then
  ok "web/node_modules installed"
else
  bad "web/node_modules missing: run make setup"
fi

if ((problems > 0)); then
  echo "doctor: $problems problem(s)"
  exit 1
fi

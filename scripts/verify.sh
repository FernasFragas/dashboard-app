#!/usr/bin/env bash
# Runs every project check and prints a pass/fail summary: the source checks from `make check`,
# plus checks on what the project produces - the committed seed, the production build, and the
# built binary actually serving requests.
#
# Unlike `make check` it keeps going after a failure, so one run lists everything that is broken.
# Steps that need the build are skipped when the build fails. FAIL_FAST=1 stops at the first
# failure instead.
#
# Usage: make verify
set -uo pipefail
cd "$(dirname "$0")/.."

make_cmd="${MAKE:-make}"

# target|description, in run order. Cheap checks first so the common failures show up fast.
steps=(
  "fmt-check|Formatting (gofmt, prettier)"
  "tidy-check|go.mod and go.sum are tidy"
  "vet|go vet"
  "lint|Lint (golangci-lint, eslint)"
  "typecheck|TypeScript"
  "plan-check|Master plan parses"
  "seed-check|Committed seed matches master plan"
  "test-race|Go tests with race detector"
  "test-web|Frontend tests (vitest)"
  "build|Production build"
  "test-embed|Go tests against embedded frontend"
  "smoke|Built binary serves correct responses"
)
needs_build=" test-embed smoke "

if [[ -t 1 ]]; then
  green=$'\033[32m' red=$'\033[31m' yellow=$'\033[33m' bold=$'\033[1m' reset=$'\033[0m'
else
  green="" red="" yellow="" bold="" reset=""
fi

log_dir="$(mktemp -d)"
results=()
failed=0
build_failed=0

run_step() {
  local target="$1" description="$2" log="$log_dir/$1.log" start elapsed
  printf '  %-45s ' "$description"
  start="$(date +%s)"
  if "$make_cmd" --no-print-directory "$target" >"$log" 2>&1; then
    elapsed=$(($(date +%s) - start))
    printf '%sPASS%s %3ss\n' "$green" "$reset" "$elapsed"
    results+=("PASS|$target|$description")
    return 0
  fi
  elapsed=$(($(date +%s) - start))
  printf '%sFAIL%s %3ss\n' "$red" "$reset" "$elapsed"
  results+=("FAIL|$target|$description")
  echo "    --- make $target (last 30 lines) ---"
  tail -30 "$log" | sed 's/^/    /'
  echo
  return 1
}

echo "${bold}Toolchain${reset}"
if ! "$make_cmd" --no-print-directory doctor; then
  echo
  echo "${red}${bold}verify: fix the toolchain problems above first${reset}"
  rm -rf "$log_dir"
  exit 1
fi

echo
echo "${bold}Checks${reset}"
for step in "${steps[@]}"; do
  target="${step%%|*}"
  description="${step#*|}"

  if ((build_failed)) && [[ "$needs_build" == *" $target "* ]]; then
    printf '  %-45s %sSKIP%s (build failed)\n' "$description" "$yellow" "$reset"
    results+=("SKIP|$target|$description")
    continue
  fi

  if ! run_step "$target" "$description"; then
    failed=$((failed + 1))
    [[ "$target" == build ]] && build_failed=1
    if [[ "${FAIL_FAST:-0}" == 1 ]]; then
      break
    fi
  fi
done

echo
if ((failed == 0)); then
  echo "${green}${bold}verify: all ${#results[@]} checks passed${reset}"
  rm -rf "$log_dir"
  exit 0
fi

echo "${red}${bold}verify: $failed failed${reset}"
for result in "${results[@]}"; do
  IFS='|' read -r status target _ <<<"$result"
  [[ "$status" == PASS ]] && continue
  echo "  $status  make $target"
done
echo "Full logs: $log_dir"
exit 1

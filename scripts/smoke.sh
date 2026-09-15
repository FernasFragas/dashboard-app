#!/usr/bin/env bash
# Boots a built binary against a throwaway data directory and checks what it actually serves:
# health and the stamped version, the embedded plan seeded with the counts the plan parser
# reports, a write that reads back, the embedded frontend (index, its assets, SPA deep links),
# API 404s not swallowed by the SPA, and a clean exit on SIGTERM with no error logs.
#
# Usage: scripts/smoke.sh BIN [EXPECTED_VERSION]
# SMOKE_PORT sets the port (default 18484). SMOKE_PLAN is the plan the binary embeds.
set -euo pipefail
cd "$(dirname "$0")/.."

bin="${1:?usage: scripts/smoke.sh BIN [EXPECTED_VERSION]}"
expected_version="${2:-}"
plan_file="${SMOKE_PLAN:-master-plan-v5.md}"
port="${SMOKE_PORT:-18484}"
base="http://127.0.0.1:$port"

work="$(mktemp -d)"
server_pid=""

cleanup() {
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

fail() {
  echo "smoke: FAIL $*" >&2
  if [[ -s "$work/server.log" ]]; then
    echo "--- server log (last 20 lines) ---" >&2
    tail -20 "$work/server.log" >&2
  fi
  exit 1
}

pass() { echo "smoke: ok   $*"; }

# request METHOD PATH [JSON_BODY] sets $status, $body and $headers.
request() {
  local method="$1" path="$2" data="${3:-}"
  local args=(-sS -X "$method" -o "$work/body" -D "$work/headers" -w '%{http_code}')
  if [[ -n "$data" ]]; then
    args+=(-H 'Content-Type: application/json' --data "$data")
  fi
  status="$(curl "${args[@]}" "$base$path")" || fail "$method $path: no response"
  body="$(cat "$work/body")"
  headers="$(tr -d '\r' <"$work/headers")"
}

expect_status() {
  [[ "$status" == "$1" ]] || fail "$2: status $status, want $1. Body: ${body:0:300}"
}

expect_body() {
  [[ "$body" == *"$1"* ]] || fail "$2: body lacks $1. Body: ${body:0:300}"
}

[[ -x "$bin" ]] || fail "$bin is not an executable"
if curl -s -o /dev/null "$base/"; then
  fail "port $port is already in use; set SMOKE_PORT to a free port"
fi

# The binary's own parser says what the embedded plan should seed. seed-check separately
# guarantees the embedded seed matches this plan file.
validation="$("$bin" plan validate "$plan_file")" || fail "$bin plan validate $plan_file"
plan_count() { sed -n "s/.* \([0-9][0-9]*\) $1.*/\1/p" <<<"$validation"; }

"$bin" -addr "127.0.0.1:$port" -data "$work/data" -log-format json >"$work/server.log" 2>&1 &
server_pid=$!

for _ in {1..50}; do
  curl -fs -o /dev/null "$base/api/health" && break
  kill -0 "$server_pid" 2>/dev/null || fail "server exited during startup"
  sleep 0.2
done

request GET /api/health
expect_status 200 "GET /api/health"
expect_body '"status":"ok"' "GET /api/health"
if [[ -n "$expected_version" ]]; then
  expect_body "\"version\":\"$expected_version\"" "GET /api/health (stale binary? run make build)"
elif [[ "$body" == *'"version":"dev"'* ]]; then
  fail "GET /api/health: version is dev, so the build did not stamp it"
fi
pass "health ok, version stamped"

request GET /api/plan
expect_status 200 "GET /api/plan"
counts="$(sed -n 's/.*"counts":{\([^}]*\)}.*/\1/p' <<<"$body")"
[[ -n "$counts" ]] || fail "GET /api/plan: no counts object. Body: ${body:0:300}"
for kind in projects weeks tasks goals skills; do
  want="$(plan_count "$kind")"
  [[ -n "$want" ]] || fail "could not read a $kind count from: $validation"
  [[ ",$counts," == *",\"$kind\":$want,"* ]] || fail "GET /api/plan: want $kind=$want, got {$counts}"
done
pass "seeded plan matches: $validation"

request GET /api/dashboard
expect_status 200 "GET /api/dashboard"
[[ "$headers" == *"Content-Type: application/json"* ]] || fail "GET /api/dashboard: not JSON"
pass "dashboard responds with JSON"

request GET /api/projects
expect_status 200 "GET /api/projects"
project="$(sed -n 's/.*"projects":\[{"id":"\([^"]*\)".*/\1/p' <<<"$body")"
[[ -n "$project" ]] || fail "GET /api/projects: no project id. Body: ${body:0:300}"

title="smoke test goal $$"
request POST /api/goals "{\"title\":\"$title\",\"project\":\"$project\"}"
expect_status 201 "POST /api/goals"
location="$(sed -n 's/^Location: //p' <<<"$headers")"
[[ -n "$location" ]] || fail "POST /api/goals: no Location header"
request GET "$location"
expect_status 200 "GET $location"
expect_body "\"title\":\"$title\"" "GET $location"
pass "goal written and read back from $location"

request GET /
expect_status 200 "GET /"
expect_body '<div id="root"></div>' "GET / (frontend not embedded?)"
asset="$(grep -o 'src="/assets/[^"]*\.js"' <<<"$body" | head -1 | sed 's/^src="//; s/"$//' || true)"
[[ -n "$asset" ]] || fail "GET /: index.html references no script under /assets/"
request GET "$asset"
expect_status 200 "GET $asset"
request GET /goals
expect_status 200 "GET /goals"
expect_body '<div id="root"></div>' "GET /goals (SPA deep link)"
pass "frontend embedded: index, $asset, deep link"

request GET /api/does-not-exist
expect_status 404 "GET /api/does-not-exist"
[[ "$body" != *'<div id="root"></div>'* ]] || fail "GET /api/does-not-exist: served the SPA, not a 404"
pass "unknown API routes 404 instead of falling through to the SPA"

kill -TERM "$server_pid"
for _ in {1..50}; do
  kill -0 "$server_pid" 2>/dev/null || break
  sleep 0.2
done
if kill -0 "$server_pid" 2>/dev/null; then
  fail "server still running 10s after SIGTERM"
fi
exit_code=0
wait "$server_pid" || exit_code=$?
server_pid=""
[[ "$exit_code" == 0 ]] || fail "server exited with status $exit_code after SIGTERM, want 0"
if grep -q '"level":"ERROR"' "$work/server.log"; then
  fail "server logged errors"
fi
pass "SIGTERM shuts down cleanly with no error logs"

echo "smoke: all checks passed"

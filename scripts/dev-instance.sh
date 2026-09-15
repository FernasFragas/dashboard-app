#!/usr/bin/env bash
# Runs an isolated dashboard: a throwaway data directory, free ports, JSON logs on disk. Several
# instances can run side by side (one per worktree or agent) without touching ~/dashboard-data or
# the ports `make dev` uses.
#
# Usage: scripts/dev-instance.sh [--api-only] [--plan PATH] [--data DIR]
#
# Prints the URLs and runs until Ctrl-C or SIGTERM, which stops every process it started. The data
# directory is kept afterwards so its database and logs can be inspected. The same values are
# written to <data>/instance.env for tools that start this script in the background.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
api_only=false
plan=""
data=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --api-only) api_only=true; shift ;;
    --plan) plan="$2"; shift 2 ;;
    --data) data="$2"; shift 2 ;;
    -h | --help) sed -n '2,10p' "$0"; exit 0 ;;
    *) echo "dev-instance: unknown argument '$1' (see --help)" >&2; exit 2 ;;
  esac
done

port_in_use() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

free_port() {
  local port=$1
  while port_in_use "$port"; do port=$((port + 1)); done
  echo "$port"
}

wait_for() {
  local url=$1 pid=$2 log=$3 name=$4
  for _ in $(seq 1 150); do
    if curl -fsS -o /dev/null "$url" 2>/dev/null; then return 0; fi
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "dev-instance: $name exited during startup; last log lines from $log:" >&2
      tail -n 20 "$log" >&2
      return 1
    fi
    sleep 0.1
  done
  echo "dev-instance: $name did not answer $url within 15 s; see $log" >&2
  return 1
}

if [[ -z "$data" ]]; then
  data="$(mktemp -d "${TMPDIR:-/tmp}/dashboard-instance.XXXXXX")"
fi
case "$data" in
  "$HOME/dashboard-data" | "$HOME/dashboard-data/"*)
    echo "dev-instance: refusing to use the live data directory $data" >&2
    exit 2
    ;;
esac
mkdir -p "$data"

pids=()
cleanup() {
  trap - INT TERM EXIT
  for pid in ${pids[@]+"${pids[@]}"}; do kill "$pid" 2>/dev/null || true; done
  wait 2>/dev/null || true
  echo "dev-instance: stopped; data kept in $data"
}
trap cleanup INT TERM EXIT

echo "dev-instance: building server into $data/dashboard"
(cd "$root" && go build -o "$data/dashboard" ./cmd/server)

api_port="$(free_port $((18484 + RANDOM % 500)))"
api_url="http://127.0.0.1:$api_port"
server_args=(-addr "127.0.0.1:$api_port" -data "$data" -log-format json -public-url "$api_url")
if [[ -n "$plan" ]]; then server_args+=(-plan "$plan"); fi

"$data/dashboard" "${server_args[@]}" >"$data/server.log" 2>&1 &
pids+=($!)
wait_for "$api_url/api/health" "${pids[0]}" "$data/server.log" "server"

web_url=""
if [[ "$api_only" == false ]]; then
  web_port="$(free_port $((15173 + RANDOM % 500)))"
  web_url="http://127.0.0.1:$web_port"
  # node runs vite.js directly (not the .bin shim) so the recorded PID is the server itself.
  (cd "$root/web" && DASHBOARD_API_URL="$api_url" exec node ./node_modules/vite/bin/vite.js --port "$web_port" --strictPort --host 127.0.0.1) >"$data/web.log" 2>&1 &
  pids+=($!)
  wait_for "$web_url/api/health" "${pids[1]}" "$data/web.log" "vite"
fi

cat >"$data/instance.env" <<EOF
DASHBOARD_DATA=$data
DASHBOARD_API_URL=$api_url
DASHBOARD_WEB_URL=$web_url
DASHBOARD_SERVER_LOG=$data/server.log
DASHBOARD_WEB_LOG=$data/web.log
EOF

cat <<EOF
dev-instance: ready
  api     $api_url  (GET /api/health, /api/export)
  web     ${web_url:-not started (--api-only)}
  data    $data  (sqlite3 "$data/dashboard.db")
  logs    $data/server.log (JSON, one line per request)${web_url:+, $data/web.log}
  env     $data/instance.env
Ctrl-C or SIGTERM stops it.
EOF

wait

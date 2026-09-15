#!/usr/bin/env bash
# PostToolUse(Edit|Write): format the file that was just written the way CI checks it — gofmt for
# Go, Prettier for anything under web/. Never blocks; a formatter error only shows in the output.
set -uo pipefail

file="$(jq -r '.tool_input.file_path // empty')"
[[ -n "$file" && -f "$file" ]] || exit 0

root="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "$0")/../.." && pwd)}"

case "$file" in
  "$root"/web/node_modules/* | "$root"/web/dist/*) ;;
  *.go) gofmt -w "$file" ;;
  "$root"/web/*) (cd "$root/web" && ./node_modules/.bin/prettier --write --ignore-unknown --log-level warn "$file") ;;
esac

exit 0

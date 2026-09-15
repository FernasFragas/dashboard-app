#!/usr/bin/env bash
# PreToolUse(Edit|Write): refuse edits to migrations that are already committed. Applied
# migrations are immutable (docs/DATABASE.md §7). Exit 2 blocks the call and shows stderr to Claude.
set -uo pipefail

file="$(jq -r '.tool_input.file_path // empty')"
root="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "$0")/../.." && pwd)}"

case "$file" in
  "$root"/migrations/*.sql) ;;
  *) exit 0 ;;
esac

rel="${file#"$root"/}"
git -C "$root" cat-file -e "HEAD:$rel" 2>/dev/null || exit 0

last="$(ls "$root"/migrations/*.sql | sed -E 's#.*/([0-9]+)_[^/]*$#\1#' | sort -n | tail -n1)"
next="$(printf '%03d' $((10#$last + 1)))"

cat >&2 <<EOF
Blocked: $rel is committed, and applied migrations are immutable (docs/DATABASE.md §7).
A database that already ran it would never see this edit.
Put the change in a new file instead: migrations/${next}_<description>.sql
(migrations/008_v6_achievement_text.sql is the model). If the user explicitly wants this file
changed, stop and tell them why it is blocked.
EOF
exit 2

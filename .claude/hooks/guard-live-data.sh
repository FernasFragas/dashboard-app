#!/usr/bin/env bash
# PreToolUse(Bash): ask the user before commands that can change the live database or the running
# launchd service. Agents work against throwaway data (AGENTS.md, "Operating limits").
set -uo pipefail

cmd="$(jq -r '.tool_input.command // empty')"

reason=""
case "$cmd" in
  *-plan-reset*) reason="-plan-reset deletes dashboard.db" ;;
  *launchctl*) reason="launchctl changes the running dashboard service" ;;
  *"~/dashboard-data"* | *'$HOME/dashboard-data'* | *"$HOME/dashboard-data"*) reason="the command touches the live data directory ~/dashboard-data" ;;
esac

[[ -n "$reason" ]] || exit 0

jq -n --arg reason "$reason. Agents use scripts/dev-instance.sh or -data \"\$(mktemp -d)\" unless the user asked for the live instance." \
  '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "ask", permissionDecisionReason: $reason}}'

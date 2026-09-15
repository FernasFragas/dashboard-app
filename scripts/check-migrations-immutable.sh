#!/usr/bin/env bash
# Applied migrations are immutable (docs/DATABASE.md §7). Once a migration exists on the base ref,
# any later change must be a new NNN_description.sql, never an edit to or deletion of the old file:
# a database that already ran it will never see the edit.
#
# Usage: scripts/check-migrations-immutable.sh [BASE_REF]   (default: origin/main)
set -euo pipefail

cd "$(dirname "$0")/.."

base="${1:-origin/main}"

# A push that creates a branch, and scheduled runs, have no "before" commit to compare with.
if [[ -z "$base" || "$base" =~ ^0+$ ]]; then
  echo "check-migrations: no base commit to compare with; skipped"
  exit 0
fi

if ! git rev-parse --verify --quiet "${base}^{commit}" >/dev/null; then
  echo "check-migrations: base '$base' is not a known commit; run 'git fetch origin main' or pass a ref" >&2
  exit 2
fi

merge_base="$(git merge-base "$base" HEAD)"
changed="$(git diff --no-renames --name-only --diff-filter=MD "$merge_base" -- 'migrations/*.sql')"

if [[ -n "$changed" ]]; then
  {
    echo "check-migrations: applied migrations were modified or deleted (compared with $base):"
    sed 's/^/  /' <<<"$changed"
    echo
    echo "Migrations are forward-only and immutable once merged (docs/DATABASE.md §7)."
    echo "Restore each file with:  git checkout $merge_base -- <file>"
    echo "then put the change in a new migrations/NNN_description.sql with the next free number."
  } >&2
  exit 1
fi

echo "check-migrations: no applied migration changed since $base"

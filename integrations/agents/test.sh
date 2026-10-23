#!/usr/bin/env bash
set -euo pipefail

source_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_source="$source_dir/../../skills/open-review/SKILL.md"
target="$(mktemp -d)"
trap 'rm -rf "$target"' EXIT
git -C "$target" init -q
for host in codex cursor compatible claude kimi opencode; do
  installed="$(bash "$source_dir/install.sh" "$host" "$target")"
  cmp "$skill_source" "$installed"
done
if bash "$source_dir/install.sh" unknown "$target" >/dev/null 2>&1; then
  echo 'unknown host accepted' >&2
  exit 1
fi

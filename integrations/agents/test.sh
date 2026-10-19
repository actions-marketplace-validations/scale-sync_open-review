#!/usr/bin/env bash
set -euo pipefail

source_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
target="$(mktemp -d)"
trap 'rm -rf "$target"' EXIT
git -C "$target" init -q
for host in codex cursor compatible claude kimi opencode; do
  installed="$(bash "$source_dir/install.sh" "$host" "$target")"
  cmp "$source_dir/open-review/SKILL.md" "$installed"
done
if bash "$source_dir/install.sh" unknown "$target" >/dev/null 2>&1; then
  echo 'unknown host accepted' >&2
  exit 1
fi

#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo 'usage: install.sh HOST TARGET_REPOSITORY' >&2
  exit 2
fi

case "$1" in
  codex|cursor|compatible) folder='.agents/skills/open-review' ;;
  claude) folder='.claude/skills/open-review' ;;
  kimi) folder='.kimi-code/skills/open-review' ;;
  opencode) folder='.opencode/skills/open-review' ;;
  *) echo 'supported hosts: codex, cursor, claude, kimi, opencode, compatible' >&2; exit 2 ;;
esac

target="$2"
if [[ ! -d "$target/.git" ]]; then
  echo 'target must be a Git repository root' >&2
  exit 2
fi
source_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "$target/$folder"
cp "$source_dir/open-review/SKILL.md" "$target/$folder/SKILL.md"
echo "$target/$folder/SKILL.md"

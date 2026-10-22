#!/usr/bin/env bash
set -euo pipefail

release_version="${1:?usage: build.sh v1.12.0}"
if [[ ! "$release_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release version: $release_version" >&2
  exit 1
fi

release_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
release_out="$release_root/dist/release"
mkdir -p "$release_out"

for release_target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  release_os="${release_target%/*}"
  release_arch="${release_target#*/}"
  release_stage="$(mktemp -d)"
  release_executable="open-review"
  if [[ "$release_os" == windows ]]; then release_executable="open-review.exe"; fi
  GOOS="$release_os" GOARCH="$release_arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags='-s -w' -o "$release_stage/$release_executable" ./cmd/open-review
  release_asset="open-review_${release_version}_${release_os}_${release_arch}"
  if [[ "$release_os" == windows ]]; then
    (cd "$release_stage" && zip -q "$release_out/$release_asset.zip" "$release_executable")
  else
    tar -czf "$release_out/$release_asset.tar.gz" -C "$release_stage" "$release_executable"
  fi
  rm -r "$release_stage"
done

node "$release_root/integrations/release/manifest.mjs" "$release_out" "$release_version"

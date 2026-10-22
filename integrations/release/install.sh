#!/usr/bin/env bash
set -euo pipefail

release_version="${1:?usage: install.sh v1.12.0}"
if [[ ! "$release_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release version: $release_version" >&2
  exit 1
fi
case "$(uname -s)" in
  Darwin) release_os=darwin ;;
  Linux) release_os=linux ;;
  *) echo "unsupported operating system" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64) release_arch=amd64 ;;
  arm64|aarch64) release_arch=arm64 ;;
  *) echo "unsupported architecture" >&2; exit 1 ;;
esac

release_asset="open-review_${release_version}_${release_os}_${release_arch}.tar.gz"
release_base="https://github.com/scale-sync/open-review/releases/download/$release_version"
release_stage="$(mktemp -d)"
trap 'rm -r "$release_stage"' EXIT
curl --fail --location --silent --show-error "$release_base/$release_asset" -o "$release_stage/$release_asset"
curl --fail --location --silent --show-error "$release_base/SHA256SUMS-$release_version" -o "$release_stage/SHA256SUMS"
release_expected="$(awk -v file="$release_asset" '$2 == file { print $1 }' "$release_stage/SHA256SUMS")"
if [[ -z "$release_expected" ]]; then echo "missing checksum for $release_asset" >&2; exit 1; fi
release_actual="$(shasum -a 256 "$release_stage/$release_asset" | awk '{ print $1 }')"
if [[ "$release_actual" != "$release_expected" ]]; then echo "checksum mismatch" >&2; exit 1; fi
tar -xzf "$release_stage/$release_asset" -C "$release_stage" open-review
release_install_dir="${OPEN_REVIEW_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$release_install_dir"
install -m 755 "$release_stage/open-review" "$release_install_dir/open-review"
echo "Installed $release_install_dir/open-review"

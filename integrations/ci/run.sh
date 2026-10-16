#!/usr/bin/env bash
set -u
set -o pipefail
umask 077

: "${OPEN_REVIEW_BASE:?Set OPEN_REVIEW_BASE to the target commit or ref}"
: "${OPEN_REVIEW_HEAD:?Set OPEN_REVIEW_HEAD to the reviewed commit or ref}"

repo="${OPEN_REVIEW_REPO:-.}"
out="${OPEN_REVIEW_OUT_DIR:-open-review-artifacts}"
mkdir -p "$out" || exit 1

review_cli() {
  if [[ -n "${OPEN_REVIEW_BIN:-}" ]]; then
    "$OPEN_REVIEW_BIN" "$@"
  else
    go run ./cmd/open-review "$@"
  fi
}

review_cli review --repo "$repo" --base "$OPEN_REVIEW_BASE" --head "$OPEN_REVIEW_HEAD" --output json --output-file "$out/review.json" --quiet
status=$?
if [[ -s "$out/review.json" ]]; then
  review_cli sarif --input "$out/review.json" --output "$out/review.sarif" || exit 1
fi
exit "$status"

#!/usr/bin/env bash
set -euo pipefail

task_tmp="$(mktemp -d)"
trap 'rm -rf "$task_tmp"' EXIT
cat > "$task_tmp/reviewer" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
command="$1"
shift
output=""
while (($#)); do
  if [[ "$1" == "--output-file" || "$1" == "--output" ]]; then output="$2"; shift 2; else shift; fi
done
if [[ "$command" == "review" ]]; then
  printf '%s' '{"status":"partial","findings":[]}' > "$output"
  exit 2
fi
printf '%s' '{"version":"2.1.0"}' > "$output"
SCRIPT
chmod 700 "$task_tmp/reviewer"

set +e
OPEN_REVIEW_BIN="$task_tmp/reviewer" OPEN_REVIEW_BASE=base OPEN_REVIEW_HEAD=head OPEN_REVIEW_OUT_DIR="$task_tmp/out" bash integrations/ci/run.sh
status=$?
set -e
[[ "$status" == 2 ]]
[[ -s "$task_tmp/out/review.json" && -s "$task_tmp/out/review.sarif" ]]
if OPEN_REVIEW_BIN="$task_tmp/reviewer" OPEN_REVIEW_HEAD=head OPEN_REVIEW_OUT_DIR="$task_tmp/invalid" bash integrations/ci/run.sh >/dev/null 2>&1; then
  exit 1
fi

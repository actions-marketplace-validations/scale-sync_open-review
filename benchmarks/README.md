# Benchmark method

This benchmark uses the public code review corpus described in [the research paper](https://arxiv.org/abs/2601.19494). The dataset stays in ignored local research material; its contents are not copied into the product repository.

The validated dataset file has SHA-256 `0804505f0a474765ce2840c832cfeaa6c4f0250dd6ccb169fe73c6758b245a86`. It contains 200 pull requests from 50 projects in 10 languages, with 1,505 accepted and 640 rejected annotated comments. Every result includes the checksum so a later dataset revision cannot silently change a score.

## Inspect the corpus

Place the public `dataset.json` file at `_research_refs/aacr-bench/dataset.json`, then run:

```bash
go run ./cmd/benchmark inspect --dataset _research_refs/aacr-bench/dataset.json
go run ./cmd/benchmark run --dataset _research_refs/aacr-bench/dataset.json
```

The second command is a dry run. It lists the number of pull requests and makes no repository or model calls.

## Run a review

Build a pinned CLI binary and use a separate run file for each model, reviewer version, and effort level. The `--execute` flag downloads source repositories and calls the selected model, so use it only with a defined API budget.

```bash
go build -o /tmp/open-review-bench ./cmd/open-review
go run ./cmd/benchmark run \
  --dataset _research_refs/aacr-bench/dataset.json \
  --binary /tmp/open-review-bench \
  --cache /tmp/open-review-benchmark-repos \
  --out /tmp/open-review-benchmark-run.json \
  --provider openai --model MODEL_NAME --effort medium \
  --version GIT_SHA --execute
```

The runner uses the fixed base and head commits recorded for each pull request. It records status, findings, elapsed time, and token usage after each case. A checkpoint lets a stopped run resume; failed cases are retried. `--limit N` supports a small trial but cannot produce a publishable full-corpus score.

## Adjudicate findings

Every generated finding needs a human decision. A `match` points to the zero-based row index of one accepted dataset comment from the same pull request. A `false_positive` has no row. Two findings cannot claim the same accepted comment. Keep this judgment file with the run artifact so another reviewer can inspect disagreements.

```json
[
  {
    "prUrl": "https://github.com/OWNER/REPO/pull/1",
    "findingId": "FINDING_ID",
    "verdict": "match",
    "referenceRow": 0
  },
  {
    "prUrl": "https://github.com/OWNER/REPO/pull/1",
    "findingId": "ANOTHER_ID",
    "verdict": "false_positive"
  }
]
```

Score only after all 200 cases are complete and all findings are judged:

```bash
go run ./cmd/benchmark score \
  --dataset _research_refs/aacr-bench/dataset.json \
  --run /tmp/open-review-benchmark-run.json \
  --judgments /tmp/open-review-benchmark-judgments.json
```

## Score definitions

- **Precision** = matched findings / all generated findings.
- **Recall** = matched accepted comments / 1,505 accepted comments.
- **F1** = harmonic mean of precision and recall.
- **Line precision** = matched findings with the same file and an overlapping or adjacent right-side line range / all generated findings.
- **Noise** = unmatched findings / all generated findings.
- **Mean time and tokens** = arithmetic mean across pull requests, including zero-finding reviews. Estimated token usage is counted separately.

The scorer also reports each language separately. Rejected annotated comments are never counted as ground truth. A model finding that repeats one is a false positive unless a human reviewer identifies a different valid issue. Findings on left-side or unchanged lines cannot earn a location hit under the current Go review contract.

## Publication gate

Do not publish a leaderboard number from a partial run, a changed dataset, or unjudged findings. Keep the raw run, judgments, report, model identity, effort, reviewer revision, and dataset checksum together. The website reads reviewed reports from `web/src/content/benchmark-results.json`; it starts empty.

# GitHub Action

The bundled Action reviews pull requests from a Linux x64 runner. The TypeScript adapter handles GitHub events, and the Go engine produces findings. The Action does not install dependencies during an event.

## Add a workflow

Create `.github/workflows/open-review.yml` in the repository you want to review:

```yaml
name: Code review
on: [pull_request]
permissions:
  contents: read
  pull-requests: write
  issues: write
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: scale-sync/open-review@v1.12.3
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          openai-api-key: ${{ secrets.OPENAI_API_KEY }}
```

Use the key for the model selected by `main-model` or `MAIN_MODEL`. The Action also accepts `anthropic-api-key` and `gemini-api-key`.

## Review controls

`provider`, `main-model`, and `base-url` choose the model endpoint. `effort` accepts `low`, `medium`, or `high`. `prompt-tokens`, `output-tokens`, `total-tokens`, `request-seconds`, and `group-seconds` bound the review. `instruction` adds guidance for the run.

The Action reads repository review rules at the pull request head. It does not read a runner user's local rules. Repository linters are disabled by default on Action events; only enable `OPEN_REVIEW_TRUSTED_LINTERS=true` for a trusted checkout.

## Results

The review summary tracks open and resolved findings. Inline comments are posted only for findings whose positions match added lines on the reviewed head commit. An uncertain finding remains in the summary for inspection.

Live model quality and GitHub comment posting require a test pull request with model credentials. See [Getting started](/docs/getting-started) for the local path.

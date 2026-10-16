# CI review adapters

Each example runs the same Go CLI once and archives `review.json` plus `review.sarif`. Copy this `integrations/ci` folder into the host repository, place the example at the platform's pipeline path, provide a model API key as a masked CI secret, and make Go 1.23 or later available. The examples install the CLI from the pinned v1.6.0 source tag. The runner accepts `OPEN_REVIEW_BIN` for that executable; without it, the runner uses the local source checkout. It never prints the key.

- GitLab: copy `gitlab/example.gitlab-ci.yml` into `.gitlab-ci.yml`. Merge request pipelines provide the diff base SHA and head SHA. Full Git history is requested so both refs resolve.
- GitFlic: copy `gitflic/gitflic-ci.yaml` to the project root. It fetches the default branch and compares it with the pipeline commit.
- Gerrit: adapt `gerrit/Jenkinsfile` in a Jenkins job with the Gerrit Trigger plugin and a checkout of the patch set. It compares the patch set with its first parent.

All three archive the report; they do not post comments. A partial review exits 2 and still writes both artifacts. Model or input failures exit 1. Keep write-capable platform credentials out of these jobs. GitHub code scanning can upload `review.sarif` with `github/codeql-action/upload-sarif` and `security-events: write`; repository code scanning availability depends on the host plan and settings.

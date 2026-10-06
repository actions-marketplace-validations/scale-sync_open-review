# Getting started with open-review

v0.6.0 is a source release. Build it locally to review workspace changes, compare branches, or review one commit, with visible file coverage.

```bash
git clone https://github.com/scale-sync/open-review.git
cd open-review
pnpm install --frozen-lockfile
pnpm build
go run ./cmd/open-review review --workspace
go run ./cmd/open-review review --workspace --preview
```

Use `--repo /path/to/repository` to review a different checkout. For a nonempty review, set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GEMINI_API_KEY`. Clean workspaces need no key.

```bash
go run ./cmd/open-review review --repo /path/to/repository --base main --head feature-branch
go run ./cmd/open-review review --repo /path/to/repository --commit HEAD --output json
```

See [setup](SETUP.md) for requirements and tests. The existing GitHub pull request CLI is available from source via `node cli/dist/main.mjs review --url <PR-URL>`. The GitHub Action is not packaged for direct use in this release. No package registry installation is available yet.

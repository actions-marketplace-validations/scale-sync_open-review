# Getting started with open-review

v1.5.0 runs fast and deep reviews in Go. Build it locally to review workspace changes, compare branches, review one commit, or scan whole files without Git, with visible file coverage and bounded model use.

```bash
git clone https://github.com/scale-sync/open-review.git
cd open-review
pnpm install --frozen-lockfile
pnpm build
go run ./cmd/open-review review --workspace
go run ./cmd/open-review review --workspace --preview
go run ./cmd/open-review rules check --file review/review.go
go run ./cmd/open-review review --workspace --deep
go run ./cmd/open-review review --workspace --effort high
go run ./cmd/open-review config setup
go run ./cmd/open-review config test
```

Use `--repo /path/to/repository` to review a different checkout. For a nonempty review, set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GEMINI_API_KEY`. Clean workspaces need no key.

```bash
go run ./cmd/open-review review --repo /path/to/repository --base main --head feature-branch
go run ./cmd/open-review review --repo /path/to/repository --commit HEAD --output json
```

See [setup](SETUP.md) for requirements and tests. The pull request CLI is available from source via `node cli/dist/main.mjs review --url <PR-URL>`. The bundled GitHub Action is available at `scale-sync/open-review/action@v1.5.0` for Linux x64. No package registry installation is available yet.

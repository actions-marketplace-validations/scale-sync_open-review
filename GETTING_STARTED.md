# Getting started with open-review

v1.12.3 runs fast and deep reviews in Go. Build it locally to review workspace changes, compare branches, review one commit, or scan whole files without Git, with visible file coverage, SARIF export, bounded model use, public SDKs, and agent integrations.

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

See [setup](SETUP.md) for requirements and tests. The pull request CLI is available from source via `node cli/dist/main.mjs review --url <PR-URL>`. The bundled GitHub Action is available at `scale-sync/open-review@v1.12.3` for Linux x64. SDK packages are available from [npm](https://www.npmjs.com/package/@scalesync/open-review), [PyPI](https://pypi.org/project/openreview-sdk/), [crates.io](https://crates.io/crates/openreview-sdk), and [pkg.go.dev](https://pkg.go.dev/github.com/scale-sync/open-review/sdk/go). Agent plugin installation is documented in [`MARKETPLACES.md`](MARKETPLACES.md).

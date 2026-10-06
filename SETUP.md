# open-review setup

## Requirements

- Git, Go 1.23+, Node.js 20+, and pnpm 10+
- One supported model API key for a nonempty review: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GEMINI_API_KEY`

## Build from source

```bash
git clone https://github.com/scale-sync/open-review.git
cd open-review
pnpm install --frozen-lockfile
pnpm build
go test ./...
pnpm typecheck
pnpm test
```

## Review local changes

Run these commands from the source checkout. `--repo` may point to another Git repository.

```bash
# Staged, unstaged, and untracked changes
go run ./cmd/open-review review --repo /path/to/repository --workspace

# Preview selected and skipped files without a model call
go run ./cmd/open-review review --repo /path/to/repository --workspace --preview

# Override default file filters, while keeping unsafe inputs excluded
go run ./cmd/open-review review --repo /path/to/repository --workspace --include '**/*.go' --exclude 'vendor/**'

# Changes introduced by a branch since the merge base
go run ./cmd/open-review review --repo /path/to/repository --base main --head feature-branch

# One commit, compared with its first parent
go run ./cmd/open-review review --repo /path/to/repository --commit HEAD --output json
```

An empty review returns without a model call. For a nonempty review, configure an API key in the environment or in a local `.env` file. The Go CLI runs Git and invokes the built TypeScript reviewer at `cli/dist/local-bridge.mjs`; `OPEN_REVIEW_BRIDGE` can select a different bridge path.

## Existing pull request CLI

The existing TypeScript CLI still reviews a GitHub pull request when provided a URL. It requires `GITHUB_PAT` for private repositories and a model API key.

```bash
node cli/dist/main.mjs review --url https://github.com/owner/repo/pull/123
```

The GitHub Action source remains in `action/`, but v0.6.0 does not package it for direct use. There is no npm, PyPI, crates.io, or Go registry publication in this release.

## Test and troubleshoot

```bash
pnpm lint
pnpm build
pnpm typecheck
pnpm test
go test ./...
```

If `go run` reports that the bridge is unavailable, run `pnpm build` from the source checkout. If a nonempty review reports missing model configuration, set one supported API key. See [review behavior](review/README.md) and [the release notes](_plan_docs/releases/v0.6.0.md).

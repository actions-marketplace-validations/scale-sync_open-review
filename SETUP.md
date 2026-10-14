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

# Spend fewer model calls on a quick review, or allow more context rounds
go run ./cmd/open-review review --repo /path/to/repository --workspace --effort low
```

An empty review returns without a model call. For a nonempty review, set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GEMINI_API_KEY` in the environment. The Go CLI reads Git changes, selects files, calls the model, and checks findings directly.

## Existing pull request CLI

The existing TypeScript CLI still reviews a GitHub pull request when provided a URL. It requires `GITHUB_PAT` for private repositories and a model API key.

```bash
node cli/dist/main.mjs review --url https://github.com/owner/repo/pull/123
```

The GitHub Action lives in `action/` and includes a bundled adapter plus a Linux x64 Go executable. Use `scale-sync/open-review/action@v1.4.0` from a Linux x64 runner. It does not install dependencies when the event runs. Install Deno in an earlier workflow step to enable sandbox checks for deep review. There is no npm, PyPI, crates.io, or Go registry publication in this release.

## Test and troubleshoot

```bash
pnpm lint
pnpm build
pnpm typecheck
pnpm test
go test ./...
```

If a nonempty review reports a missing model API key, set the key for the model selected by `MAIN_MODEL` or run `go run ./cmd/open-review config set --provider openai --model gpt-4o --api-key-stdin`. See [review behavior](review/README.md) and [the release notes](_plan_docs/releases/v1.4.0.md).

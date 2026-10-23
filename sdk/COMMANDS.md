# OpenReview command reference

This is the detailed command guide for the source checkout, CLI, SDKs, provider
keys, tests, and release validation. The root [README](../README.md) keeps the
overview and links here for the complete command set.

## Run from a local clone

```bash
git clone https://github.com/scale-sync/open-review.git
cd open-review
pnpm install --frozen-lockfile
pnpm build
go test ./...
```

Run the Go CLI from the checkout:

```bash
go run ./cmd/open-review review --workspace
go run ./cmd/open-review review --workspace --preview
go run ./cmd/open-review review --base main --head feature-branch
go run ./cmd/open-review review --commit HEAD --output json
go run ./cmd/open-review review --scan src --preview --output json
go run ./cmd/open-review rules check --file review/review.go
```

Useful output and integration commands:

```bash
go run ./cmd/open-review review --workspace --output sarif --output-file review.sarif
go run ./cmd/open-review review --base main --head feature --session-dir /tmp/open-review-sessions
go run ./cmd/open-review sessions list --dir /tmp/open-review-sessions
go run ./cmd/open-review sessions serve --dir /tmp/open-review-sessions
go run ./cmd/open-review mcp serve --repo .
go run ./cmd/open-review acp serve --repo .
```

Use `--repo PATH` to review another local checkout. `--deep`, `--high-recall`,
`--effort`, `--include`, `--exclude`, `--total-tokens`, and `--quiet` can be
combined with the review commands.

## Configure and rotate provider keys

Interactive setup:

```bash
go run ./cmd/open-review config setup
go run ./cmd/open-review config show
go run ./cmd/open-review config test
```

Replace the current provider and key without editing the config file. The key
is read from stdin and is never placed in process arguments:

```bash
printf '%s\n' "$ANTHROPIC_API_KEY" | \
  go run ./cmd/open-review config set \
    --provider anthropic --model claude-sonnet-4-5 --api-key-stdin
```

Provider changes use the same command. Examples:

```bash
# OpenAI-compatible local model
printf '%s\n' 'local-secret' | go run ./cmd/open-review config set \
  --provider openai --model local-model --base-url http://127.0.0.1:1234/v1 --api-key-stdin

# OpenRouter
printf '%s\n' "$OPENROUTER_API_KEY" | go run ./cmd/open-review config set \
  --provider openrouter --model openrouter/free --api-key-stdin

# Enterprise endpoint
printf '%s\n' "$ENTERPRISE_API_KEY" | go run ./cmd/open-review config set \
  --provider azure --model gpt-4o --base-url https://example.openai.azure.com --api-key-stdin
```

For non-interactive use, set a provider-specific variable (`OPENAI_API_KEY`,
`ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `OPENROUTER_API_KEY`,
`AZURE_OPENAI_API_KEY`, or `AWS_BEDROCK_API_KEY`) or the provider-agnostic
`OPEN_REVIEW_API_KEY`. Saved keys live at
`~/.config/open-review/config.json` with mode `0600`.

## SDK installation and use

All SDKs invoke the same compatible `open-review` binary. Put it on `PATH` or
set `OPEN_REVIEW_BIN`.

### TypeScript / Node.js

```bash
npm install open-review
```

```ts
import { configureProvider, review } from 'open-review';

await configureProvider({
  provider: 'anthropic',
  model: 'claude-sonnet-4-5',
  apiKey: process.env.ANTHROPIC_API_KEY!,
});
const result = await review({ repo: '.', mode: 'workspace' });
```

### Python

```bash
pip install openreview-sdk
```

```python
from openreview import ProviderConfigOptions, ReviewOptions, configure_provider, review

configure_provider(ProviderConfigOptions(
    provider="openrouter", model="openrouter/free", api_key="..."
))
result = review(ReviewOptions(repo=".", mode="workspace"))
```

### Rust

```toml
openreview-sdk = "1.12"
```

Use `configure_provider` with `ProviderConfigOptions` to replace the saved
provider before calling `review` or `scan`.

### Go

```bash
go get github.com/scale-sync/open-review/sdk/go
```

```go
err := openreview.ConfigureProvider(openreview.ProviderConfig{
    Provider: "openai", Model: "gpt-4o", APIKey: os.Getenv("OPENAI_API_KEY"),
})
```

## SDK and package checks

```bash
pnpm --dir sdk/typescript build
pnpm --dir sdk/typescript test
PYTHONPATH=sdk/python python3 -m unittest discover -s sdk/python/tests -v
cargo test --manifest-path sdk/rust/Cargo.toml
go test ./sdk/go
```

## Release checks

```bash
git diff --check
go test ./...
pnpm test
pnpm typecheck
pnpm build
git tag --list 'v*' --sort=-version:refname
```

Release notes and the complete product history are in
[`RELEASES.md`](../RELEASES.md).

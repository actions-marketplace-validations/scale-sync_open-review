# Local review engine

`cmd/open-review` is the Go command. `gitcmd` resolves local Git inputs, `agent` checks reviewer output, and `model` defines the versioned JSON request and response. The JSON shape is recorded in `schema/review-v1.json`.

Use `go run ./cmd/open-review review --workspace` from the repository root after `pnpm build`. `--repo` selects a different checkout. Choose one mode: `--workspace`, `--base REF --head REF`, or `--commit REF`. Add `--output json` for machine-readable output.

For nonempty changes, set a supported model API key. The Go process invokes the built TypeScript bridge. It defaults to `cli/dist/local-bridge.mjs` relative to the current directory, or to `local-bridge.mjs` beside a built Go executable. `OPEN_REVIEW_BRIDGE` overrides the path. Empty changes need no API key.

Merge commits compare against their first parent. Untracked files must be regular files of at most 5 MiB each. The current reviewer may shorten very large file diffs to fit the model window; coverage reporting is scheduled for a later release.

# open-review

[![CI](https://github.com/scale-sync/open-review/actions/workflows/ci.yml/badge.svg)](https://github.com/scale-sync/open-review/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Open-source, agentic code review tool. v1.6.0 runs coordinated fast and deep reviews in Go for local Git changes, whole files, and GitHub pull requests, with bounded repository reads, review rounds, per-file rules, precise findings, resumable local sessions, SARIF output, provider setup, and script-friendly results. TypeScript 7 handles GitHub events, review state, and conversation.

## v1.6.0 from source

Requires Go 1.23+, Node.js 20+, pnpm, Git, and a supported model API key for nonempty reviews. No package registry release is available yet.

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
go run ./cmd/open-review review --workspace --include '**/*.go' --exclude 'vendor/**'
go run ./cmd/open-review review --base main --head feature-branch
go run ./cmd/open-review review --commit HEAD --output json
go run ./cmd/open-review config setup
go run ./cmd/open-review config test
go run ./cmd/open-review review --workspace --model gpt-4o --total-tokens 10000 --output json --output-file review.json --quiet
go run ./cmd/open-review review --base main --head feature-branch --session-dir /tmp/open-review-sessions
go run ./cmd/open-review review --scan src --preview --output json
go run ./cmd/open-review review --commit HEAD --output sarif --output-file review.sarif
go run ./cmd/open-review sessions list --dir /tmp/open-review-sessions
go run ./cmd/open-review sessions serve --dir /tmp/open-review-sessions
```

Use `--repo PATH` to inspect another checkout. See [local review details](review/README.md), [CI examples](integrations/ci/README.md), and [v1.6.0 release notes](_plan_docs/releases/v1.6.0.md).

## Current capabilities

- **Go fast review** — local changes and pull requests use one file-selection and model-review path.
- **Go deep review** — up to four investigation and sandbox cycles, followed by validated findings; without Deno, it records an unverified reasoning pass.
- **Linter fusion** — available ESLint, Ruff, Semgrep, ShellCheck, and Gitleaks binaries add changed-line findings to the same Go result.
- **Model choice** — OpenAI, Anthropic, Gemini, and OpenAI-compatible chat endpoints.
- **Predictable runs** — private provider setup, per-run overrides, token and time limits, usage totals, partial status, and atomic output files.
- **Local sessions** — private event history, safe reuse of completed groups, a loopback viewer, and finding triage.
- **Bounded context** — pinned repository reads and search, controlled review rounds, and prompt compression.
- **Explainable rules** — effective instructions resolve per file, with source diagnostics before a model call.
- **GitHub posting** — findings use PR review comments, while the summary preserves prior review state.
- **Codebase-aware chat** — ask questions about bounded PR context via `@open-review`, or use `ask --repo PATH` locally.
- **Team learnings** — the existing PR comment commands can list and remove stored learnings.

## Quick Start

### GitHub Action

The GitHub Action ships with a bundled TypeScript adapter and a Linux x64 Go executable. It runs from `action/` without installing dependencies during the event. A live PR posting smoke test still needs model credentials. Linter execution is off in Actions unless `OPEN_REVIEW_TRUSTED_LINTERS=true` is set for a trusted checkout.

```yaml
on: [pull_request]
permissions:
  contents: read
  pull-requests: write
  issues: write
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: scale-sync/open-review/action@v1.6.0
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          openai-api-key: ${{ secrets.OPENAI_API_KEY }}
```

### CLI (review any PR from your terminal)

From this source checkout after `pnpm build`:

```bash
# Review a PR (Fast mode)
node cli/dist/main.mjs review --url https://github.com/owner/repo/pull/123
node cli/dist/main.mjs review --url https://github.com/owner/repo/pull/123 --provider openai --model gpt-4o --total-tokens 10000 --output json --output-file review.json --quiet
```

For developers working in a local clone:

```bash
# 1. Build the project
pnpm build

# 2. Run a review
node cli/dist/main.mjs review --url https://github.com/owner/repo/pull/123
# OR
node cli/dist/main.mjs review --url https://github.com/owner/repo/pull/123
```

### GitHub Comment Commands

| Command                              | Description                         |
| ------------------------------------ | ----------------------------------- |
| `@open-review review`                | Trigger a fresh Fast mode review    |
| `@open-review rlm`                   | Trigger Deep/RLM mode review        |
| `@open-review <question>`            | Ask a codebase-aware question       |
| `@open-review list learnings`        | List stored learnings for this repo |
| `@open-review forget: <description>` | Delete a stored learning            |

## CLI Commands

### `review` — Review a Pull Request

```bash
node cli/dist/main.mjs review --url <PR-URL> [options]
```

| Option           | Default  | Description                                           |
| ---------------- | -------- | ----------------------------------------------------- |
| `--url <url>`    | required | GitHub PR URL                                         |
| `--mode <mode>`  | `fast`   | Review mode: `fast` or `rlm`                          |
| `--output <fmt>` | `text`   | Output format: `text`, `markdown`, or `json`          |
| `--model <id>`   | —        | Override the LLM model at runtime                     |
| `--provider <p>` | —        | Select OpenAI, Anthropic, or Gemini                   |
| `--base-url <u>` | —        | Use a custom provider endpoint                        |
| `--output-file`  | —        | Atomically save the result to a private file          |
| `--total-tokens` | —        | Limit aggregate model token use                       |
| `--effort`       | `medium` | Choose `low`, `medium`, or `high` review rounds       |
| `--expert`       | off      | Comprehensive SOLID, security, and quality review     |
| `--submit`       | off      | Post findings as GitHub PR comment (inline + summary) |
| `--quiet`        | off      | Suppress progress messages on stderr                  |

**Expert mode** (`--expert`) adds deep analysis covering:

- SOLID principles (single responsibility, open/closed, Liskov, interface segregation, dependency inversion)
- Security (OWASP Top 10, injection, auth/authz, data exposure)
- Code quality (complexity, readability, maintainability, test coverage gaps)
- Performance (algorithmic complexity, resource leaks, unnecessary allocations)

### `ask` — Interactive Q&A

```bash
node cli/dist/main.mjs ask [--url <PR-URL>] [--repo <path>]
```

Opens an interactive REPL for codebase-aware Q&A. Supports commands: `reset`, `history`, `files`, `exit`.

### `traces` — View Review Logs

```bash
node cli/dist/main.mjs traces --list             # List recent traces (last 20)
node cli/dist/main.mjs traces --pr <PR-URL>      # Traces for a specific PR
node cli/dist/main.mjs traces --open <file>      # Pretty-print a trace file
```

The Go result includes coverage, a model trace, and bounded audit events. The older TypeScript trace reader remains available for legacy traces.

### `serve` — API Server

```bash
node cli/dist/main.mjs serve [--port <n>] [--host <host>]
```

Starts an Express.js server for the web UI (Phase 2) and API integrations.

## Configuration

Copy `.env.example` to `.env` and set your API key:

```bash
cp .env.example .env
```

Key settings:

| Variable              | Default       | Description                                              |
| --------------------- | ------------- | -------------------------------------------------------- |
| `OPENAI_API_KEY`      | —             | OpenAI API key                                           |
| `ANTHROPIC_API_KEY`   | —             | Anthropic API key                                        |
| `GEMINI_API_KEY`      | —             | Google Gemini API key                                    |
| `MAIN_MODEL`          | `gpt-4o`      | Primary model for review and chat                        |
| `SUB_MODEL`           | `gpt-4o-mini` | Secondary model for suggestions                          |
| `MAX_FILES`           | `100`         | Max files per review                                     |
| `MAX_ITERATIONS`      | `12`          | Legacy TypeScript RLM loop limit                         |
| `MAX_LLM_CALLS`       | `35`          | Legacy TypeScript RLM call limit                         |
| `DEFAULT_REVIEW_MODE` | `fast`        | Default mode: `fast` or `rlm`                            |
| `INCLUDE_GLOBS`       | —             | File patterns to include (e.g. `src/**/*.ts`)            |
| `EXCLUDE_GLOBS`       | —             | File patterns to exclude (e.g. `dist/**`)                |
| `OPENAI_BASE_URL`     | —             | Custom OpenAI-compatible endpoint URL                    |
| `GITHUB_PAT`          | —             | GitHub Personal Access Token (Optional for public repos) |

See [`.env.example`](.env.example) for the full list with all linter toggles and review behavior settings.

### `.reviewbuddy.yaml` (per-repo, version-controlled)

Drop a `.reviewbuddy.yaml` at the repo root to customize review behavior without touching `.env`. It's read straight from the PR's head commit via the GitHub API — no local checkout needed, so it works from the CLI, the Action, and `@open-review` comment commands alike.

```yaml
profile: chill # chill | assertive (default) — chill skips informational/stylistic nitpicks
path_instructions:
  - path: 'apps/web/**'
    instructions: Favor accessibility findings over micro-optimizations.
    mode: merge # optional: merge or replace lower-priority rules
labeling_instructions: Treat lint-fixable style issues as informational, not investigate.
finishing_touches: '— reviewed by OpenReview'
```

See [`.reviewbuddy.yaml.example`](.reviewbuddy.yaml.example). All fields are optional; an absent or empty file behaves exactly like today's defaults.

## Review Modes

### Fast Mode

Structured model calls over selected diffs. Large diffs are split into bounded chunks, with a 20-call limit. Non-reviewable files are skipped with coverage reasons. When a matching local checkout and linter binary are available, Go adds changed-line diagnostics and merges overlaps with model findings.

Findings are categorized:

| Severity                    | Description                                                    |
| --------------------------- | -------------------------------------------------------------- |
| 🔴 **Bug — Severe**         | Requires immediate fix. Security risk or broken functionality. |
| 🟠 **Bug — Non-severe**     | Should be reviewed. Incorrect but not critical.                |
| 🔍 **Flag — Investigate**   | Warrants closer examination. May or may not be an issue.       |
| ℹ️ **Flag — Informational** | Explanatory annotation. No action required.                    |

### Deep / RLM Mode

Go coordinates up to four reasoning and verification cycles. Deno runs short generated scripts with a private temporary file, a 15-second timeout, limited output, and no access to model keys or network. Go checks final citations against changed lines.

When Deno is unavailable, deep review continues with an explicit `unavailable` audit event; it does not claim executable verification. Install Deno before invoking `@open-review rlm` if sandbox verification is required.

Triggered via `@open-review rlm`, `--mode rlm`, or the Go CLI's `--deep` flag.

## Instruction Files

open-review automatically reads these files from your repository to customize reviews:

1. `REVIEW.md` — project-specific review rules (any directory level)
2. `AGENTS.md`
3. `CLAUDE.md`
4. `.cursorrules`
5. `.windsurfrules`

Files at subdirectory level are scoped to code in that subtree. Go resolves built-in language guidance, optional user rules at `~/.config/open-review/rules.yaml`, repository files and `.reviewbuddy.yaml`, then an explicit `--instruction` in that order. A rule with `mode: replace` clears lower-priority guidance for matching files. Run `open-review rules check --file PATH` or `review --preview --output json` to see each file's effective text and sources without a model call. Include/exclude flags only select files; they are not prompt rules. Total repository rule content is capped at 40KB.

## Tech Stack

| Layer           | Technology                    |
| --------------- | ----------------------------- |
| Review engine   | Go 1.23+                      |
| GitHub and chat | TypeScript 7 / Node.js        |
| LLM providers   | OpenAI, Anthropic, Google     |
| Package manager | pnpm                          |
| Build tool      | tsdown                        |
| Testing         | Vitest                        |
| Linting         | ESLint 10 + Prettier          |
| Sandbox         | Deno (MVP) → Docker (Phase 2) |
| CLI             | commander                     |
| API server      | Express.js                    |

## Development

```bash
# Prerequisites: Node.js ≥ 20, pnpm, Deno ≥ 2.7 (optional, for RLM sandbox)
git clone https://github.com/scale-sync/open-review.git
cd open-review
pnpm install

# Build all packages
pnpm build

# Run tests (376 tests)
pnpm test

# Type checking
pnpm typecheck

# Lint & format
pnpm lint
pnpm format:check
```

See [GETTING_STARTED.md](GETTING_STARTED.md) for the full setup guide, [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines, and [SETUP.md](SETUP.md) for detailed configuration reference.

## Roadmap

- **Phase 1 (MVP)** ✅ — CLI + GitHub Action, Fast + RLM review, codebase chat, learnings, trace logging
- **Phase 2 (Growth)** — Web UI, auto-fix, Jira/Linear integration, 30+ linters, Docker sandbox, Impact Analysis
- **Phase 3 (Enterprise)** — Multi-platform (GitLab, Azure DevOps, Bitbucket), IDE extension, cloud hosting, analytics

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)

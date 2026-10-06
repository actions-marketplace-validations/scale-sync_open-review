# open-review

[![CI](https://github.com/scale-sync/open-review/actions/workflows/ci.yml/badge.svg)](https://github.com/scale-sync/open-review/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Open-source, agentic code review tool. v0.6.0 adds visible file selection and coverage to local Git reviews. The existing TypeScript reviewer still powers model analysis while the Go engine is built out.

## v0.6.0 from source

Requires Go 1.23+, Node.js 20+, pnpm, Git, and a supported model API key for nonempty reviews. No package registry release is available yet.

```bash
git clone https://github.com/scale-sync/open-review.git
cd open-review
pnpm install --frozen-lockfile
pnpm build
go run ./cmd/open-review review --workspace
go run ./cmd/open-review review --workspace --preview
go run ./cmd/open-review review --workspace --include '**/*.go' --exclude 'vendor/**'
go run ./cmd/open-review review --base main --head feature-branch
go run ./cmd/open-review review --commit HEAD --output json
```

Use `--repo PATH` to inspect another checkout. See [local review details](review/README.md) and [v0.6.0 release notes](_plan_docs/releases/v0.6.0.md).

## Existing PR review features

- **Self-hosted** — runs entirely on your infrastructure. Code never leaves your machine except to your chosen LLM API.
- **Dual review modes** — Fast mode (single-shot, < 60s) and Deep/RLM mode (agentic loop with sandboxed code execution).
- **Any LLM** — bring your own API key. OpenAI, Anthropic Claude, Google Gemini, or any OpenAI-compatible endpoint.
- **Full GitHub sync** — findings posted as native PR review comments with inline suggestions.
- **Built-in linters** — ESLint, Ruff, Semgrep, ShellCheck, Gitleaks run in parallel alongside AI review.
- **Codebase-aware chat** — ask questions about your PR with full repo context via `@openreview`.
- **Learns from feedback** — persistent learnings database avoids repeating false positives.
- **Language & framework aware** — detects TypeScript, Python, Go, Rust, React, Next.js, and more for targeted review.

## Quick Start

### GitHub Action

The Action source is retained during migration. It is not packaged as a usable GitHub Action in v0.6.0.

### CLI (review any PR from your terminal)

From this source checkout after `pnpm build`:

```bash
# Review a PR (Fast mode)
node cli/dist/main.mjs review --url https://github.com/owner/repo/pull/123
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

| Command                             | Description                         |
| ----------------------------------- | ----------------------------------- |
| `@openreview review`                | Trigger a fresh Fast mode review    |
| `@openreview rlm`                   | Trigger Deep/RLM mode review        |
| `@openreview <question>`            | Ask a codebase-aware question       |
| `@openreview list learnings`        | List stored learnings for this repo |
| `@openreview forget: <description>` | Delete a stored learning            |

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
| `--expert`       | off      | Comprehensive SOLID, security, and quality review     |
| `--submit`       | off      | Post findings as GitHub PR comment (inline + summary) |
| `--quiet`        | off      | Suppress progress output (only print findings)        |

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

Every review (Fast or RLM) generates a JSON trace at `~/.openreview/traces/` with full audit trail: prompts, responses, findings, duration, and iteration details.

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
| `MAX_ITERATIONS`      | `12`          | Max RLM loop iterations                                  |
| `MAX_LLM_CALLS`       | `35`          | Max LLM calls per RLM session                            |
| `DEFAULT_REVIEW_MODE` | `fast`        | Default mode: `fast` or `rlm`                            |
| `INCLUDE_GLOBS`       | —             | File patterns to include (e.g. `src/**/*.ts`)            |
| `EXCLUDE_GLOBS`       | —             | File patterns to exclude (e.g. `dist/**`)                |
| `OPENAI_BASE_URL`     | —             | Custom OpenAI-compatible endpoint URL                    |
| `GITHUB_PAT`          | —             | GitHub Personal Access Token (Optional for public repos) |

See [`.env.example`](.env.example) for the full list with all linter toggles and review behavior settings.

### `.reviewbuddy.yaml` (per-repo, version-controlled)

Drop a `.reviewbuddy.yaml` at the repo root to customize review behavior without touching `.env`. It's read straight from the PR's head commit via the GitHub API — no local checkout needed, so it works from the CLI, the Action, and `@openreview` comment commands alike.

```yaml
profile: chill # chill | assertive (default) — chill skips informational/stylistic nitpicks
path_instructions:
  - path: 'apps/web/**'
    instructions: Favor accessibility findings over micro-optimizations.
labeling_instructions: Treat lint-fixable style issues as informational, not investigate.
finishing_touches: '— reviewed by OpenReview'
```

See [`.reviewbuddy.yaml.example`](.reviewbuddy.yaml.example). All fields are optional; an absent or empty file behaves exactly like today's defaults.

## Review Modes

### Fast Mode

Single-shot structured LLM call over the diff. Large diffs are automatically chunked by file (~40K chars per chunk) and reviewed in parallel. Non-reviewable files (lock files, generated code, images) are skipped automatically. Runs bundled linters in parallel.

**Smart prompting:**

- Small PRs (≤ 5 files, ≤ 3000 chars) use a compact, focused prompt for high accuracy
- Large PRs use a comprehensive prompt with full category checklist
- File-type detection adapts the reviewer persona (code reviewer / K8s auditor / docs reviewer / config specialist)

Findings are categorized:

| Severity                    | Description                                                    |
| --------------------------- | -------------------------------------------------------------- |
| 🔴 **Bug — Severe**         | Requires immediate fix. Security risk or broken functionality. |
| 🟠 **Bug — Non-severe**     | Should be reviewed. Incorrect but not critical.                |
| 🔍 **Flag — Investigate**   | Warrants closer examination. May or may not be an issue.       |
| ℹ️ **Flag — Informational** | Explanatory annotation. No action required.                    |

### Deep / RLM Mode

Agentic review using LangGraph.js. The LLM reasons about the code, writes verification scripts, executes them in a Deno sandbox, observes results, and repeats — up to `MAX_ITERATIONS`. Every finding includes grounded citations.

When Deno is not installed, RLM automatically operates in reasoning-only mode — still effective, just without sandbox execution.

Triggered via `@openreview rlm` or `--mode rlm`.

## Instruction Files

OpenReview automatically reads these files from your repository to customize reviews:

1. `REVIEW.md` — project-specific review rules (any directory level)
2. `AGENTS.md`
3. `CLAUDE.md`
4. `.cursorrules`
5. `.windsurfrules`

Files at subdirectory level are scoped to code in that subtree. Total instruction content is capped at 40KB.

## Tech Stack

| Layer             | Technology                    |
| ----------------- | ----------------------------- |
| Core language     | TypeScript / Node.js (≥ 20)   |
| LLM orchestration | LangGraph.js                  |
| LLM providers     | OpenAI, Anthropic, Google     |
| Package manager   | pnpm                          |
| Build tool        | tsdown                        |
| Testing           | Vitest                        |
| Linting           | ESLint 10 + Prettier          |
| Sandbox           | Deno (MVP) → Docker (Phase 2) |
| CLI               | commander                     |
| API server        | Express.js                    |

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

# Contributing to open-review

Thank you for your interest in contributing to open-review! This document covers the process for contributing to this project.

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone git@github.com:<your-username>/open-review.git
   cd open-review
   ```
3. **Install dependencies:**
   ```bash
   nvm use
   pnpm install
   ```
4. **Create a branch** for your work:
   ```bash
   git checkout -b feat/your-feature-name
   ```

## Development Workflow

### Build

```bash
pnpm build                          # all workspaces
go build ./cmd/open-review          # Go CLI
pnpm --filter @openreview/web build # website
```

### Test

```bash
pnpm test                                        # all tests
go test ./...                                     # Go tests
pnpm test:watch                                  # watch mode
```

### Manual E2E Testing

```bash
# Review a real PR (fast mode)
GITHUB_TOKEN=ghp_xxx node cli/dist/main.mjs review --url <PR-URL> --output text

# Review + post findings to GitHub
GITHUB_TOKEN=ghp_xxx node cli/dist/main.mjs review --url <PR-URL> --submit

# Run the 10-PR QA suite
node tests/qa/test-10-prs.mjs
```

### Lint & Format

```bash
pnpm lint          # check
pnpm lint:fix      # auto-fix
pnpm format        # format all files
pnpm format:check  # check formatting
```

Run `pnpm lint`, `pnpm typecheck`, `pnpm test`, and `go test ./...` before submitting a PR.

## Project Structure

```
cmd/     # Go CLI entry point
review/  # Go review engine and its tests
core/    # shared TypeScript modules
cli/     # TypeScript pull request CLI
action/  # GitHub Action adapter and bundled runtime
web/     # Next.js website and documentation
```

- The Go engine owns local review selection and finding validation. The TypeScript CLI and Action handle GitHub integration.
- TypeScript packages use ESM (`"type": "module"`).
- ESLint 10 flat config (`eslint.config.js`) — no `.eslintrc` files.
- Vitest for testing — no Jest.
- tsdown for bundling — no tsup.

## Submitting a Pull Request

1. Ensure your branch is up to date with `main`:
   ```bash
   git fetch origin
   git rebase origin/main
   ```
2. Run all checks:
   ```bash
   pnpm lint && pnpm typecheck && pnpm test && go test ./...
   ```
3. Push your branch and open a PR against `main`.
4. Fill in the PR template with a description of your changes.
5. Wait for CI to pass and a maintainer to review.

## Branch Naming

| Prefix      | Use                           |
| ----------- | ----------------------------- |
| `feat/`     | New features                  |
| `fix/`      | Bug fixes                     |
| `refactor/` | Code refactoring              |
| `docs/`     | Documentation only            |
| `test/`     | Test additions or fixes       |
| `chore/`    | Build, CI, dependency updates |

## Commit Messages

Use conventional commits:

```
feat: add copy/move detection to diff parser
fix: handle rate limit 403 in GitHub client
docs: update CLI examples in README
```

## Reporting Issues

Open an issue on GitHub with:

- Steps to reproduce
- Expected vs actual behavior
- Go and Node.js versions, OS, and relevant environment details

## Project Guidance Files

- [CLAUDE.md](CLAUDE.md) — Architecture overview, commands, conventions for AI coding agents
- [REVIEW.md](REVIEW.md) — open-review's own review rules (code style, architecture, testing, security)
- [SETUP.md](SETUP.md) — Detailed setup guide with configuration reference and troubleshooting

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

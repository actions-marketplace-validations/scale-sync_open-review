# open-review architecture and low-level design

> Target decision, updated 2026-10-15. The [folder-only tree](Target_Repository_Tree.md) shows the final layout, languages, and first release for each path. Current `core/`, `cli/`, and `action/` are TypeScript migration sources, not proof that the target exists.

## Shape

- `cmd/open-review/` parses commands and shows results. It calls the public `review/` package.
- `review/internal/` contains the Go review jobs: Git input, diffs, model calls, coverage, sessions, scan, and tools. Go's `internal` rule keeps those packages private to `review/`.
- `review/schema/` stores one JSON contract for the CLI, Action, integrations, and SDKs. A version field in the data handles compatibility; there is no release-number folder.
- `actions/github/` receives GitHub events and posts comments in TypeScript 7. `integrations/` and `extensions/` translate other hosts and editor events.
- `web/` is a basic Next.js, TypeScript 7, Tailwind CSS product/docs site. `review/` becomes the public Go SDK at v1.11.0; `sdk/` holds Node, Python, and Rust bindings.

All TypeScript packages use TypeScript 7, including the current GitHub Action and CLI. Future editor UI, VS Code extension, and Node SDK also target TypeScript 7. Kotlin owns the JetBrains plugin. Rust owns its SDK; a later Rust helper requires a measured slow path and a small, tested boundary. The Go engine owns the product fast-review path from v0.7.0 onward.

The flow is `CLI/Action/integration → checked request → one Go review engine → checked result → platform output`. Go owns file selection, coverage, and findings. An adapter never creates a second review algorithm. TypeScript handles the GitHub message boundary; the PR Q&A answer language remains open.

## Ownership

| Area                                       | Owns                                      | Does not own                  |
| ------------------------------------------ | ----------------------------------------- | ----------------------------- |
| `review/internal/gitcmd` and `diff`        | safe Git calls, normalized changed files  | model prompts, GitHub posting |
| `review/internal/agent` and `model`        | review flow, coverage, finding validation | CLI flags, platform APIs      |
| `review/internal/llm`                      | provider calls, retry, usage              | file selection, review state  |
| `review/internal/config/rules`             | deterministic rule order and explanations | model transport               |
| `review/internal/session`                  | append-only trace and safe resume         | credential storage            |
| `review/internal/scan`                     | full-file input to the same engine        | a separate scan engine        |
| `actions/`, `integrations/`, `extensions/` | event translation and presentation        | deciding review findings      |
| `web/` and `sdk/`                          | documentation or typed calls              | a review backend              |

## Shared data

- `ReviewRequest`: contract version, mode, immutable input identity, repository context, limits, and rule fingerprints. No credentials.
- `ChangedFile`: old/new path, change kind, hunks, binary flag, and content reference.
- `ReviewResult`: status, coverage for each selected file, findings, warnings, usage, time, optional session identity.
- `Finding`: stable ID, source, severity, explanation, changed-line location, optional suggestion, and evidence.
- `SessionEvent`: append-only record with input/config identity and redacted data.

Keep the canonical JSON shape in `review/schema/` and run Go/TypeScript compatibility tests on the same examples. Add compatible fields in place; increment the **contract version value** for breaking changes. No directory is named after a contract or product release number.

## Design rules

1. **One owner for review decisions.** v0.5.0–v0.6.0 bridge to the existing TypeScript reviewer. v0.7.0 moves product fast reviews to Go; the old TypeScript implementation remains for comparison until parity is demonstrated. v0.8.0 moves deep review after parity. Delete replaced paths only when all callers pass.
2. **Small boundaries where outside systems enter.** Use narrow replaceable edges for Git execution, model calls, time, and saved sessions. Keep ordinary Go code concrete.
3. **Never hide incomplete work.** Register every selected file before review. Each finishes as reviewed, partial, failed, or skipped with a reason.
4. **Check untrusted input.** Validate paths, refs, model output, schema versions, and GitHub comment positions. Keep tokens out of requests, traces, and logs.
5. **Limit work.** One owner counts time and token budgets. Parallel work and retries have caps and support cancellation.
6. **Protect saved work.** Validate resume identity, write output atomically, and avoid erasing existing conversations or editor changes.

## Migration checks

| Current TypeScript area                     | Planned destination                              | Proof needed                                         |
| ------------------------------------------- | ------------------------------------------------ | ---------------------------------------------------- |
| `cli/src/commands/review.ts`                | `cmd/open-review/` and `review/internal/gitcmd/` | workspace/range/commit Git fixtures                  |
| `core/src/review/fast-review.ts`            | `review/internal/agent/` and `llm/`              | findings, coverage, malformed response, budget cases |
| `core/src/review/rlm-runner.ts`, `sandbox/` | `review/internal/sandbox/`                       | escape, timeout, output parity                       |
| `core/src/review/linters.ts`                | `review/internal/linters/`                       | missing binary, timeout, deduplication               |
| `core/src/config/*`                         | `review/internal/config/`                        | setting and rule precedence                          |
| `core/src/learnings/*`, `trace/*`           | `review/internal/session/` and `feedback/`       | persistence, redaction, deletion                     |
| `core/src/chat/*`, `action/src/*`           | `actions/github/`; answer ownership still open   | webhook, thread, posting smoke cases                 |

Do not move code merely to match the diagram. Inspect callers, exports, fixtures, and documentation before deleting a TypeScript path. Create only folders needed by the release being implemented.

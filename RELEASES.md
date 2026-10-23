# OpenReview release history

This ledger summarizes every numbered release from `v0.5.0` onward. Dates are
GitHub publication dates in UTC. A release with no GitHub publication is marked
as tag-only rather than inferred from commit dates.

| Release | Date | Features | Fixes and enhancements | User impact |
| --- | --- | --- | --- | --- |
| `v1.12.2` — GitHub Marketplace action | 2026-10-23 | Added root-level Marketplace metadata for the GitHub Action. | Preserved the existing nested action path while adding `scale-sync/open-review@v1.12.2`. | Teams can publish and consume OpenReview directly as a GitHub Marketplace action. |
| `v1.12.1` — SDK registry publication | 2026-10-23 | Published the Rust, TypeScript, and Python SDKs to their public registries. | Added scoped npm publication, retry-safe registry checks, and tagged Go SDK verification. | Developers can install the SDKs from public package registries or the Go module. |
| `v0.5.0` — local Git review | 2026-10-15 | Workspace, branch, and single-commit review modes. | Defined rename, deletion, merge, root-commit, and empty-worktree behavior. | Review local changes without a PR or model call for empty input. |
| `v0.6.0` — visible review scope | 2026-10-15 | Full diff fidelity, include/exclude selection, coverage manifests, and preview. | Unsafe paths, binaries, generated files, and size limits are explicit skip reasons. | Users can see exactly what will be reviewed before spending model tokens. |
| `v0.7.0` — Go fast review | 2026-10-15 | Go review engine, GitHub Action adapter, and PR/local conversation. | Added changed-line validation, bundled Action execution, and bounded context reads. | Faster, more predictable local and pull-request reviews. |
| `v0.8.0` — deep verification | 2026-10-15 | Deep review, sandboxed verification, linter fusion, and review continuity. | Added retry-safe summaries, stable finding markers, and secret-safe linter output. | More actionable findings with continuity across pushes. |
| `v0.9.0` — provider setup | 2026-10-15 | Provider setup, output contracts, completions, and budget controls. | Added redacted config display, atomic output, partial statuses, and deadlines. | Users can select providers and control cost, time, and automation behavior. |
| `v1.0.0` — coordinated review | 2026-10-16 | Related-file grouping, bounded parallelism, and conditional planning. | Preserved deterministic file assignment and safe partial results under failures. | Large changes review faster while keeping coverage understandable. |
| `v1.1.0` — controlled context | 2026-10-16 | Read-only repository tools, iterative review rounds, and context compression. | Pinned revisions, path checks, size bounds, and changed-line validation prevent unsafe reads. | Reviews gain relevant context without exposing a shell or unbounded repository data. |
| `v1.2.0` — explainable rules | 2026-10-16 | Per-file rule precedence, language/path rules, and rule diagnostics. | Merge/replace behavior and preview parity make effective instructions inspectable. | Teams can understand and control why a file receives specific review guidance. |
| `v1.3.0` — actionable comments | 2026-10-16 | Comment relocation, finding verification, and code suggestions. | Duplicate, missing, and unchanged-line citations are filtered or made visibly uncertain. | PR comments point to valid changed lines and can include safe suggestions. |
| `v1.4.0` — resumable sessions | 2026-10-16 | Append-only traces, safe resume, and session inspection/export commands. | Prompts and credentials stay out of session events; reuse invalidates on changed inputs. | Long reviews can resume without repeating completed model work. |
| `v1.5.0` — whole-file review | 2026-10-16 | Full-file scans, local session viewer, and finding triage. | Private exports, HTML escaping, and guarded PR-thread transitions protect local state. | Users can review non-Git inputs and track findings through resolution. |
| `v1.6.0` — CI delivery | 2026-10-16 | Mature GitHub delivery, GitLab/GitFlic/Gerrit examples, and SARIF 2.1.0. | Checkpoints, sticky summaries, inline markers, and partial-result handling reduce CI duplication. | Reviews fit more CI platforms and code-scanning workflows. |
| `v1.7.0` — protocol adapters | 2026-10-16 | MCP client/server and experimental ACP editor adapter. | Shared preview/contract behavior keeps integrations on the same Go engine. | Editors and agent hosts can invoke bounded reviews through standard protocols. |
| `v1.8.0` — developer tools | 2026-10-22 | Host-agent delegation, portable skills, VS Code, and experimental JetBrains integrations. | Preview/delegation avoid model keys and suggestions require confirmation. | Reviews can run inside coding agents and IDE workflows. |
| `v1.9.0` — provider breadth | 2026-10-22 | Azure OpenAI, Bedrock, OpenRouter, retry handling, prompt caching, and telemetry. | Secret path exclusion, token redaction, and provider-specific credential resolution improve privacy. | Enterprise, cloud, and privacy-sensitive deployments gain safer provider choices. |
| `v1.10.0` — deeper review and knowledge | 2026-10-22 | High-recall review, durable learnings, localization, and documentation website. | Learnings expire/prune safely, reject prompt injection, and preserve invariant JSON contracts. | Teams can retain review guidance and run high-risk reviews with broader context. |
| `v1.11.0` — public SDKs | 2026-10-22 | Go, TypeScript, and Python SDK entry points. | Contract validation, cancellation/timeouts, and package artifacts make subprocess use predictable. | Applications can embed OpenReview without reimplementing the review protocol. |
| `v1.12.0` — Rust and binaries | 2026-10-22 | Rust SDK, cross-platform binaries, checksums, manifest, and cross-SDK CI. | SHA-256 install verification and target builds improve distribution integrity. | Users can install a verified binary and integrate from four SDK languages. |

## Release source notes

Detailed scope and verification notes remain in the per-release files under
`_plan_docs/releases/`. The release workflow proves CI and published assets;
live model quality, platform credentials, and registry publication are only
claimed where explicitly verified.

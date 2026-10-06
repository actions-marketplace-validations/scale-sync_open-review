# open-review product direction

> Updated 2026-10-15 for the multi-release roadmap. This record supersedes conflicting identity, locked TypeScript stack, phase roadmap, and npm launch assumptions in `PRD.md`, `OpenReview_Feature_Spec.md`, `Milestones.md`, and `TodoList.md`. Their detailed feature history remains useful; unverified completion claims are not release proof.

## Identity and purpose

- Product, local repository directory, and GitHub repository: `open-review`; target CLI command and Action branding: `open-review`.
- The daily workflow starts with local Git changes, then supports PR review, inline findings, and conversation in GitHub.
- The product remains independently implemented and self-hostable. Product documentation, release notes, commits, and code comments use only our product language and behavior descriptions.

## Architecture

- Go owns review input, deterministic file selection, coverage accounting, budgets, context retrieval, model review orchestration, result validation, resumable state, and the CLI.
- A thin TypeScript 7 GitHub Action adapter handles GitHub event/credential plumbing and calls the Go engine through a versioned JSON contract. Avoid direct internal Go/TypeScript coupling.
- TypeScript 7 is the target across the Action, VS Code extension, shared editor UI, Node SDK, and website. The website uses Next.js and Tailwind CSS if that combination passes a production build; React with TypeScript 7 and Tailwind CSS is the fallback. Existing TypeScript packages are upgraded only with passing type checks, builds, and tests.
- Kotlin owns the JetBrains plugin. Rust owns the Rust SDK. Rust may be used for a measured, isolated slow path later; it does not replace the Go engine or Kotlin plugin by default.
- Migrate the existing TypeScript engine in stages. At each stage, one implementation is authoritative for the migrated path. Remove redundant code after parity is proven.
- Apply SOLID and DRY to actual boundaries: one review schema, one coverage model, small testable Go packages, and explicit adapter interfaces. Do not add generic frameworks or plugin layers without a concrete caller.
- The [repository structure and low-level design](Go_Architecture.md) defines ownership and migration boundaries.

## Feature selection

The roadmap covers local Git review, deterministic coverage, bounded cost, accurate positions, related-file grouping, path-aware rules, controlled context tools, resume, scans, session inspection, protocols, editor integrations, and four SDKs. Preserve PR conversation with citations and suggestions, explicit sandboxed deep review, linter fusion, and GitHub review-state updates. Durable feedback learning is scheduled later because the current local JSON store is not reliably connected to review inputs or ephemeral Action runners.

One release contains exactly three features. The version sequence and individual release plans live in the [release roadmap](../_plan_docs/Go_Dominant_Release_Plan.md).

## Publication

- The sequence begins v0.5.0, v0.6.0, v0.7.0, v0.8.0, v0.9.0, then v1.0.0 and subsequent minor versions. Every version has three features, a tested push, human-written notes, and a GitHub release.
- The release roadmap has three major groups: daily review foundation (v0.5.0–v0.9.0), review depth and workflow (v1.0.0–v1.6.0), and ecosystem and distribution (v1.7.0–v1.12.0).
- Four SDKs follow the third release once the review API is stable: Go, TypeScript 7, Python, and Rust, published through Go modules, npm, PyPI, and crates.io respectively. They are scheduled at v1.11.0–v1.12.0; no registry publication precedes them.
- Resolved in v0.5.0: package metadata follows the release version, and tags create GitHub releases without package registry publication.
- The Go-dominant CLI and Action must be proven by their release gates. Later language SDKs share the same engine and contract.

## Open decisions

- Whether October 5 is only the roadmap baseline or should change Git commit timestamps.
- Binary distribution strategy and exact SDK package names/APIs.
- Provider onboarding defaults for the Go CLI.

# open-review release roadmap

> Current planning index, 2026-10-16. One version equals one three-feature batch, one tested GitHub push, human-written release notes, and one GitHub release. Planned scope is not implemented proof.

## Three major release groups

| Group                               | Versions       | Releases | Features | Outcome                                                                                                                        |
| ----------------------------------- | -------------- | -------: | -------: | ------------------------------------------------------------------------------------------------------------------------------ |
| **I. Daily review foundation**      | v0.5.0–v0.9.0  |        5 |       15 | Review local changes and PRs accurately through a Go-led CLI and Action, while keeping conversation, deep review, and linters. |
| **II. Review depth and workflow**   | v1.0.0–v1.6.0  |        7 |       21 | Add context, rules, precision, resumable sessions, scans, triage, and CI delivery.                                             |
| **III. Ecosystem and distribution** | v1.7.0–v1.12.0 |        6 |       18 | Add protocol, agent, and IDE surfaces; operational controls; then four public SDKs.                                            |

The four SDKs begin only after v0.7.0 and after the public review contract is stable. They are scheduled late so language bindings do not freeze an unstable API. The repository stays Go-dominant; adapters and editor UI use their platform languages. See the [architecture decision](../_decision_docs/Go_Architecture.md).

The [target repository tree](../_decision_docs/Target_Repository_Tree.md) shows the first release and language for every planned area through v1.12.0.

## Release status — 2026-10-16

| Group | Published | Remaining | Next version |
| --- | ---: | ---: | --- |
| I. Daily review foundation | 5 of 5 | 0 | Complete |
| II. Review depth and workflow | 7 of 7 | 0 | Complete |
| III. Ecosystem and distribution | 2 of 6 | 4 | v1.9.0 |
| **Total** | **14 of 18** | **4** | **v1.9.0** |

Tags and GitHub releases are planned through v1.8.0; host-agent delegation and IDE adapter suites passed. Release publication does not prove live model quality or live PR posting. The benchmark runner and scorer are prepared locally, but no complete, adjudicated open-review run exists, so the product leaderboard has no scores. The website and benchmark work are included with this release.

## Complete feature list by release

### I. Daily review foundation — 15 features

| Version             | Feature 1             | Feature 2                    | Feature 3                     |
| ------------------- | --------------------- | ---------------------------- | ----------------------------- |
| [v0.5.0](v0.5.0.md) | Workspace review      | Branch comparison            | Single-commit review          |
| [v0.6.0](v0.6.0.md) | Faithful diff model   | Deterministic file selection | Coverage manifest and preview |
| [v0.7.0](v0.7.0.md) | Go fast review engine | GitHub Action adapter        | PR conversation               |
| [v0.8.0](v0.8.0.md) | Sandboxed deep review | Linter fusion                | Review-state continuity       |
| [v0.9.0](v0.9.0.md) | Provider setup        | Output contracts             | Cost and time controls        |

### II. Review depth and workflow — 21 features

| Version             | Feature 1                 | Feature 2               | Feature 3               |
| ------------------- | ------------------------- | ----------------------- | ----------------------- |
| [v1.0.0](v1.0.0.md) | Related-file grouping     | Bounded parallelism     | Conditional plan phase  |
| [v1.1.0](v1.1.0.md) | Read-only context tools   | Iterative review        | Context compression     |
| [v1.2.0](v1.2.0.md) | Rule precedence           | Language and path rules | Rule diagnostics        |
| [v1.3.0](v1.3.0.md) | Comment relocation        | Finding verification    | Code suggestions        |
| [v1.4.0](v1.4.0.md) | Append-only session trace | Safe resume             | Session CLI             |
| [v1.5.0](v1.5.0.md) | Full-file scan            | Local session viewer    | Finding triage          |
| [v1.6.0](v1.6.0.md) | Mature GitHub delivery    | Other CI paths          | SARIF and code scanning |

### III. Ecosystem and distribution — 18 features

| Version               | Feature 1                 | Feature 2                   | Feature 3                                |
| --------------------- | ------------------------- | --------------------------- | ---------------------------------------- |
| [v1.7.0](v1.7.0.md)   | MCP client                | MCP server                  | Experimental ACP adapter                 |
| [v1.8.0](v1.8.0.md)   | Host-agent delegation     | Portable agent integrations | IDE extensions                           |
| [v1.9.0](v1.9.0.md)   | Broader provider support  | Observability               | Privacy and security controls            |
| [v1.10.0](v1.10.0.md) | Optional high-recall mode | Durable feedback learnings  | Localization, documentation, and website |
| [v1.11.0](v1.11.0.md) | Go SDK                    | TypeScript 7 SDK            | Python SDK                               |
| [v1.12.0](v1.12.0.md) | Rust SDK                  | Binary distribution         | Cross-SDK compatibility                  |

## Release rule

For each version: finish its three features, run the targeted and full relevant suites, record local/CI/live proof separately, write release notes, push the tested commit, tag, and create the GitHub release. No registry publication through v1.10.0. Publishing npm, PyPI, crates.io, and a public Go module occurs only in the SDK releases after package names, account access, public source access, and install tests are ready. The tag workflow has created GitHub-only releases since v0.5.0, with workspace metadata aligned to each release.

The user selected October 5, 2026 for the first batch's author and committer dates. Subsequent batches continue in date order; GitHub releases show their actual publication dates.

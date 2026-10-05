# Target folder structure through v1.12.0

> Folder-only design. The bracket on each line shows the **first release** needing that folder and its language. The roadmap ends at v1.12.0; the earlier “v0.12.0” wording is treated as that endpoint. These are planned paths, not empty folders to create today.

The structure follows a small Go CLI, task-based Go packages, separate editor/protocol adapters, and a website. Our chosen product areas stay visible at the root: `review/`, `actions/`, `web/`, and `sdk/`. **No release number is a folder name.** A contract version is a field in the data, so there is no `v1/` directory.

**Language decision:** use TypeScript 7 for every TypeScript area in this tree. The current `core/`, `cli/`, and `action/` packages still pin an older compiler; migrate them only with passing type checks, builds, and tests. TypeScript 7 improves compiler/editor speed, while the shipped JavaScript still runs on its usual runtime. Go stays the review engine, Kotlin stays the JetBrains plugin language, and Rust owns its SDK. Add Rust to another isolated task only after a measured bottleneck justifies it.

## Review engine and CLI

```text
open-review/
├── cmd/                                      [v0.5.0 | Go]
│   └── open-review/                          [v0.5.0 | Go] command line entry
└── review/                                   [v0.5.0 | Go] one review engine
    ├── internal/                             [v0.5.0 | Go] engine-only packages
    │   ├── agent/                            [v0.5.0 | Go] review flow and coverage
    │   ├── gitcmd/                           [v0.5.0 | Go] safe Git calls
    │   ├── model/                            [v0.5.0 | Go] request, change, result types
    │   ├── diff/                             [v0.6.0 | Go] hunks and file selection
    │   ├── llm/                              [v0.7.0 | Go] provider calls and usage
    │   ├── sandbox/                          [v0.8.0 | Go] bounded deep review
    │   ├── linters/                          [v0.8.0 | Go] linter result adapters
    │   ├── config/                           [v0.9.0 | Go] validated settings
    │   │   ├── providers/                    [v0.9.0 | Go] provider setup/checks
    │   │   ├── rules/                        [v1.2.0 | Go] precedence and matching
    │   │   │   └── guidance/                 [v1.2.0 | Markdown] rule explanations
    │   │   └── tools/                        [v1.7.0 | Go] allowed external tools
    │   ├── tool/                             [v1.1.0 | Go] read-only context tools
    │   ├── session/                          [v1.4.0 | Go] trace and safe resume
    │   ├── scan/                             [v1.5.0 | Go] full-file input
    │   ├── viewer/                           [v1.5.0 | Go/HTML/CSS] local session UI
    │   │   ├── templates/                   [v1.5.0 | HTML]
    │   │   └── static/                      [v1.5.0 | CSS/assets]
    │   │       └── icons/                   [v1.5.0 | assets]
    │   ├── mcp/                              [v1.7.0 | Go] client and server
    │   ├── delegate/                         [v1.8.0 | Go] host-agent handoff
    │   ├── telemetry/                        [v1.9.0 | Go] opt-in signals
    │   └── feedback/                         [v1.10.0 | Go] scoped learnings
    ├── schema/                               [v0.5.0 | JSON] one review data contract
    │   └── examples/                        [v0.5.0 | JSON] shared request/results
    └── testdata/                             [v0.5.0 | data] Git/model cases
        ├── git/                             [v0.5.0 | data] workspace/range/commit
        └── model/                           [v0.7.0 | data] response and failure cases
```

The Go packages mirror actual jobs, not each individual feature. `review/internal/` keeps engine details private. At v1.11.0, the **root `review/` package** becomes the stable public Go SDK; it does not need a second Go implementation under `sdk/`. The existing TypeScript `cli/` and `core/` are migration paths until the matching Go behavior passes tests.

This deeper pass adds folders where content has a different owner: provider/rule/tool configuration, viewer assets, and Git/model fixtures. `agent/`, `gitcmd/`, `diff/`, `llm/`, `session/`, `scan/`, and `tool/` stay flat Go packages until a real dependency or test boundary calls for a split.

## GitHub, CI, protocols, and agents

```text
open-review/
├── actions/                                  [v0.7.0 | TypeScript 7/YAML]
│   └── github/                               [v0.7.0 | TypeScript 7/YAML]
│       └── src/                             [v0.7.0 | TypeScript 7]
│           ├── events/                      [v0.7.0 | TypeScript 7]
│           ├── posting/                     [v0.7.0 | TypeScript 7]
│           └── conversation/                [v0.7.0 | TypeScript 7]
└── integrations/                             [v1.6.0 | mixed]
    ├── ci/                                   [v1.6.0 | YAML/Markdown]
    │   ├── gitlab/                           [v1.6.0 | YAML/Markdown]
    │   ├── bitbucket/                        [v1.6.0 | YAML/Markdown]
    │   └── generic/                          [v1.6.0 | YAML/Markdown]
    ├── acp/                                  [v1.7.0 | Go]
    │   ├── cmd/                              [v1.7.0 | Go]
    │   │   └── open-review-acp/             [v1.7.0 | Go]
    │   └── internal/                         [v1.7.0 | Go]
    │       ├── contract/                    [v1.7.0 | Go] protocol messages
    │       └── adapter/                     [v1.7.0 | Go]
    │           ├── presentation/            [v1.7.0 | Go] progress/findings
    │           ├── transport/               [v1.7.0 | Go] stdio I/O
    │           └── workspace/               [v1.7.0 | Go] path and repo context
    └── agents/                               [v1.8.0 | Markdown/config]
        ├── skill/                            [v1.8.0 | Markdown]
        └── hosts/                            [v1.8.0 | Markdown/config]
            ├── claude-code/                 [v1.8.0 | Markdown/config]
            ├── codex/                       [v1.8.0 | Markdown/config]
            ├── cursor/                      [v1.8.0 | Markdown/config]
            ├── kimi-code/                   [v1.8.0 | Markdown/config]
            └── opencode/                    [v1.8.0 | Markdown/config]
```

The Action receives GitHub events and posts results. Go decides review findings. The PR Q&A answer language remains open; `conversation/` is the GitHub message adapter either way. CI and protocol integrations translate requests; they never duplicate the review engine.

## Editors and website

```text
open-review/
├── extensions/                               [v1.8.0 | TypeScript 7/Kotlin]
│   ├── frontend/                             [v1.8.0 | React/TypeScript 7]
│   │   └── src/                             [v1.8.0 | React/TypeScript 7]
│   │       ├── shared/                      [v1.8.0 | TypeScript 7]
│   │       └── webview/                     [v1.8.0 | React/TypeScript 7]
│   │           ├── components/              [v1.8.0 | React/TypeScript 7]
│   │           ├── views/                   [v1.8.0 | React/TypeScript 7]
│   │           └── styles/                  [v1.8.0 | CSS]
│   ├── vscode/                               [v1.8.0 | TypeScript 7]
│   │   ├── src/                             [v1.8.0 | TypeScript 7]
│   │   │   └── extension/                   [v1.8.0 | TypeScript 7]
│   │   │       ├── providers/               [v1.8.0 | TypeScript 7]
│   │   │       └── services/                [v1.8.0 | TypeScript 7]
│   │   └── resources/                       [v1.8.0 | assets]
│   └── jetbrains/                            [v1.8.0 | Kotlin]
│       └── src/                             [v1.8.0 | Kotlin/resources]
│           ├── main/                       [v1.8.0 | Kotlin/resources]
│           │   ├── kotlin/                  [v1.8.0 | Kotlin]
│           │   │   ├── actions/             [v1.8.0 | Kotlin] explicit user actions
│           │   │   ├── services/            [v1.8.0 | Kotlin] CLI bridge
│           │   │   └── toolwindow/          [v1.8.0 | Kotlin] review UI
│           │   └── resources/               [v1.8.0 | assets]
│           └── test/                       [v1.8.0 | Kotlin]
│               └── kotlin/                  [v1.8.0 | Kotlin]
└── web/                                      [v1.10.0 | Next.js/TypeScript 7/Tailwind CSS]
    ├── public/                              [v1.10.0 | assets]
    │   ├── images/                          [v1.10.0 | assets]
    │   └── icons/                           [v1.10.0 | assets]
    └── src/                                 [v1.10.0 | TypeScript 7/Tailwind CSS]
        ├── app/                             [v1.10.0 | Next.js/TypeScript 7]
        │   └── [locale]/                    [v1.10.0 | Next.js/TypeScript 7]
        │       ├── docs/                    [v1.10.0 | Next.js/TypeScript 7]
        │       │   └── [...slug]/           [v1.10.0 | Next.js/TypeScript 7]
        │       ├── install/                 [v1.10.0 | Next.js/TypeScript 7]
        │       └── releases/                [v1.10.0 | Next.js/TypeScript 7]
        │           └── [version]/           [v1.10.0 | Next.js/TypeScript 7]
        ├── components/                      [v1.10.0 | React/TypeScript 7]
        ├── content/                         [v1.10.0 | Markdown]
        │   └── docs/                        [v1.10.0 | Markdown]
        │       └── [locale]/                [v1.10.0 | Markdown]
        │           ├── cli/                 [v1.10.0 | Markdown]
        │           ├── action/              [v1.10.0 | Markdown]
        │           ├── integrations/        [v1.10.0 | Markdown]
        │           └── sdk/                 [v1.11.0 | Markdown]
        ├── i18n/                            [v1.10.0 | TypeScript 7]
        └── styles/                          [v1.10.0 | Tailwind CSS]
```

The website is a basic product/docs site, with no review backend of its own. **TypeScript 7 stays required.** Use a Next.js version that passes development, type checking, and production build with TypeScript 7; if none does at implementation time, use React + TypeScript 7 + Tailwind CSS. [TypeScript 7 release](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/); [Next.js compatibility discussion](https://github.com/vercel/next.js/discussions/95633).

## SDKs, tests, docs, and release support

```text
open-review/
├── sdk/                                      [v1.11.0 | mixed]
│   ├── node/                                 [v1.11.0 | TypeScript 7]
│   │   ├── src/                             [v1.11.0 | TypeScript 7]
│   │   └── test/                            [v1.11.0 | TypeScript 7]
│   ├── python/                               [v1.11.0 | Python]
│   │   ├── src/                             [v1.11.0 | Python]
│   │   │   └── open_review/                 [v1.11.0 | Python]
│   │   └── tests/                           [v1.11.0 | Python]
│   └── rust/                                 [v1.12.0 | Rust]
│       ├── src/                             [v1.12.0 | Rust]
│       └── tests/                           [v1.12.0 | Rust]
├── tests/                                    [v0.5.0 | mixed]
│   ├── contract/                            [v0.5.0 | Go/TypeScript 7]
│   ├── actions/                             [v0.7.0 | TypeScript 7]
│   │   └── github/                          [v0.7.0 | TypeScript 7]
│   ├── protocols/                           [v1.7.0 | Go]
│   │   ├── mcp/                             [v1.7.0 | Go]
│   │   └── acp/                             [v1.7.0 | Go]
│   ├── editors/                             [v1.8.0 | TypeScript 7/Kotlin]
│   │   ├── vscode/                          [v1.8.0 | TypeScript 7]
│   │   └── jetbrains/                       [v1.8.0 | Kotlin]
│   └── sdk/                                 [v1.11.0 | Go/TypeScript 7/Python/Rust]
│       ├── go/                              [v1.11.0 | Go]
│       ├── node/                            [v1.11.0 | TypeScript 7]
│       ├── python/                          [v1.11.0 | Python]
│       └── rust/                            [v1.12.0 | Rust]
├── docs/                                     [v0.5.0 | Markdown]
│   ├── guides/                              [v0.5.0 | Markdown]
│   │   ├── cli/                             [v0.5.0 | Markdown]
│   │   ├── action/                          [v0.7.0 | Markdown]
│   │   ├── integrations/                    [v1.6.0 | Markdown]
│   │   │   ├── ci/                          [v1.6.0 | Markdown]
│   │   │   ├── protocols/                   [v1.7.0 | Markdown]
│   │   │   └── editors/                     [v1.8.0 | Markdown]
│   │   └── sdk/                             [v1.11.0 | Markdown]
│   ├── architecture/                        [v0.5.0 | Markdown]
│   └── releases/                            [v0.5.0 | Markdown]
├── scripts/                                  [v1.12.0 | shell]
│   └── install/                             [v1.12.0 | shell]
├── .github/                                  [v0.5.0 | YAML]
│   └── workflows/                           [v0.5.0 | YAML]
├── _decision_docs/                          [v0.5.0 | Markdown]
├── _plan_docs/                              [v0.5.0 | Markdown]
└── _research_refs/                          [local only | ignored]
```

Only the first three SDKs appear at v1.11.0: Go at `review/`, Node and Python at `sdk/`. Rust follows at v1.12.0. Every SDK uses the same review logic and contract. Each release creates only the folders its three tested features need; the final tree is a map, not a batch of empty scaffolding. The local reference folder remains excluded from Git.

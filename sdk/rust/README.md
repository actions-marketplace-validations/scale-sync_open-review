# openreview-sdk

Typed Rust client for contract v2. Install a compatible `open-review` binary on `PATH`, or set `OPEN_REVIEW_BIN` to its path. The crate runs the Go CLI; it does not reimplement the reviewer.

```rust
use openreview_sdk::{review, Mode, Options};
let result = review(&Options::new(".", Mode::Workspace))?;
```

Set `OPENAI_API_KEY` (or the selected provider's variable) for CI, or
`OPEN_REVIEW_API_KEY` for a provider-agnostic BYOK setup. In an interactive
terminal, the CLI prompts once and stores the key locally with mode `0600`.

Use `configure_provider` with `ProviderConfigOptions` to rotate providers; the
secret is sent over stdin and never placed in process arguments.

Set `Options::timeout` to terminate a long run. Partial coverage returns a result with `status == "partial"`.

# openreview-sdk

Typed Rust client for contract v2. Install a compatible `open-review` binary on `PATH`, or set `OPEN_REVIEW_BIN` to its path. The crate runs the Go CLI; it does not reimplement the reviewer.

```rust
use openreview_sdk::{review, Mode, Options};
let result = review(&Options::new(".", Mode::Workspace))?;
```

Set `Options::timeout` to terminate a long run. Partial coverage returns a result with `status == "partial"`.

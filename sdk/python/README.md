# openreview-sdk

Typed Python client for contract v2. Install a compatible `open-review` binary on `PATH`, or set `OPEN_REVIEW_BIN` to its path. The package runs the Go CLI and does not contain a review engine.

```python
from openreview import ReviewOptions, review

result = review(ReviewOptions(repo=".", mode="workspace"))
```

Set `OPENAI_API_KEY` (or the selected provider's variable) for CI, or
`OPEN_REVIEW_API_KEY` for a provider-agnostic BYOK setup. In an interactive
terminal, the CLI prompts once and stores the key locally with mode `0600`.

Rotate providers with `configure_provider(ProviderConfigOptions(...))`; the
secret is sent over stdin and never placed in process arguments.

Pass `timeout` to limit the subprocess. Partial coverage returns `status="partial"`; malformed output and failed runs raise `ReviewError`.

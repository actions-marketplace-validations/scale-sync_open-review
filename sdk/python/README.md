# openreview-sdk

Typed Python client for contract v2. Install a compatible `open-review` binary on `PATH`, or set `OPEN_REVIEW_BIN` to its path. The package runs the Go CLI and does not contain a review engine.

```python
from openreview import ReviewOptions, review

result = review(ReviewOptions(repo=".", mode="workspace"))
```

Pass `timeout` to limit the subprocess. Partial coverage returns `status="partial"`; malformed output and failed runs raise `ReviewError`.

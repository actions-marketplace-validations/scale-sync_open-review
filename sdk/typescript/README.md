# @openreview/sdk

Typed Node client for contract v2. Install a compatible `open-review` binary on `PATH`, or set `OPEN_REVIEW_BIN` to its path. The package runs the Go CLI; it does not contain a review engine.

```ts
import { review } from '@openreview/sdk';
const result = await review({ repo: '.', mode: 'workspace' });
```

Pass an `AbortSignal` to cancel. A partial review resolves with `status: 'partial'`; malformed output and failed runs throw `ReviewError`.

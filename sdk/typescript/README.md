# @openreview/sdk

Typed Node client for contract v2. Install a compatible `open-review` binary on `PATH`, or set `OPEN_REVIEW_BIN` to its path. The package runs the Go CLI; it does not contain a review engine.

```ts
import { review } from '@openreview/sdk';
const result = await review({ repo: '.', mode: 'workspace' });
```

The child CLI uses the caller's key. Set `OPENAI_API_KEY` (or another
provider-specific variable) for CI, or `OPEN_REVIEW_API_KEY` for a
provider-agnostic BYOK setup. In an interactive terminal, the CLI prompts once
and stores the key locally with mode `0600`.

Rotate providers without editing config files:

```ts
import { configureProvider } from '@openreview/sdk';
await configureProvider({ provider: 'anthropic', model: 'claude-sonnet-4-5', apiKey: process.env.ANTHROPIC_API_KEY! });
```

Pass an `AbortSignal` to cancel. A partial review resolves with `status: 'partial'`; malformed output and failed runs throw `ReviewError`.

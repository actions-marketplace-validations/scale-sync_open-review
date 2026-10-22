# Durable Feedback Learnings

open-review stores repository-scoped feedback and triage decisions as durable guidance, allowing team preferences to persist across local runs and CI workflows without unreviewed prompt injection.

## Overview

When developers dismiss or accept review findings during triage, or explicitly record team guidance:
- Feedback is stored in repository-scoped JSON under `~/.config/open-review/learnings/<repo>.json` or `OPEN_REVIEW_HOME/learnings/<repo>.json`.
- Each learning includes trigger pattern, guidance finding, origin source (`feedback`, `triage_dismissed`, `triage_accepted`, `manual`), creation timestamp, and expiration date.
- Stale or expired learnings automatically age out (default 90-day TTL).
- Unused learnings are auto-pruned when reaching the 50-item repository capacity.

## Prompt Safety & Injection Defense

To protect the review model from prompt injection:
- Special token sequences (`<|im_start|>`, `<|endoftext|>`, `system:`, ```` ``` ````, null bytes) are blocked.
- Learning entries are strictly validated and length-bounded (max 1000 characters per entry).
- The formatted team learnings section injected into review prompts is capped at ~2,000 tokens (~8,000 characters).

## CLI Commands

### List Learnings

```bash
open-review learnings list
open-review learnings list --json
```

### Add Learning Manually

```bash
open-review learnings add --trigger "null pointer" --finding "Ensure pointers are checked for nil before method invocation"
```

### Delete Learning

```bash
open-review learnings delete <learning-id>
```

### CI State Exchange (Action Runners)

Ephemeral CI runners can export and import learnings to preserve team memory:

```bash
# Export from persistent storage or cache
open-review learnings export --output learnings.json

# Import into fresh CI workspace
open-review learnings import learnings.json
```

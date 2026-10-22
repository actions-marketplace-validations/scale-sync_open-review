# High-Recall Review Mode

High-recall mode is an opt-in review policy designed for high-risk changes, critical security modules, infrastructure code, and major architectural refactors.

## Overview

Normal open-review runs prioritize precision and bounded token usage (default 3 rounds, bounded context). High-recall mode activates extended exploration rounds and broader context verification without changing the core engine:

- **Extended verification rounds**: Runs up to 6 iterative review rounds per file group.
- **Broader context**: Investigates touched files and surrounding dependencies more thoroughly.
- **Explicit high-risk prompt policy**: Guides the model to exhaustively analyze security vulnerabilities, edge cases, regression risks, and failure modes across all changes.
- **Transparent metrics**: Summary and trace JSON explicitly report `"highRecall": true` along with actual token usage and latency.

## Usage

### CLI

To run a review in high-recall mode:

```bash
open-review review --workspace --high-recall
```

Or for a pull request branch comparison:

```bash
open-review review --base main --head feature --high-recall
```

### GitHub Actions

In your workflow file:

```yaml
- name: Run open-review
  uses: scale-sync/open-review/actions/github@v1
  with:
    high_recall: "true"
```

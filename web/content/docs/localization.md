# Localization & Language Support

open-review supports multilingual CLI and guidance output while keeping machine interfaces strictly invariant.

## Supported Locales

- **English (`en`)** — Default
- **Spanish (`es`)**
- **Japanese (`ja`)**
- **Chinese (`zh`)**
- **German (`de`)**

## Stability Guarantees

Across all locales:
- **Command names remain stable**: `open-review review`, `open-review config`, `open-review learnings`, etc.
- **Machine-readable JSON keys remain invariant**: `contractVersion`, `findings`, `summary`, `trace`, `status`.
- **SARIF formats remain standard**: Complies with SARIF 2.1.0 specifications for GitHub Code Scanning and IDEs.

## Configuring Locale

Set the locale via CLI flag:

```bash
open-review review --workspace --locale ja
```

Or configure via environment variables:

```bash
export OPEN_REVIEW_LOCALE=es
# Or standard system locale:
export LANG=ja_JP.UTF-8
```

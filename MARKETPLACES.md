# Agent marketplace packaging

Open Review ships one portable `open-review` skill with manifests for the major agent marketplaces.

## Install from source

- Codex/ChatGPT: add this repository as a marketplace source, or use the repository-local marketplace catalog when available.
- Claude Code: add `https://github.com/scale-sync/open-review.git` as a marketplace source, then install `scale-sync-open-review`.
- Cursor: submit this public repository at <https://cursor.com/marketplace/publish> or install it as a local plugin.
- Gemini CLI: run `gemini extensions install https://github.com/scale-sync/open-review.git --ref main`.

The plugin installs the skill only. Install the CLI separately with:

```bash
go install github.com/scale-sync/open-review/cmd/open-review@v1.12.3
```

The skill previews coverage before delegation and returns actionable findings without editing the reviewed repository.

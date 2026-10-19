# Model selection

open-review supports OpenAI, Anthropic, Gemini, and OpenAI-compatible endpoints. A nonempty review needs the API key for its selected provider. An empty change set returns without a model call.

## Configure a provider

```bash
go run ./cmd/open-review config setup
go run ./cmd/open-review config test
```

For scripts, use `config set --provider NAME --model NAME`. `--api-key-stdin` stores a key in a private user config file. `config show` never prints the key. `--provider`, `--model`, and `--base-url` override the saved selection for one run. Custom endpoints require HTTPS except for local development.

## Review effort

| Effort   | Model rounds per group | When to use it                                |
| -------- | ---------------------: | --------------------------------------------- |
| `low`    |                      1 | A quick pass over a small diff.               |
| `medium` |                      3 | A balanced review with limited context reads. |
| `high`   |                      5 | Changes that need more repository context.    |

These are effort limits, not model quality scores. Related files share a model call when they fit the group limit, and up to three groups can run at once. A large change may use one additional planning call when budget remains.

## Bound cost and time

`--prompt-tokens`, `--output-tokens`, and `--total-tokens` set separate ceilings. `--request-seconds` limits a model call; `--group-seconds` limits the review. Provider usage counts are used when available and otherwise marked as estimated. A budget stop returns `partial` with findings collected so far.

```bash
go run ./cmd/open-review review --workspace --provider openai --model gpt-4o --effort low --total-tokens 10000
```

For a comparison of the supported review paths, see [Features](/features). For measurement criteria, see [Benchmarks](/benchmark).

# Host-agent review

Build the Go CLI with `go build -o open-review ./cmd/open-review` and place it on `PATH`. Then run `bash integrations/agents/install.sh HOST /path/to/target-repo`. Supported hosts are `codex`, `cursor`, `claude`, `kimi`, `opencode`, and `compatible`. The installer copies one portable review skill into that host's project skill directory. No host account or model key is required by the CLI's `delegate` command; the selected host agent performs the review under its own authorization.

The skill first asks the CLI for a preview, then takes a structured task with selected diff and rules. The host may have its own network and model behavior. Review the task before sending repository content to a host service. The installer never changes host settings or sends data by itself.

Test the installation paths with `bash integrations/agents/test.sh`. Host discovery follows their documented skill locations; live host invocation remains to be verified in each installed host.

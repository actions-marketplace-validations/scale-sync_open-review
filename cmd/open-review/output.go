package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func writeOutput(output io.Writer, path string, data []byte) error {
	if path == "" || path == "-" {
		_, err := output.Write(data)
		return err
	}
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, ".open-review-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("save review output: %w", err)
	}
	return nil
}

func runCompletion(args []string, output io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: open-review completion bash|zsh|fish")
	}
	commands := "review review-json sessions rules config completion"
	flags := "--repo --workspace --base --head --commit --scan --deep --effort --instruction --include --exclude --preview --output --output-file --session-dir --quiet --provider --model --base-url --prompt-tokens --output-tokens --total-tokens --request-seconds --group-seconds"
	switch args[0] {
	case "bash":
		_, err := fmt.Fprintf(output, "_open_review_complete() { COMPREPLY=( $(compgen -W '%s %s' -- \"${COMP_WORDS[COMP_CWORD]}\") ); }; complete -F _open_review_complete open-review\n", commands, flags)
		return err
	case "zsh":
		_, err := fmt.Fprintf(output, "#compdef open-review\n_arguments '*: :(%s %s)'\n", commands, flags)
		return err
	case "fish":
		_, err := fmt.Fprintf(output, "complete -c open-review -f -a '%s %s'\n", commands, flags)
		return err
	default:
		return fmt.Errorf("unsupported shell %q", args[0])
	}
}

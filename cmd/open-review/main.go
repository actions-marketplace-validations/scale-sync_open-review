package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/scale-sync/open-review/review"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 || args[0] != "review" {
		return errors.New("usage: open-review review [--repo PATH] [--workspace | --base REF --head REF | --commit REF] [--output text|json]")
	}
	flags := flag.NewFlagSet("review", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", ".", "repository path")
	workspace := flags.Bool("workspace", false, "review working tree")
	base := flags.String("base", "", "base branch or commit")
	head := flags.String("head", "", "head branch or commit")
	commit := flags.String("commit", "", "single commit")
	format := flags.String("output", "text", "text or json")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported output %q", *format)
	}
	mode := "workspace"
	if *commit != "" {
		mode = "commit"
	} else if *base != "" || *head != "" {
		mode = "branch"
	}
	if *workspace && mode != "workspace" || *commit != "" && (*base != "" || *head != "") {
		return errors.New("choose only one review mode")
	}
	req, err := review.Prepare(ctx, review.Options{Repo: *repo, Mode: mode, Base: *base, Head: *head, Commit: *commit})
	if err != nil {
		return err
	}
	result, err := review.Run(ctx, req)
	if err != nil {
		return err
	}
	if *format == "json" {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	_, err = fmt.Fprintf(output, "open-review: %s review (%s)\nRepository: %s\n", req.Mode, result.Status, req.RepoRoot)
	if err != nil {
		return err
	}
	if req.BaseRef != "" {
		fmt.Fprintf(output, "Base: %s (%s)\nHead: %s (%s)\nMerge base: %s\n", req.BaseRef, req.BaseSHA, req.HeadRef, req.HeadSHA, req.MergeBaseSHA)
	} else if req.CommitSHA != "" {
		fmt.Fprintf(output, "Commit: %s\nCompared with: %s\n", req.CommitSHA, req.BaseSHA)
	}
	fmt.Fprintf(output, "Changed files: %d\nFindings: %d\n", len(req.Files), len(result.Findings))
	for _, raw := range result.Findings {
		var finding struct {
			Severity  string `json:"severity"`
			File      string `json:"file"`
			StartLine int    `json:"startLine"`
			Title     string `json:"title"`
		}
		if err := json.Unmarshal(raw, &finding); err != nil {
			return err
		}
		fmt.Fprintf(output, "- [%s] %s:%d %s\n", finding.Severity, finding.File, finding.StartLine, finding.Title)
	}
	return nil
}

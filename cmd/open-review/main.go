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

type patterns []string

func (p *patterns) String() string { return strings.Join(*p, ",") }
func (p *patterns) Set(value string) error {
	*p = append(*p, value)
	return nil
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 || args[0] != "review" {
		return errors.New("usage: open-review review [--repo PATH] [--workspace | --base REF --head REF | --commit REF] [--include GLOB] [--exclude GLOB] [--preview] [--output text|json]")
	}
	flags := flag.NewFlagSet("review", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", ".", "repository path")
	workspace := flags.Bool("workspace", false, "review working tree")
	base := flags.String("base", "", "base branch or commit")
	head := flags.String("head", "", "head branch or commit")
	commit := flags.String("commit", "", "single commit")
	format := flags.String("output", "text", "text or json")
	preview := flags.Bool("preview", false, "show selected and skipped files without calling a model")
	maxFiles := flags.Int("max-files", 100, "maximum selected files")
	maxFileBytes := flags.Int("max-file-bytes", 1<<20, "maximum full patch bytes per file")
	maxPatchBytes := flags.Int("max-patch-bytes", 38_000, "maximum bytes sent per file")
	var include, exclude patterns
	flags.Var(&include, "include", "include file glob; repeatable")
	flags.Var(&exclude, "exclude", "exclude file glob; repeatable")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported output %q", *format)
	}
	if *maxFiles < 1 || *maxFileBytes < 1 || *maxPatchBytes < 1024 || *maxPatchBytes > 38_000 {
		return errors.New("review limits must be positive and max-patch-bytes must be between 1024 and 38000")
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
	req, err := review.Prepare(ctx, review.Options{Repo: *repo, Mode: mode, Base: *base, Head: *head, Commit: *commit, Include: include, Exclude: exclude, MaxFiles: *maxFiles, MaxFileBytes: *maxFileBytes, MaxPatchBytes: *maxPatchBytes})
	if err != nil {
		return err
	}
	if *preview {
		if *format == "json" {
			return json.NewEncoder(output).Encode(struct {
				Mode     string `json:"mode"`
				RepoRoot string `json:"repoRoot"`
				Coverage any    `json:"coverage"`
			}{req.Mode, req.RepoRoot, req.Coverage})
		}
		return printCoverage(output, req.Coverage)
	}
	if len(req.Coverage) > 0 {
		fmt.Fprintln(os.Stderr, "Review scope:")
		_ = printCoverage(os.Stderr, req.Coverage)
	}
	result, reviewErr := review.Run(ctx, req)
	if reviewErr != nil && result.Status == "" {
		return reviewErr
	}
	if *format == "json" {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return err
		}
		return reviewErr
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
	fmt.Fprintf(output, "Changed files: %d\nSelected files: %d\nFindings: %d\n", len(req.Changes), len(req.Files), len(result.Findings))
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
	if err := printCoverage(output, result.Coverage); err != nil {
		return err
	}
	return reviewErr
}

func printCoverage(output io.Writer, entries []review.CoverageEntry) error {
	for _, entry := range entries {
		reason := ""
		if entry.Reason != "" {
			reason = ", " + entry.Reason
		}
		if _, err := fmt.Fprintf(output, "  %s %s (%d/%d bytes%s)\n", entry.State, entry.Path, entry.SentBytes, entry.PatchBytes, reason); err != nil {
			return err
		}
	}
	return nil
}

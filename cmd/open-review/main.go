package main

import (
	"bytes"
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

var errPartial = errors.New("review is partial")

type patterns []string

func (p *patterns) String() string { return strings.Join(*p, ",") }
func (p *patterns) Set(value string) error {
	*p = append(*p, value)
	return nil
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		if errors.Is(err, errPartial) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) > 0 && args[0] == "config" {
		return runConfig(ctx, args[1:], output)
	}
	if len(args) > 0 && args[0] == "rules" {
		return runRules(args[1:], output)
	}
	if len(args) > 0 && args[0] == "completion" {
		return runCompletion(args[1:], output)
	}
	if len(args) > 0 && args[0] == "review-json" {
		return runJSON(ctx, args[1:], os.Stdin, output)
	}
	if len(args) == 0 || args[0] != "review" {
		return errors.New("usage: open-review review|review-json|rules|config|completion")
	}
	flags := flag.NewFlagSet("review", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", ".", "repository path")
	workspace := flags.Bool("workspace", false, "review working tree")
	base := flags.String("base", "", "base branch or commit")
	head := flags.String("head", "", "head branch or commit")
	commit := flags.String("commit", "", "single commit")
	format := flags.String("output", "text", "text or json")
	outputFile := flags.String("output-file", "", "write output atomically to a file")
	quiet := flags.Bool("quiet", false, "suppress progress messages")
	instruction := flags.String("instruction", "", "explicit review instruction")
	provider := flags.String("provider", "", "model provider")
	modelName := flags.String("model", "", "model name")
	baseURL := flags.String("base-url", "", "provider endpoint")
	promptTokens := flags.Int("prompt-tokens", 0, "maximum estimated prompt tokens per request")
	outputTokens := flags.Int("output-tokens", 0, "maximum output tokens per request")
	totalTokens := flags.Int("total-tokens", 0, "aggregate token budget")
	requestSeconds := flags.Int("request-seconds", 0, "per-request timeout")
	groupSeconds := flags.Int("group-seconds", 0, "review deadline")
	preview := flags.Bool("preview", false, "show selected and skipped files without calling a model")
	deep := flags.Bool("deep", false, "run bounded deep verification before review")
	effort := flags.String("effort", "medium", "review rounds: low, medium, or high")
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
	if *effort != "low" && *effort != "medium" && *effort != "high" {
		return fmt.Errorf("unsupported review effort %q", *effort)
	}
	if len(*instruction) > 8_000 {
		return errors.New("instruction exceeds 8 KB")
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
	req, err := review.Prepare(ctx, review.Options{Repo: *repo, Instructions: *instruction, Mode: mode, Base: *base, Head: *head, Commit: *commit, Include: include, Exclude: exclude, MaxFiles: *maxFiles, MaxFileBytes: *maxFileBytes, MaxPatchBytes: *maxPatchBytes})
	if err != nil {
		return err
	}
	if *deep {
		req.ReviewKind = "deep"
	}
	req.ReviewEffort = *effort
	req.Model = review.ModelIdentity{Provider: *provider, Name: *modelName, BaseURL: *baseURL}
	req.Limits = review.Limits{PromptTokens: *promptTokens, OutputTokens: *outputTokens, TotalTokens: *totalTokens, RequestSeconds: *requestSeconds, GroupSeconds: *groupSeconds}
	var rendered bytes.Buffer
	if *preview {
		if *format == "json" {
			if err := json.NewEncoder(&rendered).Encode(struct {
				Mode     string `json:"mode"`
				RepoRoot string `json:"repoRoot"`
				Coverage any    `json:"coverage"`
				Rules    any    `json:"rules"`
			}{req.Mode, req.RepoRoot, req.Coverage, req.RuleDiagnostics}); err != nil {
				return err
			}
		} else {
			if err := printCoverage(&rendered, req.Coverage); err != nil {
				return err
			}
			for _, rule := range req.RuleDiagnostics {
				fmt.Fprintf(&rendered, "Rule %s: %s\n", rule.Path, strings.Join(rule.Sources, " -> "))
			}
		}
		return writeOutput(output, *outputFile, rendered.Bytes())
	}
	if !*quiet && len(req.Coverage) > 0 {
		fmt.Fprintln(os.Stderr, "Review scope:")
		_ = printCoverage(os.Stderr, req.Coverage)
	}
	result, reviewErr := review.Run(ctx, req)
	if reviewErr != nil && result.Status == "" {
		return reviewErr
	}
	if reviewErr != nil && *outputFile != "" {
		return reviewErr
	}
	if *format == "json" {
		encoder := json.NewEncoder(&rendered)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return err
		}
		if err := writeOutput(output, *outputFile, rendered.Bytes()); err != nil {
			return err
		}
		if reviewErr != nil {
			return reviewErr
		}
		if result.Status == "partial" {
			return errPartial
		}
		return nil
	}
	_, err = fmt.Fprintf(&rendered, "open-review: %s review (%s)\nRepository: %s\n", req.Mode, result.Status, req.RepoRoot)
	if err != nil {
		return err
	}
	if req.BaseRef != "" {
		fmt.Fprintf(&rendered, "Base: %s (%s)\nHead: %s (%s)\nMerge base: %s\n", req.BaseRef, req.BaseSHA, req.HeadRef, req.HeadSHA, req.MergeBaseSHA)
	} else if req.CommitSHA != "" {
		fmt.Fprintf(&rendered, "Commit: %s\nCompared with: %s\n", req.CommitSHA, req.BaseSHA)
	}
	fmt.Fprintf(&rendered, "Changed files: %d\nSelected files: %d\nFindings: %d\nTokens: %d prompt + %d output = %d total (%d calls)\n", len(req.Changes), len(req.Files), len(result.Findings), result.Usage.PromptTokens, result.Usage.OutputTokens, result.Usage.TotalTokens, result.Usage.ModelCalls)
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
		fmt.Fprintf(&rendered, "- [%s] %s:%d %s\n", finding.Severity, finding.File, finding.StartLine, finding.Title)
	}
	if err := printCoverage(&rendered, result.Coverage); err != nil {
		return err
	}
	for _, event := range result.Audit {
		if _, err := fmt.Fprintf(&rendered, "  audit %s: %s\n", event.Stage, event.Status); err != nil {
			return err
		}
	}
	if err := writeOutput(output, *outputFile, rendered.Bytes()); err != nil {
		return err
	}
	if reviewErr != nil {
		return reviewErr
	}
	if result.Status == "partial" {
		return errPartial
	}
	return nil
}

func runJSON(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("review-json", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	maxFiles := flags.Int("max-files", 100, "maximum selected files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *maxFiles < 1 {
		return errors.New("invalid review-json arguments")
	}
	raw, err := io.ReadAll(io.LimitReader(input, 10_000_001))
	if err != nil {
		return err
	}
	if len(raw) > 10_000_000 {
		return errors.New("review request exceeds 10 MB")
	}
	var req review.Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return fmt.Errorf("invalid review JSON: %w", err)
	}
	req, err = review.PrepareExternal(req, *maxFiles)
	if err != nil {
		return err
	}
	result, reviewErr := review.Run(ctx, req)
	if reviewErr != nil && result.Status == "" {
		return reviewErr
	}
	if err := json.NewEncoder(output).Encode(result); err != nil {
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

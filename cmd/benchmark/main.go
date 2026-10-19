package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/scale-sync/open-review/benchmarks"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: benchmark inspect|run|score")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	datasetPath := flags.String("dataset", "", "path to dataset.json")
	switch args[0] {
	case "inspect":
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		_, _, counts, hash, err := loadDataset(*datasetPath)
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(struct {
			SHA256 string `json:"sha256"`
			benchmarks.Counts
		}{hash, counts})
	case "score":
		runPath := flags.String("run", "", "review run JSON")
		judgmentsPath := flags.String("judgments", "", "adjudications JSON")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		rows, cases, _, hash, err := loadDataset(*datasetPath)
		if err != nil {
			return err
		}
		var reviewRun benchmarks.Run
		if err := readJSON(*runPath, &reviewRun); err != nil {
			return err
		}
		var judgments []benchmarks.Judgment
		if err := readJSON(*judgmentsPath, &judgments); err != nil {
			return err
		}
		report, err := benchmarks.Score(rows, cases, hash, reviewRun, judgments)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case "run":
		binary := flags.String("binary", "", "path to built open-review executable")
		cache := flags.String("cache", "", "directory for repository clones")
		out := flags.String("out", "", "run JSON checkpoint")
		provider := flags.String("provider", "", "model provider")
		model := flags.String("model", "", "model name")
		effort := flags.String("effort", "medium", "low, medium, or high")
		version := flags.String("version", "", "reviewer version or Git SHA")
		limit := flags.Int("limit", 0, "first N PRs; zero means all")
		execute := flags.Bool("execute", false, "allow repository downloads and model API calls")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		_, cases, counts, hash, err := loadDataset(*datasetPath)
		if err != nil {
			return err
		}
		if *limit < 0 || *limit > len(cases) {
			return errors.New("limit is outside dataset size")
		}
		if *limit > 0 {
			cases = cases[:*limit]
		}
		if !*execute {
			_, err := fmt.Fprintf(output, "Prepared %d of %d PRs, %d annotated rows, dataset SHA-256 %s. No repositories fetched or model calls made.\n", len(cases), counts.PRs, counts.Rows, hash)
			return err
		}
		if *binary == "" || *cache == "" || *out == "" || *provider == "" || *model == "" || *version == "" {
			return errors.New("execute requires binary, cache, out, provider, model, and version")
		}
		if *effort != "low" && *effort != "medium" && *effort != "high" {
			return errors.New("invalid effort")
		}
		return executeRun(output, cases, hash, *binary, *cache, *out, *provider, *model, *effort, *version)
	default:
		return errors.New("usage: benchmark inspect|run|score")
	}
}

func loadDataset(path string) ([]benchmarks.Row, []benchmarks.Case, benchmarks.Counts, string, error) {
	if path == "" {
		return nil, nil, benchmarks.Counts{}, "", errors.New("dataset path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, benchmarks.Counts{}, "", err
	}
	return benchmarks.LoadDataset(data)
}

func readJSON(path string, result any) error {
	if path == "" {
		return errors.New("JSON path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func executeRun(output io.Writer, cases []benchmarks.Case, hash, binary, cache, out, provider, model, effort, version string) error {
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		return err
	}
	run := benchmarks.Run{DatasetSHA256: hash, Reviewer: "open-review", Version: version, Provider: provider, Model: model, Effort: effort}
	if _, err := os.Stat(out); err == nil {
		if err := readJSON(out, &run); err != nil {
			return err
		}
		if run.DatasetSHA256 != hash || run.Version != version || run.Provider != provider || run.Model != model || run.Effort != effort || run.Reviewer != "open-review" {
			return errors.New("checkpoint metadata differs from requested run")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	completed := make(map[string]bool)
	kept := make([]benchmarks.RunCase, 0, len(run.Cases))
	for _, item := range run.Cases {
		if item.Status == "complete" {
			if completed[item.PRURL] {
				return fmt.Errorf("duplicate case in checkpoint: %s", item.PRURL)
			}
			completed[item.PRURL] = true
			kept = append(kept, item)
		}
	}
	run.Cases = kept
	for _, item := range cases {
		if completed[item.PRURL] {
			continue
		}
		result := reviewCase(item, binary, cache, provider, model, effort)
		run.Cases = append(run.Cases, result)
		if err := writeJSON(out, run); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(output, "%s %s (%d findings)\n", result.Status, item.PRURL, len(result.Findings)); err != nil {
			return err
		}
	}
	return nil
}

var githubName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var prNumber = regexp.MustCompile(`^[0-9]+$`)

func repoForPR(prURL string) (string, string, error) {
	parsed, err := url.Parse(prURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" {
		return "", "", fmt.Errorf("invalid GitHub PR URL %q", prURL)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" || !githubName.MatchString(parts[0]) || !githubName.MatchString(parts[1]) || parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." || !prNumber.MatchString(parts[3]) {
		return "", "", fmt.Errorf("invalid GitHub PR URL %q", prURL)
	}
	return "https://github.com/" + parts[0] + "/" + parts[1] + ".git", parts[0] + "__" + parts[1], nil
}

func reviewCase(item benchmarks.Case, binary, cache, provider, model, effort string) benchmarks.RunCase {
	result := benchmarks.RunCase{PRURL: item.PRURL, Status: "failed"}
	cloneURL, name, err := repoForPR(item.PRURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	repoDir := filepath.Join(cache, name)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); errors.Is(err, os.ErrNotExist) {
		if out, err := exec.CommandContext(ctx, "git", "clone", "--filter=blob:none", "--no-checkout", cloneURL, repoDir).CombinedOutput(); err != nil {
			result.Error = fmt.Sprintf("clone: %v: %s", err, out)
			return result
		}
	} else if err != nil {
		result.Error = err.Error()
		return result
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", repoDir, "fetch", "--no-tags", "origin", item.BaseCommit, item.HeadCommit).CombinedOutput(); err != nil {
		result.Error = fmt.Sprintf("fetch: %v: %s", err, out)
		return result
	}
	start := time.Now()
	command := exec.CommandContext(ctx, binary, "review", "--repo", repoDir, "--base", item.BaseCommit, "--head", item.HeadCommit, "--output", "json", "--quiet", "--provider", provider, "--model", model, "--effort", effort)
	stdout, err := command.Output()
	result.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	var review struct {
		Status   string               `json:"status"`
		Usage    benchmarks.Usage     `json:"usage"`
		Findings []benchmarks.Finding `json:"findings"`
	}
	if err := json.Unmarshal(stdout, &review); err != nil {
		result.Error = err.Error()
		return result
	}
	result.Status, result.Usage, result.Findings = review.Status, review.Usage, review.Findings
	if result.Status != "complete" {
		result.Error = "review did not complete"
	}
	return result
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".benchmark-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

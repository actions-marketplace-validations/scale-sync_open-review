package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scale-sync/open-review/review/internal/agent"
	"github.com/scale-sync/open-review/review/internal/config"
	"github.com/scale-sync/open-review/review/internal/deep"
	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/gitcmd"
	"github.com/scale-sync/open-review/review/internal/linter"
	"github.com/scale-sync/open-review/review/internal/llm"
	"github.com/scale-sync/open-review/review/internal/model"
)

type Options struct {
	Repo          string
	Mode          string
	Base          string
	Head          string
	Commit        string
	Include       []string
	Exclude       []string
	MaxFiles      int
	MaxFileBytes  int
	MaxPatchBytes int
}
type Request = model.Request
type Result = model.Result
type CoverageEntry = model.CoverageEntry
type ModelIdentity = model.ModelIdentity
type Limits = model.Limits
type ProviderConfig = config.Provider

func LoadProviderConfig() (ProviderConfig, error)   { return config.Load() }
func SaveProviderConfig(value ProviderConfig) error { return config.Save(value) }
func ProviderConfigPath() (string, error)           { return config.Path() }
func ResolveProviderConfig(identity ModelIdentity) (ProviderConfig, error) {
	return config.Resolve(identity)
}
func TestProvider(ctx context.Context, identity ModelIdentity) error { return llm.Check(ctx, identity) }

func Prepare(ctx context.Context, opts Options) (Request, error) {
	in, err := gitcmd.Prepare(ctx, gitcmd.Options{Repo: opts.Repo, Mode: opts.Mode, Base: opts.Base, Head: opts.Head, Commit: opts.Commit})
	if err != nil {
		return Request{}, err
	}
	req := Request{
		ContractVersion: model.ContractVersion,
		Mode:            in.Mode,
		RepoRoot:        in.RepoRoot,
		BaseRef:         in.BaseRef,
		HeadRef:         in.HeadRef,
		BaseSHA:         in.BaseSHA,
		MergeBaseSHA:    in.MergeBaseSHA,
		HeadSHA:         in.HeadSHA,
		CommitSHA:       in.CommitSHA,
		Parents:         in.Parents,
		Diff:            in.Diff,
		Files:           in.Files,
		Changes:         in.Changes,
	}
	req, err = agent.Select(req, agent.Selection{Include: opts.Include, Exclude: opts.Exclude, MaxFiles: opts.MaxFiles, MaxFileBytes: opts.MaxFileBytes, MaxPatchBytes: opts.MaxPatchBytes})
	if err != nil {
		return Request{}, err
	}
	req.Instructions = config.Instructions(req.RepoRoot, req.Files)
	req.WorktreeRoot = req.RepoRoot
	return req, nil
}

func Run(ctx context.Context, req Request) (Result, error) {
	if req.ReviewEffort != "" && req.ReviewEffort != "low" && req.ReviewEffort != "medium" && req.ReviewEffort != "high" {
		return Result{}, fmt.Errorf("unsupported review effort %q", req.ReviewEffort)
	}
	ctx, cancel, usage, err := llm.WithSession(ctx, req.Limits)
	if err != nil {
		return Result{}, err
	}
	defer cancel()
	var result Result
	if req.ReviewKind == "deep" {
		result, err = deep.Run(ctx, req)
	} else if req.ReviewKind != "" && req.ReviewKind != "fast" {
		return Result{}, fmt.Errorf("unsupported review kind %q", req.ReviewKind)
	} else {
		result, err = agent.Run(ctx, req, llm.Review)
	}
	result.Usage = usage()
	if err != nil || (result.Status != "complete" && result.Status != "partial") {
		return result, err
	}
	findings, audit := linter.Run(ctx, req, result.Findings)
	result.Findings = findings
	result.Audit = append(result.Audit, audit...)
	if len(findings) > 0 {
		var summary map[string]any
		if json.Unmarshal(result.Summary, &summary) == nil {
			counts := map[string]int{"severe": 0, "non-severe": 0, "investigate": 0, "informational": 0}
			for _, raw := range findings {
				var finding model.Finding
				if json.Unmarshal(raw, &finding) == nil {
					counts[finding.Severity]++
				}
			}
			summary["totalFindings"] = len(findings)
			summary["findingsBySeverity"] = counts
			result.Summary, _ = json.Marshal(summary)
		}
		var trace map[string]any
		if json.Unmarshal(result.Trace, &trace) == nil {
			trace["finalFindingsCount"] = len(findings)
			result.Trace, _ = json.Marshal(trace)
		}
	}
	return result, nil
}

func PrepareExternal(req Request, maxFiles int) (Request, error) {
	if req.ContractVersion != model.ContractVersion || req.Mode != "branch" || req.RepoRoot == "" {
		return Request{}, fmt.Errorf("invalid external review request")
	}
	req.WorktreeRoot = ""
	for i := range req.Changes {
		change := &req.Changes[i]
		if change.InputIssue == "" && (change.Patch == "" || !strings.HasPrefix(change.Patch, "diff --git ")) {
			change.InputIssue = "missing_patch"
		}
		*change = diff.FromPatch(*change)
	}
	return agent.Select(req, agent.Selection{MaxFiles: maxFiles})
}

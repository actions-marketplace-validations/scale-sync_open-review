package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/scale-sync/open-review/review/internal/agent"
	"github.com/scale-sync/open-review/review/internal/config"
	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/gitcmd"
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
	return req, nil
}

func Run(ctx context.Context, req Request) (Result, error) {
	return agent.Run(ctx, req, llm.Review)
}

func PrepareExternal(req Request, maxFiles int) (Request, error) {
	if req.ContractVersion != model.ContractVersion || req.Mode != "branch" || req.RepoRoot == "" {
		return Request{}, fmt.Errorf("invalid external review request")
	}
	for i := range req.Changes {
		change := &req.Changes[i]
		if change.InputIssue == "" && (change.Patch == "" || !strings.HasPrefix(change.Patch, "diff --git ")) {
			change.InputIssue = "missing_patch"
		}
		*change = diff.FromPatch(*change)
	}
	return agent.Select(req, agent.Selection{MaxFiles: maxFiles})
}

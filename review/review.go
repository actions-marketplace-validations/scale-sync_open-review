package review

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/scale-sync/open-review/review/internal/agent"
	"github.com/scale-sync/open-review/review/internal/config"
	"github.com/scale-sync/open-review/review/internal/config/rules"
	"github.com/scale-sync/open-review/review/internal/deep"
	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/gitcmd"
	"github.com/scale-sync/open-review/review/internal/linter"
	"github.com/scale-sync/open-review/review/internal/llm"
	"github.com/scale-sync/open-review/review/internal/mcpclient"
	"github.com/scale-sync/open-review/review/internal/model"
	"github.com/scale-sync/open-review/review/internal/sarif"
	"github.com/scale-sync/open-review/review/internal/scan"
	"github.com/scale-sync/open-review/review/internal/session"
	"github.com/scale-sync/open-review/review/internal/verify"
	"github.com/scale-sync/open-review/review/internal/viewer"
)

type Options struct {
	Repo          string
	Instructions  string
	Mode          string
	Base          string
	Head          string
	Commit        string
	ScanPath      string
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
type Session = session.Store
type SessionSummary = session.Summary
type SessionEvent = session.Event
type SessionRecord = session.Record

func OpenSession(dir string, req Request) (*Session, error) { return session.Open(dir, req) }
func WithSession(ctx context.Context, store *Session) context.Context {
	return session.WithStore(ctx, store)
}
func SessionDir() (string, error)                             { return session.DefaultDir() }
func ListSessions(dir, repo string) ([]SessionSummary, error) { return session.List(dir, repo) }
func ReadSession(dir, id, repo string) (SessionRecord, error) { return session.Read(dir, id, repo) }
func DeleteSession(dir, id, repo string) error                { return session.Delete(dir, id, repo) }
func SessionViewer(dir, repo string) http.Handler             { return viewer.Handler(dir, repo) }
func SetFindingTriage(dir, id, repo, findingID, state string) (map[string]string, error) {
	return session.SetTriage(dir, id, repo, findingID, state)
}
func FormatSARIF(result Result) ([]byte, error) { return sarif.Format(result) }

func ConnectMCP(ctx context.Context, path string) (*mcpclient.Client, error) {
	config, err := mcpclient.Load(path)
	if err != nil {
		return nil, err
	}
	return mcpclient.Connect(ctx, config)
}
func WithMCP(ctx context.Context, client *mcpclient.Client) context.Context {
	return mcpclient.WithClient(ctx, client)
}

func LoadProviderConfig() (ProviderConfig, error)   { return config.Load() }
func SaveProviderConfig(value ProviderConfig) error { return config.Save(value) }
func ProviderConfigPath() (string, error)           { return config.Path() }
func ResolveProviderConfig(identity ModelIdentity) (ProviderConfig, error) {
	return config.Resolve(identity)
}
func TestProvider(ctx context.Context, identity ModelIdentity) error { return llm.Check(ctx, identity) }

func Prepare(ctx context.Context, opts Options) (Request, error) {
	if opts.Mode == "scan" {
		root, changes, err := scan.Prepare(opts.Repo, opts.ScanPath)
		if err != nil {
			return Request{}, err
		}
		req := Request{ContractVersion: model.ContractVersion, Instructions: opts.Instructions, Mode: "scan", RepoRoot: root, Changes: changes}
		return selectAndResolve(req, opts)
	}
	in, err := gitcmd.Prepare(ctx, gitcmd.Options{Repo: opts.Repo, Mode: opts.Mode, Base: opts.Base, Head: opts.Head, Commit: opts.Commit})
	if err != nil {
		return Request{}, err
	}
	req := Request{
		ContractVersion: model.ContractVersion,
		Instructions:    opts.Instructions,
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
	return selectAndResolve(req, opts)
}

func selectAndResolve(req Request, opts Options) (Request, error) {
	req, err := agent.Select(req, agent.Selection{Include: opts.Include, Exclude: opts.Exclude, MaxFiles: opts.MaxFiles, MaxFileBytes: opts.MaxFileBytes, MaxPatchBytes: opts.MaxPatchBytes})
	if err != nil {
		return Request{}, err
	}
	req.WorktreeRoot = req.RepoRoot
	req.RuleSources, err = rules.LoadLocal(req.RepoRoot, req.Files)
	if err != nil {
		return Request{}, err
	}
	req.RuleDiagnostics, err = rules.Resolve(req)
	if err != nil {
		return Request{}, err
	}
	return req, nil
}

func Run(ctx context.Context, req Request) (Result, error) {
	if req.ReviewEffort != "" && req.ReviewEffort != "low" && req.ReviewEffort != "medium" && req.ReviewEffort != "high" {
		return Result{}, fmt.Errorf("unsupported review effort %q", req.ReviewEffort)
	}
	if req.RuleDiagnostics == nil {
		var err error
		req.RuleDiagnostics, err = rules.Resolve(req)
		if err != nil {
			return Result{}, err
		}
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
	findings, diagnostics := verify.Findings(req, findings)
	result.Findings = findings
	result.Audit = append(result.Audit, audit...)
	result.Audit = append(result.Audit, diagnostics...)
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
	req, err := agent.Select(req, agent.Selection{MaxFiles: maxFiles})
	if err != nil {
		return Request{}, err
	}
	req.RuleDiagnostics, err = rules.Resolve(req)
	return req, err
}

func CheckRules(repo string, files []string, instructions string) ([]model.RuleDiagnostic, error) {
	root, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	sources, err := rules.LoadLocal(root, files)
	if err != nil {
		return nil, err
	}
	return rules.Resolve(Request{Files: files, Instructions: instructions, RuleSources: sources, WorktreeRoot: root})
}

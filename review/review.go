package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/scale-sync/open-review/review/internal/agent"
	"github.com/scale-sync/open-review/review/internal/gitcmd"
	"github.com/scale-sync/open-review/review/internal/model"
)

type Options = gitcmd.Options
type Request = model.Request
type Result = model.Result

func Prepare(ctx context.Context, opts Options) (Request, error) {
	in, err := gitcmd.Prepare(ctx, opts)
	if err != nil {
		return Request{}, err
	}
	return Request{
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
	}, nil
}

func Run(ctx context.Context, req Request) (Result, error) {
	return agent.Run(ctx, req, runBridge)
}

func runBridge(ctx context.Context, req Request) (model.BridgeResponse, error) {
	path := os.Getenv("OPEN_REVIEW_BRIDGE")
	if path == "" {
		if executable, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(executable), "local-bridge.mjs")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
		if path == "" {
			path = filepath.Join("cli", "dist", "local-bridge.mjs")
		}
	}
	if _, err := os.Stat(path); err != nil {
		return model.BridgeResponse{}, fmt.Errorf("review bridge unavailable at %q; run pnpm build: %w", path, err)
	}
	input, err := json.Marshal(req)
	if err != nil {
		return model.BridgeResponse{}, err
	}
	cmd := exec.CommandContext(ctx, "node", path)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return model.BridgeResponse{}, fmt.Errorf("review bridge: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	var result model.BridgeResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return model.BridgeResponse{}, fmt.Errorf("invalid review bridge output: %w", err)
	}
	return result, nil
}

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestEmptySkipsReviewer(t *testing.T) {
	req := model.Request{ContractVersion: 2, RepoRoot: "/tmp/repo", Files: []string{}}
	result, err := Run(context.Background(), req, func(context.Context, model.Request) (model.BridgeResponse, error) {
		t.Fatal("reviewer should not run for empty changes")
		return model.BridgeResponse{}, nil
	})
	if err != nil || result.Status != "empty" || len(result.Findings) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestRejectsFindingOutsideChangedFiles(t *testing.T) {
	req := model.Request{ContractVersion: 2, RepoRoot: "/tmp/repo", Files: []string{"changed.go"}, Diff: "diff"}
	_, err := Run(context.Background(), req, func(context.Context, model.Request) (model.BridgeResponse, error) {
		return model.BridgeResponse{
			Summary:  json.RawMessage(`{"totalFindings":1}`),
			Findings: []json.RawMessage{json.RawMessage(`{"file":"other.go","startLine":1}`)},
		}, nil
	})
	if err == nil {
		t.Fatal("expected invalid finding error")
	}
}

func TestCoverageAfterSuccessAndFailure(t *testing.T) {
	req := model.Request{
		ContractVersion: 2,
		RepoRoot:        "/tmp/repo",
		Files:           []string{"one.go", "two.go"},
		Diff:            "diff",
		Coverage: []model.CoverageEntry{
			{Path: "one.go", State: "selected"},
			{Path: "two.go", State: "partial", Reason: "budget_truncated"},
			{Path: "three.go", State: "skipped", Reason: "user_excluded"},
		},
	}
	success, err := Run(context.Background(), req, func(context.Context, model.Request) (model.BridgeResponse, error) {
		return model.BridgeResponse{Summary: json.RawMessage(`{"totalFindings":0}`)}, nil
	})
	if err != nil || success.Coverage[0].State != "reviewed" || success.Coverage[1].State != "partial" || success.Coverage[2].State != "skipped" {
		t.Fatalf("success coverage = %+v, %v", success.Coverage, err)
	}
	failure, err := Run(context.Background(), req, func(context.Context, model.Request) (model.BridgeResponse, error) {
		return model.BridgeResponse{}, errors.New("model unavailable")
	})
	if err == nil || failure.Status != "failed" || failure.Coverage[0].State != "failed" || failure.Coverage[1].State != "failed" || failure.Coverage[2].State != "skipped" {
		t.Fatalf("failure coverage = %+v, %v", failure.Coverage, err)
	}
}

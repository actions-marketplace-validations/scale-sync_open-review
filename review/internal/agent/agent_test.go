package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestEmptySkipsReviewer(t *testing.T) {
	req := model.Request{ContractVersion: 1, RepoRoot: "/tmp/repo", Files: []string{}}
	result, err := Run(context.Background(), req, func(context.Context, model.Request) (model.BridgeResponse, error) {
		t.Fatal("reviewer should not run for empty changes")
		return model.BridgeResponse{}, nil
	})
	if err != nil || result.Status != "empty" || len(result.Findings) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestRejectsFindingOutsideChangedFiles(t *testing.T) {
	req := model.Request{ContractVersion: 1, RepoRoot: "/tmp/repo", Files: []string{"changed.go"}, Diff: "diff"}
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

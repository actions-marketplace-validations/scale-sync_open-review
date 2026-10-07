package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

func TestEmptySkipsReviewer(t *testing.T) {
	req := model.Request{ContractVersion: 2, RepoRoot: "/tmp/repo", Files: []string{}}
	result, err := Run(context.Background(), req, func(context.Context, model.Request) (model.ReviewResponse, error) {
		t.Fatal("reviewer should not run for empty changes")
		return model.ReviewResponse{}, nil
	})
	if err != nil || result.Status != "empty" || len(result.Findings) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestFindingMustCiteChangedNewLine(t *testing.T) {
	patch := "diff --git a/changed.go b/changed.go\n@@ -0,0 +1,2 @@\n+good\n context\n"
	req := model.Request{
		ContractVersion: 2, RepoRoot: "/tmp/repo", Files: []string{"changed.go"}, Diff: patch,
		Changes:  []diff.Change{diff.FromPatch(diff.Change{Path: "changed.go", Status: "modified", Patch: patch})},
		Coverage: []model.CoverageEntry{{Path: "changed.go", State: "selected"}},
	}
	for _, test := range []struct{ start, end int }{{1, 1}, {1, 2}, {2, 2}} {
		finding := model.Finding{
			ID: "ai-1", Category: "bug", Severity: "non-severe", File: "changed.go",
			StartLine: test.start, EndLine: test.end, Title: "Bad value", Explanation: "Wrong result",
			Source: "ai", Citations: []model.Citation{{File: "changed.go", StartLine: test.start, EndLine: test.end}},
		}
		raw, _ := json.Marshal(finding)
		result, err := Run(context.Background(), req, func(context.Context, model.Request) (model.ReviewResponse, error) {
			return model.ReviewResponse{Summary: json.RawMessage(`{"totalFindings":1}`), Findings: []json.RawMessage{raw}}, nil
		})
		if test.start == 1 && test.end == 1 && (err != nil || result.Status != "complete") {
			t.Fatalf("valid line: %+v, %v", result, err)
		}
		if test.end == 2 && (err == nil || result.Coverage[0].State != "failed") {
			t.Fatalf("invalid line: %+v, %v", result, err)
		}
	}
}

func TestRejectsFindingOutsideChangedFiles(t *testing.T) {
	req := model.Request{ContractVersion: 2, RepoRoot: "/tmp/repo", Files: []string{"changed.go"}, Diff: "diff"}
	_, err := Run(context.Background(), req, func(context.Context, model.Request) (model.ReviewResponse, error) {
		return model.ReviewResponse{
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
	success, err := Run(context.Background(), req, func(context.Context, model.Request) (model.ReviewResponse, error) {
		return model.ReviewResponse{Summary: json.RawMessage(`{"totalFindings":0}`)}, nil
	})
	if err != nil || success.Coverage[0].State != "reviewed" || success.Coverage[1].State != "partial" || success.Coverage[2].State != "skipped" {
		t.Fatalf("success coverage = %+v, %v", success.Coverage, err)
	}
	failure, err := Run(context.Background(), req, func(context.Context, model.Request) (model.ReviewResponse, error) {
		return model.ReviewResponse{}, errors.New("model unavailable")
	})
	if err == nil || failure.Status != "failed" || failure.Coverage[0].State != "failed" || failure.Coverage[1].State != "failed" || failure.Coverage[2].State != "skipped" {
		t.Fatalf("failure coverage = %+v, %v", failure.Coverage, err)
	}
}

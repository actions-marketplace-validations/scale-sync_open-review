package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/scale-sync/open-review/review/internal/model"
)

type Reviewer func(context.Context, model.Request) (model.BridgeResponse, error)

func Run(ctx context.Context, req model.Request, reviewer Reviewer) (model.Result, error) {
	if req.ContractVersion != model.ContractVersion || req.RepoRoot == "" {
		return model.Result{}, errors.New("invalid review request")
	}
	result := model.Result{
		ContractVersion: model.ContractVersion,
		Request:         req,
		Coverage:        append([]model.CoverageEntry{}, req.Coverage...),
		Findings:        []json.RawMessage{},
	}
	if len(req.Files) == 0 {
		result.Status = "empty"
		if len(req.Changes) > 0 {
			result.Status = "skipped"
		}
		return result, nil
	}
	if req.Diff == "" {
		return failed(result, errors.New("selected files have no diff"))
	}
	response, err := reviewer(ctx, req)
	if err != nil {
		return failed(result, err)
	}
	var summary map[string]json.RawMessage
	if err := json.Unmarshal(response.Summary, &summary); err != nil || summary == nil {
		return failed(result, errors.New("reviewer returned no valid summary"))
	}
	fileSet := make(map[string]bool, len(req.Files))
	for _, file := range req.Files {
		fileSet[file] = true
	}
	for _, raw := range response.Findings {
		var finding model.Finding
		if err := json.Unmarshal(raw, &finding); err != nil || !fileSet[finding.File] || finding.StartLine < 1 {
			return failed(result, fmt.Errorf("reviewer returned invalid finding"))
		}
	}
	result.Status = "complete"
	for i := range result.Coverage {
		if result.Coverage[i].State == "selected" {
			result.Coverage[i].State = "reviewed"
		}
	}
	result.Findings = response.Findings
	if result.Findings == nil {
		result.Findings = []json.RawMessage{}
	}
	result.Summary = response.Summary
	result.Trace = response.Trace
	return result, nil
}

func failed(result model.Result, err error) (model.Result, error) {
	result.Status, result.Error = "failed", err.Error()
	for i := range result.Coverage {
		if result.Coverage[i].State == "selected" || result.Coverage[i].State == "partial" {
			result.Coverage[i].State = "failed"
			result.Coverage[i].Reason = "reviewer_failed"
		}
	}
	return result, err
}

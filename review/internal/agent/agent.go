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
		Findings:        []json.RawMessage{},
	}
	if len(req.Files) == 0 {
		result.Status = "empty"
		return result, nil
	}
	if req.Diff == "" {
		return model.Result{}, errors.New("changed files have no diff")
	}
	response, err := reviewer(ctx, req)
	if err != nil {
		return model.Result{}, err
	}
	var summary map[string]json.RawMessage
	if err := json.Unmarshal(response.Summary, &summary); err != nil || summary == nil {
		return model.Result{}, errors.New("reviewer returned no valid summary")
	}
	fileSet := make(map[string]bool, len(req.Files))
	for _, file := range req.Files {
		fileSet[file] = true
	}
	for _, raw := range response.Findings {
		var finding model.Finding
		if err := json.Unmarshal(raw, &finding); err != nil || !fileSet[finding.File] || finding.StartLine < 1 {
			return model.Result{}, fmt.Errorf("reviewer returned invalid finding")
		}
	}
	result.Status = "complete"
	result.Findings = response.Findings
	if result.Findings == nil {
		result.Findings = []json.RawMessage{}
	}
	result.Summary = response.Summary
	result.Trace = response.Trace
	return result, nil
}

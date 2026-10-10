package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

type Reviewer func(context.Context, model.Request) (model.ReviewResponse, error)

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
		if err := json.Unmarshal(raw, &finding); err != nil || !fileSet[finding.File] || finding.StartLine < 1 || finding.EndLine < finding.StartLine || finding.Title == "" || len(finding.Title) > 120 || finding.Explanation == "" || !validSeverity(finding.Severity) || (finding.Category != "bug" && finding.Category != "flag") || finding.Source != "ai" || len(finding.Citations) != 1 || finding.Citations[0].File != finding.File || finding.Citations[0].StartLine != finding.StartLine || finding.Citations[0].EndLine != finding.EndLine {
			return failed(result, fmt.Errorf("reviewer returned invalid finding"))
		}
		if len(req.Changes) > 0 {
			if finding.EndLine-finding.StartLine > 50 {
				return failed(result, errors.New("reviewer cited too many lines"))
			}
			for line := finding.StartLine; line <= finding.EndLine; line++ {
				if !ChangedLine(req.Changes, finding.File, line) {
					return failed(result, fmt.Errorf("reviewer cited a line outside the diff"))
				}
			}
		}
	}
	result.Status = "complete"
	if response.Partial {
		result.Status = "partial"
	}
	reviewed := make(map[string]bool, len(response.ReviewedFiles))
	for _, file := range response.ReviewedFiles {
		reviewed[file] = true
	}
	for i := range result.Coverage {
		if result.Coverage[i].State == "selected" {
			if response.Partial && !reviewed[result.Coverage[i].Path] {
				result.Coverage[i].State = "partial"
			} else {
				result.Coverage[i].State = "reviewed"
			}
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

func validSeverity(severity string) bool {
	switch severity {
	case "severe", "non-severe", "investigate", "informational":
		return true
	}
	return false
}

func ChangedLine(changes []diff.Change, file string, start int) bool {
	for _, change := range changes {
		if change.Path != file {
			continue
		}
		for _, hunk := range change.Hunks {
			line := hunk.NewStart
			for _, content := range hunk.Lines {
				if strings.HasPrefix(content, "+") && !strings.HasPrefix(content, "+++") && line == start {
					return true
				}
				if strings.HasPrefix(content, "+") || strings.HasPrefix(content, " ") {
					line++
				}
			}
		}
	}
	return false
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

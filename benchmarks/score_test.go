package benchmarks

import (
	"encoding/json"
	"math"
	"testing"
)

func TestScoreRequiresCompleteUniqueAdjudication(t *testing.T) {
	rows := []Row{
		{Language: "Go", PRURL: "https://github.com/a/b/pull/1", BaseCommit: "base", HeadCommit: "head", Note: "real issue", Path: "main.go", Side: "right", FromLine: 9, ToLine: 10, Label: 1},
		{Language: "Go", PRURL: "https://github.com/a/b/pull/1", BaseCommit: "base", HeadCommit: "head", Note: "bad suggestion", Path: "main.go", Side: "right", FromLine: 20, ToLine: 20, Label: 0},
	}
	data, _ := json.Marshal(rows)
	loaded, cases, counts, hash, err := LoadDataset(data)
	if err != nil || counts.PRs != 1 || counts.Valid != 1 || counts.Invalid != 1 {
		t.Fatalf("dataset = %+v, error = %v", counts, err)
	}
	run := Run{DatasetSHA256: hash, Reviewer: "open-review", Version: "test", Provider: "test", Model: "test", Effort: "low", Cases: []RunCase{{PRURL: cases[0].PRURL, Status: "complete", DurationMS: 1200, Usage: Usage{TotalTokens: 40}, Findings: []Finding{{ID: "one", File: "main.go", StartLine: 10, EndLine: 10}, {ID: "two", File: "main.go", StartLine: 20, EndLine: 20}}}}}
	index := 0
	judgments := []Judgment{{PRURL: cases[0].PRURL, FindingID: "one", Verdict: "match", ReferenceRow: &index}, {PRURL: cases[0].PRURL, FindingID: "two", Verdict: "false_positive"}}
	report, err := Score(loaded, cases, hash, run, judgments)
	if err != nil || report.Overall.TruePositive != 1 || report.Overall.FalsePositive != 1 || report.Overall.LocatedMatches != 1 || math.Abs(report.Overall.F1-2.0/3.0) > 0.0001 || report.Overall.MeanDurationMS != 1200 {
		t.Fatalf("report = %+v, error = %v", report.Overall, err)
	}
	if _, err := Score(loaded, cases, hash, run, judgments[:1]); err == nil {
		t.Fatal("missing judgment must fail")
	}
	judgments[1].Verdict, judgments[1].ReferenceRow = "match", &index
	if _, err := Score(loaded, cases, hash, run, judgments); err == nil {
		t.Fatal("reused ground truth must fail")
	}
	run.Cases[0].Status = "partial"
	if _, err := Score(loaded, cases, hash, run, judgments[:1]); err == nil {
		t.Fatal("partial run must fail")
	}
}

func TestLoadDatasetRejectsInconsistentPR(t *testing.T) {
	rows := []Row{
		{Language: "Go", PRURL: "pr", BaseCommit: "a", HeadCommit: "b", Note: "one", Path: "x", Label: 1},
		{Language: "Go", PRURL: "pr", BaseCommit: "a", HeadCommit: "different", Note: "two", Path: "x", Label: 1},
	}
	data, _ := json.Marshal(rows)
	if _, _, _, _, err := LoadDataset(data); err == nil {
		t.Fatal("conflicting PR revisions must fail")
	}
}

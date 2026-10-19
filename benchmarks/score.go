package benchmarks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

type Row struct {
	Language   string `json:"project_main_language"`
	PRURL      string `json:"pr_url"`
	BaseCommit string `json:"pr_source_commit"`
	HeadCommit string `json:"pr_target_commit"`
	Note       string `json:"note"`
	Path       string `json:"path"`
	Side       string `json:"side"`
	FromLine   int    `json:"from_line"`
	ToLine     int    `json:"to_line"`
	Label      int    `json:"label"`
}

type Case struct {
	PRURL      string `json:"prUrl"`
	Language   string `json:"language"`
	BaseCommit string `json:"baseCommit"`
	HeadCommit string `json:"headCommit"`
}

type Finding struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	StartLine   int    `json:"startLine"`
	EndLine     int    `json:"endLine"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
}

type Usage struct {
	PromptTokens int  `json:"promptTokens"`
	OutputTokens int  `json:"outputTokens"`
	TotalTokens  int  `json:"totalTokens"`
	Estimated    bool `json:"estimated"`
}

type RunCase struct {
	PRURL      string    `json:"prUrl"`
	Status     string    `json:"status"`
	DurationMS int64     `json:"durationMs"`
	Usage      Usage     `json:"usage"`
	Findings   []Finding `json:"findings"`
	Error      string    `json:"error,omitempty"`
}

type Run struct {
	DatasetSHA256 string    `json:"datasetSha256"`
	Reviewer      string    `json:"reviewer"`
	Version       string    `json:"version"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Effort        string    `json:"effort"`
	Cases         []RunCase `json:"cases"`
}

type Judgment struct {
	PRURL        string `json:"prUrl"`
	FindingID    string `json:"findingId"`
	Verdict      string `json:"verdict"`
	ReferenceRow *int   `json:"referenceRow,omitempty"`
}

type Counts struct {
	PRs       int `json:"prs"`
	Rows      int `json:"rows"`
	Valid     int `json:"valid"`
	Invalid   int `json:"invalid"`
	Languages int `json:"languages"`
}

type Metrics struct {
	Cases             int     `json:"cases"`
	GroundTruth       int     `json:"groundTruth"`
	Findings          int     `json:"findings"`
	TruePositive      int     `json:"truePositive"`
	FalsePositive     int     `json:"falsePositive"`
	FalseNegative     int     `json:"falseNegative"`
	LocatedMatches    int     `json:"locatedMatches"`
	Precision         float64 `json:"precision"`
	Recall            float64 `json:"recall"`
	F1                float64 `json:"f1"`
	LinePrecision     float64 `json:"linePrecision"`
	NoiseRate         float64 `json:"noiseRate"`
	MeanDurationMS    float64 `json:"meanDurationMs"`
	MeanTotalTokens   float64 `json:"meanTotalTokens"`
	EstimatedUsagePRs int     `json:"estimatedUsagePrs"`
}

type Report struct {
	DatasetSHA256 string             `json:"datasetSha256"`
	Reviewer      string             `json:"reviewer"`
	Version       string             `json:"version"`
	Provider      string             `json:"provider"`
	Model         string             `json:"model"`
	Effort        string             `json:"effort"`
	Overall       Metrics            `json:"overall"`
	ByLanguage    map[string]Metrics `json:"byLanguage"`
}

func LoadDataset(data []byte) ([]Row, []Case, Counts, string, error) {
	var rows []Row
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, nil, Counts{}, "", err
	}
	if len(rows) == 0 {
		return nil, nil, Counts{}, "", errors.New("dataset is empty")
	}
	casesByURL := make(map[string]Case)
	languages := make(map[string]bool)
	counts := Counts{Rows: len(rows)}
	for index, row := range rows {
		if row.PRURL == "" || row.Language == "" || row.BaseCommit == "" || row.HeadCommit == "" || row.Path == "" || row.Note == "" || row.Label < 0 || row.Label > 1 {
			return nil, nil, Counts{}, "", fmt.Errorf("invalid dataset row %d", index)
		}
		if row.Label == 1 {
			counts.Valid++
		} else {
			counts.Invalid++
		}
		languages[row.Language] = true
		candidate := Case{PRURL: row.PRURL, Language: row.Language, BaseCommit: row.BaseCommit, HeadCommit: row.HeadCommit}
		if previous, exists := casesByURL[row.PRURL]; exists && previous != candidate {
			return nil, nil, Counts{}, "", fmt.Errorf("conflicting commits or language for %s", row.PRURL)
		}
		casesByURL[row.PRURL] = candidate
	}
	cases := make([]Case, 0, len(casesByURL))
	for _, item := range casesByURL {
		cases = append(cases, item)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].PRURL < cases[j].PRURL })
	counts.PRs, counts.Languages = len(cases), len(languages)
	hash := sha256.Sum256(data)
	return rows, cases, counts, hex.EncodeToString(hash[:]), nil
}

func Score(rows []Row, cases []Case, datasetSHA string, run Run, judgments []Judgment) (Report, error) {
	if run.DatasetSHA256 != datasetSHA || run.Reviewer == "" || run.Version == "" || run.Provider == "" || run.Model == "" || run.Effort == "" {
		return Report{}, errors.New("run metadata or dataset hash is missing or mismatched")
	}
	if len(run.Cases) != len(cases) {
		return Report{}, fmt.Errorf("run has %d cases, dataset has %d", len(run.Cases), len(cases))
	}
	caseByURL := make(map[string]Case, len(cases))
	for _, item := range cases {
		caseByURL[item.PRURL] = item
	}
	judgmentByFinding := make(map[string]Judgment, len(judgments))
	for _, judgment := range judgments {
		key := judgment.PRURL + "\x00" + judgment.FindingID
		if judgment.PRURL == "" || judgment.FindingID == "" {
			return Report{}, errors.New("judgment lacks a PR URL or finding ID")
		}
		if _, exists := judgmentByFinding[key]; exists {
			return Report{}, fmt.Errorf("duplicate judgment for %s", key)
		}
		judgmentByFinding[key] = judgment
	}
	matched := make(map[int]bool)
	seenCases := make(map[string]bool)
	report := Report{DatasetSHA256: datasetSHA, Reviewer: run.Reviewer, Version: run.Version, Provider: run.Provider, Model: run.Model, Effort: run.Effort, ByLanguage: make(map[string]Metrics)}
	var durationSum, tokenSum int64
	for _, item := range run.Cases {
		caseInfo, exists := caseByURL[item.PRURL]
		if !exists || seenCases[item.PRURL] {
			return Report{}, fmt.Errorf("unknown or duplicate run case %s", item.PRURL)
		}
		seenCases[item.PRURL] = true
		if item.Status != "complete" || item.DurationMS < 0 || item.Usage.TotalTokens < 0 {
			return Report{}, fmt.Errorf("case %s is incomplete or has invalid measurements", item.PRURL)
		}
		metrics := report.ByLanguage[caseInfo.Language]
		metrics.Cases++
		metrics.Findings += len(item.Findings)
		if item.Usage.Estimated {
			metrics.EstimatedUsagePRs++
		}
		metrics.MeanDurationMS += float64(item.DurationMS)
		metrics.MeanTotalTokens += float64(item.Usage.TotalTokens)
		durationSum += item.DurationMS
		tokenSum += int64(item.Usage.TotalTokens)
		if item.Usage.Estimated {
			report.Overall.EstimatedUsagePRs++
		}
		seenFindings := make(map[string]bool)
		for _, finding := range item.Findings {
			if finding.ID == "" || seenFindings[finding.ID] {
				return Report{}, fmt.Errorf("missing or duplicate finding ID in %s", item.PRURL)
			}
			seenFindings[finding.ID] = true
			key := item.PRURL + "\x00" + finding.ID
			judgment, exists := judgmentByFinding[key]
			if !exists {
				return Report{}, fmt.Errorf("finding %s lacks adjudication", key)
			}
			delete(judgmentByFinding, key)
			switch judgment.Verdict {
			case "false_positive":
				if judgment.ReferenceRow != nil {
					return Report{}, fmt.Errorf("false positive %s cannot reference a row", key)
				}
				metrics.FalsePositive++
			case "match":
				if judgment.ReferenceRow == nil || *judgment.ReferenceRow < 0 || *judgment.ReferenceRow >= len(rows) {
					return Report{}, fmt.Errorf("invalid reference row for %s", key)
				}
				index := *judgment.ReferenceRow
				if rows[index].PRURL != item.PRURL || rows[index].Label != 1 || matched[index] {
					return Report{}, fmt.Errorf("invalid or reused ground truth row %d", index)
				}
				matched[index] = true
				metrics.TruePositive++
				if locationMatches(finding, rows[index]) {
					metrics.LocatedMatches++
				}
			default:
				return Report{}, fmt.Errorf("unknown verdict %q", judgment.Verdict)
			}
		}
		report.ByLanguage[caseInfo.Language] = metrics
	}
	if len(judgmentByFinding) != 0 {
		return Report{}, errors.New("adjudication contains findings absent from run")
	}
	for _, row := range rows {
		if row.Label == 1 {
			metrics := report.ByLanguage[row.Language]
			metrics.GroundTruth++
			report.ByLanguage[row.Language] = metrics
		}
	}
	for language, metrics := range report.ByLanguage {
		metrics = finish(metrics)
		report.ByLanguage[language] = metrics
		report.Overall.Cases += metrics.Cases
		report.Overall.GroundTruth += metrics.GroundTruth
		report.Overall.Findings += metrics.Findings
		report.Overall.TruePositive += metrics.TruePositive
		report.Overall.FalsePositive += metrics.FalsePositive
		report.Overall.LocatedMatches += metrics.LocatedMatches
	}
	report.Overall = finish(report.Overall)
	report.Overall.MeanDurationMS = float64(durationSum) / float64(report.Overall.Cases)
	report.Overall.MeanTotalTokens = float64(tokenSum) / float64(report.Overall.Cases)
	return report, nil
}

func finish(metrics Metrics) Metrics {
	metrics.FalseNegative = metrics.GroundTruth - metrics.TruePositive
	if metrics.Findings > 0 {
		metrics.Precision = float64(metrics.TruePositive) / float64(metrics.Findings)
		metrics.LinePrecision = float64(metrics.LocatedMatches) / float64(metrics.Findings)
		metrics.NoiseRate = float64(metrics.FalsePositive) / float64(metrics.Findings)
	}
	if metrics.GroundTruth > 0 {
		metrics.Recall = float64(metrics.TruePositive) / float64(metrics.GroundTruth)
	}
	if metrics.Precision+metrics.Recall > 0 {
		metrics.F1 = 2 * metrics.Precision * metrics.Recall / (metrics.Precision + metrics.Recall)
	}
	if metrics.Cases > 0 {
		metrics.MeanDurationMS /= float64(metrics.Cases)
		metrics.MeanTotalTokens /= float64(metrics.Cases)
	}
	return metrics
}

func locationMatches(finding Finding, row Row) bool {
	if row.Side != "right" || finding.File != row.Path || finding.StartLine < 1 || finding.EndLine < finding.StartLine {
		return false
	}
	start, end := row.FromLine, row.ToLine
	if start > end {
		start, end = end, start
	}
	return start > 0 && end > 0 && finding.StartLine <= end+1 && finding.EndLine+1 >= start
}

package model

import "encoding/json"

const ContractVersion = 1

type Request struct {
	ContractVersion int      `json:"contractVersion"`
	Mode            string   `json:"mode"`
	RepoRoot        string   `json:"repoRoot"`
	BaseRef         string   `json:"baseRef,omitempty"`
	HeadRef         string   `json:"headRef,omitempty"`
	BaseSHA         string   `json:"baseSha,omitempty"`
	MergeBaseSHA    string   `json:"mergeBaseSha,omitempty"`
	HeadSHA         string   `json:"headSha,omitempty"`
	CommitSHA       string   `json:"commitSha,omitempty"`
	Parents         []string `json:"parents,omitempty"`
	Diff            string   `json:"diff"`
	Files           []string `json:"files"`
}

type Finding struct {
	Severity    string `json:"severity"`
	File        string `json:"file"`
	StartLine   int    `json:"startLine"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
}

type BridgeResponse struct {
	Findings []json.RawMessage `json:"findings"`
	Summary  json.RawMessage   `json:"summary"`
	Trace    json.RawMessage   `json:"trace"`
}

type Result struct {
	ContractVersion int               `json:"contractVersion"`
	Status          string            `json:"status"`
	Request         Request           `json:"request"`
	Findings        []json.RawMessage `json:"findings"`
	Summary         json.RawMessage   `json:"summary,omitempty"`
	Trace           json.RawMessage   `json:"trace,omitempty"`
}

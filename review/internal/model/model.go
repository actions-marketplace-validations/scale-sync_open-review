package model

import (
	"encoding/json"

	"github.com/scale-sync/open-review/review/internal/diff"
)

const ContractVersion = 2

type CoverageEntry struct {
	Path       string `json:"path"`
	OldPath    string `json:"oldPath,omitempty"`
	Status     string `json:"status"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
	PatchBytes int    `json:"patchBytes"`
	SentBytes  int    `json:"sentBytes"`
}

type Request struct {
	ContractVersion int             `json:"contractVersion"`
	Mode            string          `json:"mode"`
	RepoRoot        string          `json:"repoRoot"`
	BaseRef         string          `json:"baseRef,omitempty"`
	HeadRef         string          `json:"headRef,omitempty"`
	BaseSHA         string          `json:"baseSha,omitempty"`
	MergeBaseSHA    string          `json:"mergeBaseSha,omitempty"`
	HeadSHA         string          `json:"headSha,omitempty"`
	CommitSHA       string          `json:"commitSha,omitempty"`
	Parents         []string        `json:"parents,omitempty"`
	Diff            string          `json:"diff"`
	Files           []string        `json:"files"`
	Changes         []diff.Change   `json:"changes,omitempty"`
	Coverage        []CoverageEntry `json:"coverage,omitempty"`
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
	Coverage        []CoverageEntry   `json:"coverage"`
	Findings        []json.RawMessage `json:"findings"`
	Summary         json.RawMessage   `json:"summary,omitempty"`
	Trace           json.RawMessage   `json:"trace,omitempty"`
	Error           string            `json:"error,omitempty"`
}

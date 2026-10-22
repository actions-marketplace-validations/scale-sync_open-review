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
	ContractVersion int              `json:"contractVersion"`
	Mode            string           `json:"mode"`
	RepoRoot        string           `json:"repoRoot"`
	BaseRef         string           `json:"baseRef,omitempty"`
	HeadRef         string           `json:"headRef,omitempty"`
	BaseSHA         string           `json:"baseSha,omitempty"`
	MergeBaseSHA    string           `json:"mergeBaseSha,omitempty"`
	HeadSHA         string           `json:"headSha,omitempty"`
	CommitSHA       string           `json:"commitSha,omitempty"`
	Parents         []string         `json:"parents,omitempty"`
	Diff            string           `json:"diff"`
	Files           []string         `json:"files"`
	Title           string           `json:"title,omitempty"`
	Instructions    string           `json:"instructions,omitempty"`
	RuleSources     []RuleSource     `json:"ruleSources,omitempty"`
	RuleDiagnostics []RuleDiagnostic `json:"ruleDiagnostics,omitempty"`
	ReviewKind      string           `json:"reviewKind,omitempty"`
	ReviewEffort    string           `json:"reviewEffort,omitempty"`
	HighRecall      bool             `json:"highRecall,omitempty"`
	WorktreeRoot    string           `json:"worktreeRoot,omitempty"`
	Model           ModelIdentity    `json:"model,omitempty"`
	Limits          Limits           `json:"limits,omitempty"`
	Changes         []diff.Change    `json:"changes,omitempty"`
	Coverage        []CoverageEntry  `json:"coverage,omitempty"`
}

type RuleSource struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type RuleDiagnostic struct {
	Path    string   `json:"path"`
	Sources []string `json:"sources"`
	Text    string   `json:"text"`
}

type ModelIdentity struct {
	Provider string `json:"provider,omitempty"`
	Name     string `json:"name,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
}

type Limits struct {
	PromptTokens   int `json:"promptTokens,omitempty"`
	OutputTokens   int `json:"outputTokens,omitempty"`
	TotalTokens    int `json:"totalTokens,omitempty"`
	RequestSeconds int `json:"requestSeconds,omitempty"`
	GroupSeconds   int `json:"groupSeconds,omitempty"`
}

type Usage struct {
	PromptTokens int  `json:"promptTokens"`
	OutputTokens int  `json:"outputTokens"`
	TotalTokens  int  `json:"totalTokens"`
	ModelCalls   int  `json:"modelCalls"`
	Estimated    bool `json:"estimated"`
}

type Finding struct {
	ID                 string     `json:"id"`
	Category           string     `json:"category"`
	Severity           string     `json:"severity"`
	File               string     `json:"file"`
	StartLine          int        `json:"startLine"`
	EndLine            int        `json:"endLine"`
	Title              string     `json:"title"`
	Explanation        string     `json:"explanation"`
	SuggestedFix       string     `json:"suggestedFix,omitempty"`
	CitedSnippet       string     `json:"citedSnippet,omitempty"`
	Verification       string     `json:"verification,omitempty"`
	AnchorStatus       string     `json:"anchorStatus,omitempty"`
	VerificationReason string     `json:"verificationReason,omitempty"`
	Source             string     `json:"source"`
	LinterName         string     `json:"linterName,omitempty"`
	Citations          []Citation `json:"citations"`
}

type Citation struct {
	File      string `json:"file"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

type ReviewResponse struct {
	Findings      []json.RawMessage `json:"findings"`
	Summary       json.RawMessage   `json:"summary"`
	Trace         json.RawMessage   `json:"trace"`
	Partial       bool              `json:"-"`
	ReviewedFiles []string          `json:"-"`
}

type AuditEvent struct {
	Stage   string `json:"stage"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Result struct {
	ContractVersion int               `json:"contractVersion"`
	Status          string            `json:"status"`
	Request         Request           `json:"request"`
	Coverage        []CoverageEntry   `json:"coverage"`
	Findings        []json.RawMessage `json:"findings"`
	Summary         json.RawMessage   `json:"summary,omitempty"`
	Trace           json.RawMessage   `json:"trace,omitempty"`
	Audit           []AuditEvent      `json:"audit,omitempty"`
	Usage           Usage             `json:"usage"`
	Error           string            `json:"error,omitempty"`
}

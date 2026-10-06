package review

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
)

func TestRunThroughBridge(t *testing.T) {
	bridge := filepath.Join(t.TempDir(), "bridge.mjs")
	source := `import { readFileSync } from 'node:fs';
const request = JSON.parse(readFileSync(0, 'utf8'));
if (request.contractVersion !== 2 || request.files[0] !== 'main.go' || request.changes || request.coverage) process.exit(2);
process.stdout.write(JSON.stringify({findings: [], summary: {totalFindings: 0}, trace: {}}));`
	if err := os.WriteFile(bridge, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPEN_REVIEW_BRIDGE", bridge)
	result, err := Run(context.Background(), Request{
		ContractVersion: 2,
		Mode:            "workspace",
		RepoRoot:        t.TempDir(),
		Files:           []string{"main.go"},
		Diff:            "diff --git a/main.go b/main.go\n",
		Changes:         []diff.Change{{Path: "main.go", Patch: "full original patch"}},
		Coverage:        []CoverageEntry{{Path: "main.go", State: "selected"}},
	})
	if err != nil || result.Status != "complete" || len(result.Findings) != 0 || result.Coverage[0].State != "reviewed" || result.Request.Changes[0].Patch != "full original patch" {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

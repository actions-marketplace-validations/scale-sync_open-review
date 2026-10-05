package review

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunThroughBridge(t *testing.T) {
	bridge := filepath.Join(t.TempDir(), "bridge.mjs")
	source := `import { readFileSync } from 'node:fs';
const request = JSON.parse(readFileSync(0, 'utf8'));
if (request.contractVersion !== 1 || request.files[0] !== 'main.go') process.exit(2);
process.stdout.write(JSON.stringify({findings: [], summary: {totalFindings: 0}, trace: {}}));`
	if err := os.WriteFile(bridge, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPEN_REVIEW_BRIDGE", bridge)
	result, err := Run(context.Background(), Request{
		ContractVersion: 1,
		Mode:            "workspace",
		RepoRoot:        t.TempDir(),
		Files:           []string{"main.go"},
		Diff:            "diff --git a/main.go b/main.go\n",
	})
	if err != nil || result.Status != "complete" || len(result.Findings) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

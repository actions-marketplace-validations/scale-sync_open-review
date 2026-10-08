package linter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

func TestLinterMergesChangedLineAndSkipsMissingTools(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.ts"), []byte("bad\nokay\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	response := `[ {"filePath":"main.ts","messages":[{"line":1,"ruleId":"no-undef","message":"bad name","severity":2},{"line":2,"ruleId":"no-undef","message":"unchanged","severity":2}]} ]`
	if err := os.WriteFile(filepath.Join(bin, "eslint"), []byte("#!/bin/sh\nprintf '%s' '"+response+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("LINTER_GITLEAKS", "false")
	patch := "diff --git a/main.ts b/main.ts\n@@ -1,1 +1,2 @@\n+bad\n okay\n"
	aiFinding := model.Finding{ID: "ai-1", File: "main.ts", StartLine: 1, EndLine: 1, Source: "ai", Title: "Bad name", Severity: "non-severe"}
	raw, _ := json.Marshal(aiFinding)
	req := model.Request{WorktreeRoot: root, Files: []string{"main.ts"}, Changes: []diff.Change{diff.FromPatch(diff.Change{Path: "main.ts", Patch: patch})}}
	merged, audit := Run(context.Background(), req, []json.RawMessage{raw})
	if len(merged) != 1 || len(audit) == 0 {
		t.Fatalf("merged = %s, audit = %+v", merged, audit)
	}
	var finding model.Finding
	if err := json.Unmarshal(merged[0], &finding); err != nil || finding.Source != "both" || finding.LinterName != "ESLint" {
		t.Fatalf("finding = %+v, %v", finding, err)
	}
}

func TestGitleaksNeverReturnsSecretValue(t *testing.T) {
	raw := `[ {"File":"main.go","StartLine":1,"RuleID":"key","Description":"Token","Match":"private-test-key"} ]`
	findings := parse("Gitleaks", []byte(raw))
	if len(findings) != 1 || strings.Contains(findings[0].Explanation, "private-test-key") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestGitleaksReadsRedactedReport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
[ "$1" = dir ] || exit 2
report=""
redact=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --report-path) report="$2"; shift 2 ;;
    --redact=100) redact=yes; shift ;;
    *) shift ;;
  esac
done
[ "$redact" = yes ] || exit 3
printf '%s' '[{"File":"main.go","StartLine":1,"RuleID":"key","Match":"private-test-key"}]' > "$report"
exit 1
`
	if err := os.WriteFile(filepath.Join(bin, "gitleaks"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	patch := "diff --git a/main.go b/main.go\n@@ -0,0 +1 @@\n+token\n"
	req := model.Request{WorktreeRoot: root, Files: []string{"main.go"}, Changes: []diff.Change{diff.FromPatch(diff.Change{Path: "main.go", Patch: patch})}}
	findings, audit := Run(context.Background(), req, nil)
	if len(findings) != 1 || len(audit) != 1 || audit[0].Status != "complete" {
		t.Fatalf("findings = %s, audit = %+v", findings, audit)
	}
	if strings.Contains(string(findings[0]), "private-test-key") {
		t.Fatal("secret leaked into finding")
	}
}

func TestOtherLinterFormats(t *testing.T) {
	tests := []struct{ name, output, wantTitle string }{
		{"Ruff", `[{"filename":"main.py","code":"F401","message":"unused import","location":{"row":2}}]`, "Ruff: F401"},
		{"Semgrep", `{"results":[{"path":"main.go","check_id":"unsafe","start":{"line":3},"extra":{"message":"unsafe call","severity":"ERROR"}}]}`, "Semgrep: unsafe"},
		{"ShellCheck", `[{"file":"run.sh","line":4,"code":2086,"message":"quote this","level":"warning"}]`, "ShellCheck: SC2086"},
	}
	for _, tc := range tests {
		findings := parse(tc.name, []byte(tc.output))
		if len(findings) != 1 || findings[0].Title != tc.wantTitle {
			t.Fatalf("%s findings = %+v", tc.name, findings)
		}
	}
}

func TestActionSkipsUntrustedCheckout(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("OPEN_REVIEW_TRUSTED_LINTERS", "")
	findings, audit := Run(context.Background(), model.Request{WorktreeRoot: t.TempDir(), Files: []string{"main.go"}}, nil)
	if len(findings) != 0 || len(audit) != 1 || audit[0].Status != "disabled" {
		t.Fatalf("findings = %s, audit = %+v", findings, audit)
	}
}

func TestMissingLinterIsAudited(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("LINTER_GITLEAKS", "false")
	patch := "diff --git a/main.py b/main.py\n@@ -0,0 +1 @@\n+bad\n"
	_, audit := Run(context.Background(), model.Request{WorktreeRoot: root, Files: []string{"main.py"}, Changes: []diff.Change{diff.FromPatch(diff.Change{Path: "main.py", Patch: patch})}}, nil)
	if len(audit) == 0 || audit[0].Stage != "Ruff" || audit[0].Status != "unavailable" {
		t.Fatalf("audit = %+v", audit)
	}
}

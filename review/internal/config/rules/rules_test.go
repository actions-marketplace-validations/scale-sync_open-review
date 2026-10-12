package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestPrecedenceScopeAndReplace(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(os.Getenv("OPEN_REVIEW_HOME"), "rules.yaml"), []byte("rules:\n  - path: '**/*.go'\n    instructions: user rule\n"), 0600); err != nil {
		t.Fatal(err)
	}
	req := model.Request{WorktreeRoot: t.TempDir(), Files: []string{"api/main.go", "web/app.tsx", "other.bin"}, Instructions: "explicit rule", RuleSources: []model.RuleSource{
		{Path: "REVIEW.md", Content: "repository root"},
		{Path: "api/AGENTS.md", Content: "api instructions"},
		{Path: ".reviewbuddy.yaml", Content: "path_instructions:\n  - path: 'api/**'\n    instructions: api replacement\n    mode: replace\n"},
	}}
	got, err := Resolve(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || strings.Contains(got[0].Text, "api instructions") || !strings.Contains(got[0].Text, "api replacement") || strings.Contains(got[0].Text, "user rule") || strings.Contains(got[0].Text, "repository root") || !strings.HasSuffix(got[0].Text, "explicit rule") {
		t.Fatalf("api rule = %+v", got[0])
	}
	if strings.Contains(got[1].Text, "api instructions") || !strings.Contains(got[1].Text, "type boundaries") || !strings.Contains(got[1].Text, "repository root") {
		t.Fatalf("web rule = %+v", got[1])
	}
	if strings.Contains(got[2].Text, "type boundaries") || !strings.Contains(got[2].Text, "explicit rule") {
		t.Fatalf("unknown extension = %+v", got[2])
	}
}

func TestInvalidRulesAndSourcePaths(t *testing.T) {
	for _, source := range []model.RuleSource{
		{Path: "../AGENTS.md", Content: "escape"},
		{Path: ".reviewbuddy.yaml", Content: "path_instructions:\n  - path: '[broken'\n    instructions: bad\n"},
		{Path: ".reviewbuddy.yaml", Content: "path_instructions:\n  - path: '**'\n    instructions: bad\n    mode: erase\n"},
		{Path: ".reviewbuddy.yaml", Content: "profile: unknown\n"},
	} {
		if _, err := Resolve(model.Request{Files: []string{"main.go"}, RuleSources: []model.RuleSource{source}}); err == nil {
			t.Fatalf("accepted invalid source %+v", source)
		}
	}
}

func TestLoadLocalScopesAndRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"REVIEW.md": "root", "src/AGENTS.md": "src", "other.md": "ignored"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sources, err := LoadLocal(root, []string{"src/main.go"})
	if err != nil || len(sources) != 2 {
		t.Fatalf("sources = %+v, %v", sources, err)
	}
	if err := os.Symlink(filepath.Join(root, "other.md"), filepath.Join(root, "src", "REVIEW.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLocal(root, []string{"src/main.go"}); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestLoadLocalRejectsParentEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "AGENTS.md"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "src")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLocal(root, []string{"src/main.go"}); err == nil {
		t.Fatal("accepted parent directory symlink escape")
	}
}

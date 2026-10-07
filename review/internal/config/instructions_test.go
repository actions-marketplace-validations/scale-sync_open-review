package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstructionsUseRootAndSelectedScope(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{
		"REVIEW.md":       "root first",
		"AGENTS.md":       "root second",
		"src/AGENTS.md":   "selected",
		"other/AGENTS.md": "unrelated",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got := Instructions(root, []string{"src/main.go"})
	if !strings.Contains(got, "selected") || strings.Contains(got, "unrelated") || strings.Index(got, "root first") > strings.Index(got, "root second") {
		t.Fatalf("instructions = %q", got)
	}
}

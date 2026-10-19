package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDryRunMakesNoExternalCalls(t *testing.T) {
	dataset := filepath.Join(t.TempDir(), "dataset.json")
	data := `[{
"project_main_language":"Go","pr_url":"https://github.com/a/b/pull/1",
"pr_source_commit":"base","pr_target_commit":"head","note":"a real issue",
"path":"main.go","label":1}]`
	if err := os.WriteFile(dataset, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"run", "--dataset", dataset}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "No repositories fetched or model calls made") {
		t.Fatal(output.String())
	}
}

func TestRepoForPRRejectsUnexpectedHostAndPath(t *testing.T) {
	for _, value := range []string{"https://evil.example/a/b/pull/1", "https://github.com/a/b", "https://github.com/a/../pull/1"} {
		if _, _, err := repoForPR(value); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	url, name, err := repoForPR("https://github.com/a/b/pull/1")
	if err != nil || url != "https://github.com/a/b.git" || name != "a__b" {
		t.Fatalf("%s %s %v", url, name, err)
	}
}

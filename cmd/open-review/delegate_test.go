package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDelegateNeedsNoKeyAndExcludesUnselectedContent(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for name, data := range map[string]string{"review.go": "package review\n// selected change\n", "secret.txt": "SECRET_CONTENT_DO_NOT_SEND"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OPENAI_API_KEY", "")
	var output bytes.Buffer
	if err := run(context.Background(), []string{"delegate", "--repo", root, "--include", "*.go"}, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "SECRET_CONTENT_DO_NOT_SEND") {
		t.Fatal("delegation exposed excluded file content")
	}
	var task struct {
		Version int `json:"version"`
		Request struct {
			Files    []string                       `json:"files"`
			Diff     string                         `json:"diff"`
			Coverage []struct{ Path, State string } `json:"coverage"`
		} `json:"request"`
	}
	if err := json.Unmarshal(output.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Version != 1 || len(task.Request.Files) != 1 || task.Request.Files[0] != "review.go" || !strings.Contains(task.Request.Diff, "selected change") || len(task.Request.Coverage) != 2 {
		t.Fatalf("bad task: %+v", task)
	}
}

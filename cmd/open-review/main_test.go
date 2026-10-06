package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEmptyReviewJSONNeedsNoBridge(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	t.Setenv("OPEN_REVIEW_BRIDGE", filepath.Join(repo, "missing-bridge.mjs"))
	var output bytes.Buffer
	if err := run(context.Background(), []string{"review", "--repo", repo, "--output", "json"}, &output); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Status != "empty" {
		t.Fatalf("result = %s, %v", output.String(), err)
	}
}

func TestRejectsMixedModes(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"review", "--workspace", "--commit", "HEAD"}, &output)
	if err == nil {
		t.Fatal("expected mode error")
	}
}

func TestPreviewShowsSelectedAndSkippedWithoutBridge(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for name, content := range map[string]string{"main.go": "package main\n", "pnpm-lock.yaml": "lock\n"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OPEN_REVIEW_BRIDGE", filepath.Join(repo, "missing-bridge.mjs"))
	var output bytes.Buffer
	err := run(context.Background(), []string{"review", "--repo", repo, "--preview", "--output", "json"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Coverage []struct {
			Path   string `json:"path"`
			State  string `json:"state"`
			Reason string `json:"reason"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil || len(preview.Coverage) != 2 {
		t.Fatalf("preview = %s, %v", output.String(), err)
	}
	if preview.Coverage[0].Path != "main.go" || preview.Coverage[0].State != "selected" || preview.Coverage[1].Reason != "generated_or_lockfile" {
		t.Fatalf("coverage = %+v", preview.Coverage)
	}
}

func TestRejectsOutsideRepository(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"review", "--repo", os.TempDir()}, &output)
	if err == nil {
		t.Fatal("expected repository error")
	}
}

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

func TestRejectsOutsideRepository(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"review", "--repo", os.TempDir()}, &output)
	if err == nil {
		t.Fatal("expected repository error")
	}
}

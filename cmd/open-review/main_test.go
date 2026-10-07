package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestExternalReviewAppliesGoSelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Messages) != 1 || strings.Contains(body.Messages[0].Content, "+second") {
			t.Errorf("wrong model input: %+v, %v", body, err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	request := `{"contractVersion":2,"mode":"branch","repoRoot":"org/repo","diff":"","files":[],"changes":[{"path":"first.go","status":"modified","binary":false,"patch":"diff --git a/first.go b/first.go\n@@ -0,0 +1 @@\n+first\n","hunks":[]},{"path":"second.go","status":"modified","binary":false,"patch":"diff --git a/second.go b/second.go\n@@ -0,0 +1 @@\n+second\n","hunks":[]},{"path":"logo.png","status":"added","binary":false,"patch":"","hunks":[]}]}`
	var output bytes.Buffer
	err := runJSON(context.Background(), []string{"--max-files", "1"}, strings.NewReader(request), &output)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status  string `json:"status"`
		Request struct {
			Diff  string   `json:"diff"`
			Files []string `json:"files"`
		} `json:"request"`
		Coverage []struct{ Path, State, Reason string } `json:"coverage"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || len(result.Coverage) != 3 {
		t.Fatalf("result = %+v", result)
	}
	if result.Coverage[0].State != "reviewed" || result.Coverage[1].Reason != "missing_patch" || result.Coverage[2].Reason != "file_limit" || len(result.Request.Files) != 1 || strings.Contains(result.Request.Diff, "+second") {
		t.Fatalf("coverage = %+v", result.Coverage)
	}
}

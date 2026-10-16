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

func TestRejectsUnknownReviewEffort(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"review", "--effort", "unbounded"}, &output)
	if err == nil || !strings.Contains(err.Error(), "effort") {
		t.Fatalf("error = %v", err)
	}
}

func TestMCPRejectsSessionResumeAndLowEffort(t *testing.T) {
	for _, args := range [][]string{{"review", "--mcp-config", "tools.json", "--session-dir", "sessions"}, {"review", "--mcp-config", "tools.json", "--effort", "low"}} {
		var output bytes.Buffer
		if err := run(context.Background(), args, &output); err == nil {
			t.Fatalf("accepted incompatible flags: %v", args)
		}
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

func TestRulesCheckMatchesReviewPreview(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.Mkdir(filepath.Join(repo, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"src/main.go": "package main\n", "src/AGENTS.md": "Check every error."} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var checked, preview bytes.Buffer
	if err := run(context.Background(), []string{"rules", "check", "--repo", repo, "--file", "src/main.go", "--output", "json"}, &checked); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"review", "--repo", repo, "--preview", "--output", "json"}, &preview); err != nil {
		t.Fatal(err)
	}
	var rule struct {
		Text string `json:"text"`
	}
	var result struct {
		Rules []struct{ Path, Text string } `json:"rules"`
	}
	if json.Unmarshal(checked.Bytes(), &rule) != nil || json.Unmarshal(preview.Bytes(), &result) != nil {
		t.Fatalf("check=%s preview=%s", checked.String(), preview.String())
	}
	found := false
	for _, item := range result.Rules {
		if item.Path == "src/main.go" {
			found = item.Text == rule.Text && strings.Contains(item.Text, "Check every error.")
		}
	}
	if !found {
		t.Fatalf("check=%s preview=%s", checked.String(), preview.String())
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

func TestProviderSetupAndConnectivityHideKey(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-test-key" {
			t.Errorf("missing authorization")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := runConfigInput(context.Background(), []string{"set", "--provider", "openai", "--model", "custom-model", "--base-url", server.URL, "--api-key-stdin"}, strings.NewReader("private-test-key\n"), &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := runConfigInput(context.Background(), []string{"show"}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "private-test-key") || !strings.Contains(output.String(), "Saved key: true") {
		t.Fatalf("show leaked or hid state: %s", output.String())
	}
	output.Reset()
	if err := runConfigInput(context.Background(), []string{"test"}, strings.NewReader(""), &output); err != nil || !strings.Contains(output.String(), "Connected to openai") {
		t.Fatalf("connectivity: %s, %v", output.String(), err)
	}
	output.Reset()
	if err := runConfigInput(context.Background(), []string{"setup"}, strings.NewReader("openai\ncustom-model\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Provider (") || strings.Contains(output.String(), "private-test-key") {
		t.Fatalf("setup output: %s", output.String())
	}
}

func TestOutputFileUntouchedOnInterruptedModelResponse(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 999\r\n\r\n{"))
		_ = conn.Close()
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("previous report"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := run(context.Background(), []string{"review", "--repo", repo, "--output", "json", "--output-file", path, "--quiet"}, &output)
	contents, readErr := os.ReadFile(path)
	if err == nil || readErr != nil || string(contents) != "previous report" || output.Len() != 0 {
		t.Fatalf("err = %v, file = %q, stdout = %q", err, contents, output.String())
	}
}

func TestOutputFileIsAtomicAndPrivate(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	var stdout bytes.Buffer
	if err := run(context.Background(), []string{"review", "--repo", repo, "--output", "json", "--output-file", path, "--quiet"}, &stdout); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 || stdout.Len() != 0 || !strings.Contains(string(contents), `"status": "empty"`) {
		t.Fatalf("output = %q, mode = %v, stdout = %q, err = %v", contents, info.Mode().Perm(), stdout.String(), err)
	}
}

func TestCompletionAndPartialJSONStatus(t *testing.T) {
	var completion bytes.Buffer
	if err := run(context.Background(), []string{"completion", "bash"}, &completion); err != nil || !strings.Contains(completion.String(), "--total-tokens") {
		t.Fatalf("completion = %s, %v", completion.String(), err)
	}
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	request := `{"contractVersion":2,"mode":"branch","repoRoot":"org/repo","files":["main.go"],"diff":"diff --git a/main.go b/main.go\n@@ -0,0 +1 @@\n+bad\n","changes":[{"path":"main.go","status":"modified","binary":false,"patch":"diff --git a/main.go b/main.go\n@@ -0,0 +1 @@\n+bad\n","hunks":[]}],"limits":{"totalTokens":1}}`
	var output bytes.Buffer
	if err := runJSON(context.Background(), nil, strings.NewReader(request), &output); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status string `json:"status"`
		Usage  struct {
			TotalTokens int `json:"totalTokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Status != "partial" || result.Usage.TotalTokens != 0 {
		t.Fatalf("result = %s, %v", output.String(), err)
	}
}

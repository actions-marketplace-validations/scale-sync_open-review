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
	"sync/atomic"
	"testing"

	"github.com/scale-sync/open-review/review"
)

func TestLocalReviewResumesThroughCLI(t *testing.T) {
	repo := t.TempDir()
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "Test")
	file := filepath.Join(repo, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "main.go")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "main.go")
	git("commit", "-qm", "change")
	head := git("rev-parse", "HEAD")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	for attempt := 0; attempt < 2; attempt++ {
		var output bytes.Buffer
		err := run(context.Background(), []string{"review", "--repo", repo, "--base", base, "--head", head, "--session-dir", dir, "--output", "json", "--quiet"}, &output)
		if err != nil || !strings.Contains(output.String(), `"status": "complete"`) {
			t.Fatalf("attempt %d: %v, %s", attempt, err, output.String())
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("model calls = %d", calls.Load())
	}
	items, err := review.ListSessions(dir, repo)
	if err != nil || len(items) != 2 {
		t.Fatalf("sessions = %+v, %v", items, err)
	}
}

func TestSessionCommandsRespectRepositoryAndShowComparison(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	repo := t.TempDir()
	dir := t.TempDir()
	req := review.Request{ContractVersion: 2, Mode: "branch", RepoRoot: repo, HeadSHA: strings.Repeat("a", 40), Model: review.ModelIdentity{Provider: "openai", Name: "test-model"}}
	first, err := review.OpenSession(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Finish(review.Result{Status: "complete", Findings: []json.RawMessage{json.RawMessage(`{"id":"old"}`)}}); err != nil {
		t.Fatal(err)
	}
	second, err := review.OpenSession(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Finish(review.Result{Status: "partial", Findings: []json.RawMessage{json.RawMessage(`{"id":"new"}`)}}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	call := func(args ...string) string {
		t.Helper()
		output.Reset()
		if err := runSessions(args, &output); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	if got := call("list", "--dir", dir, "--repo", repo); !strings.Contains(got, first.ID()) || !strings.Contains(got, second.ID()) {
		t.Fatalf("list = %s", got)
	}
	if got := call("inspect", "--dir", dir, "--repo", repo, first.ID()); !strings.Contains(got, `"type": "review"`) {
		t.Fatalf("inspect = %s", got)
	}
	if got := call("show", "--dir", dir, "--repo", repo, first.ID()); !strings.Contains(got, `"id": "old"`) {
		t.Fatalf("show = %s", got)
	}
	if got := call("compare", "--dir", dir, "--repo", repo, first.ID(), second.ID()); !strings.Contains(got, `"new"`) || !strings.Contains(got, `"old"`) {
		t.Fatalf("compare = %s", got)
	}
	path := filepath.Join(t.TempDir(), "export.json")
	call("export", "--dir", dir, "--repo", repo, "--output", path, first.ID())
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), first.ID()) {
		t.Fatalf("export = %q, %v", data, err)
	}
	foreign := t.TempDir()
	if err := runSessions([]string{"delete", "--dir", dir, "--repo", foreign, first.ID()}, &output); err == nil {
		t.Fatal("foreign repo could delete session")
	}
	call("delete", "--dir", dir, "--repo", repo, first.ID())
	if _, err := os.Stat(filepath.Join(dir, first.ID())); !os.IsNotExist(err) {
		t.Fatalf("session remains: %v", err)
	}
}

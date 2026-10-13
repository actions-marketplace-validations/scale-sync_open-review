package review

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

func TestGoReviewPreservesFullChanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected model request: %s, %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) != 1 || request.Messages[0].Content == "" {
			t.Errorf("invalid model prompt: %+v, %v", request, err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[{\"category\":\"bug\",\"severity\":\"non-severe\",\"file\":\"main.go\",\"startLine\":1,\"endLine\":1,\"title\":\"Bad value\",\"explanation\":\"The value is wrong\"}]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL+"/v1")
	t.Setenv("MAIN_MODEL", "gpt-4o")
	patch := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -0,0 +1 @@\n+bad\n"
	result, err := Run(context.Background(), Request{
		ContractVersion: 2, Mode: "workspace", RepoRoot: t.TempDir(),
		Files: []string{"main.go"}, Diff: patch,
		Changes:  []diff.Change{diff.Untracked("main.go", patch)},
		Coverage: []CoverageEntry{{Path: "main.go", State: "selected"}},
	})
	if err != nil || result.Status != "complete" || len(result.Findings) != 1 || result.Coverage[0].State != "reviewed" || result.Request.Changes[0].Patch != patch {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestExternalRequestCannotGrantLocalFileReads(t *testing.T) {
	req, err := PrepareExternal(Request{ContractVersion: 2, Mode: "branch", RepoRoot: "org/repo", WorktreeRoot: t.TempDir()}, 10)
	if err != nil || req.WorktreeRoot != "" {
		t.Fatalf("external worktree = %q, err = %v", req.WorktreeRoot, err)
	}
}

func TestExternalReviewUsesOnlyMatchingRuleSources(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Messages) > 0 {
			prompt = body.Messages[0].Content
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("MAIN_MODEL", "gpt-4o")
	patch := "diff --git a/src/main.go b/src/main.go\n@@ -0,0 +1 @@\n+bad\n"
	req, err := PrepareExternal(Request{ContractVersion: 2, Mode: "branch", RepoRoot: "org/repo", RuleSources: []model.RuleSource{{Path: "src/AGENTS.md", Content: "Check source errors"}, {Path: "other/AGENTS.md", Content: "Unrelated secret guidance"}}, Changes: []diff.Change{{Path: "src/main.go", Status: "modified", Patch: patch}}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), req)
	if err != nil || result.Status != "complete" || !strings.Contains(prompt, "Check source errors") || strings.Contains(prompt, "Unrelated secret guidance") {
		t.Fatalf("result=%+v prompt=%q err=%v", result, prompt, err)
	}
}

func TestDeepReviewUsesSandboxAuditAndGoValidation(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "false")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "deno"), []byte("#!/bin/sh\nprintf '%s' 'verified condition'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	finalPrompt := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		answer := `{"code":"console.log('verified condition')"}`
		if len(body.Messages) > 0 && strings.Contains(body.Messages[0].Content, "Prior observations:\n\nCheck 1:") {
			answer = `{"code":"","done":true}`
		}
		if len(body.Messages) > 0 && !strings.Contains(body.Messages[0].Content, "Investigate the diff") {
			finalPrompt = body.Messages[0].Content
			answer = `{"findings":[]}`
		}
		encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("MAIN_MODEL", "gpt-4o")
	patch := "diff --git a/main.go b/main.go\n@@ -0,0 +1 @@\n+bad\n"
	result, err := Run(context.Background(), Request{ContractVersion: 2, ReviewKind: "deep", Mode: "branch", RepoRoot: "org/repo", Files: []string{"main.go"}, Diff: patch, Changes: []diff.Change{diff.FromPatch(diff.Change{Path: "main.go", Patch: patch})}})
	if err != nil || result.Status != "complete" || len(result.Audit) != 2 || result.Audit[0].Status != "complete" || result.Audit[1].Stage != "finding_verification" || !strings.Contains(finalPrompt, "verified condition") || !strings.Contains(string(result.Summary), `"mode":"rlm"`) {
		t.Fatalf("result = %+v, prompt = %q, err = %v", result, finalPrompt, err)
	}
}

func TestDeepReviewWithoutDenoRecordsUnverifiedPass(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "false")
	t.Setenv("PATH", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("MAIN_MODEL", "gpt-4o")
	result, err := Run(context.Background(), Request{ContractVersion: 2, ReviewKind: "deep", RepoRoot: "org/repo", Files: []string{"main.go"}, Diff: "diff --git a/main.go b/main.go\n+bad\n"})
	if err != nil || result.Status != "complete" || len(result.Audit) != 2 || result.Audit[0].Status != "unavailable" || result.Audit[1].Stage != "finding_verification" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestDeepReviewBudgetReturnsPartial(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "false")
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "deno"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	patch := "diff --git a/main.go b/main.go\n@@ -0,0 +1 @@\n+bad\n"
	result, err := Run(context.Background(), Request{ContractVersion: 2, ReviewKind: "deep", RepoRoot: "org/repo", Files: []string{"main.go"}, Diff: patch, Limits: model.Limits{TotalTokens: 1}})
	if err != nil || result.Status != "partial" || len(result.Audit) == 0 || result.Audit[0].Status != "partial" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

package review

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
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

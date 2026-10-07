package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestModelProvidersAndMalformedOutput(t *testing.T) {
	tests := []struct{ model, key, base, endpoint, response string }{
		{"gpt-4o", "OPENAI_API_KEY", "OPENAI_BASE_URL", "/chat/completions", `{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`},
		{"claude-sonnet", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "/v1/messages", `{"content":[{"text":"{\"findings\":[]}"}]}`},
		{"gemini-flash", "GEMINI_API_KEY", "GEMINI_BASE_URL", "/v1beta/models/gemini-flash:generateContent", `{"candidates":[{"content":{"parts":[{"text":"{\"findings\":[]}"}]}}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.endpoint {
					t.Errorf("path = %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			t.Setenv(tc.key, "test-key")
			t.Setenv(tc.base, server.URL)
			answer, err := call(context.Background(), tc.model, "diff")
			if err != nil || answer != `{"findings":[]}` {
				t.Fatalf("answer = %q, %v", answer, err)
			}
		})
	}
	t.Setenv("OPENAI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not json"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	_, err := Review(context.Background(), model.Request{Diff: "diff --git a/x.go b/x.go\n+bad\n", Files: []string{"x.go"}})
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("error = %v", err)
	}
}

func TestChunkBudget(t *testing.T) {
	_, err := chunk(strings.Repeat("x", maxChunkBytes+1))
	if err == nil {
		t.Fatal("oversized patch should fail")
	}
	parts, err := chunk("diff --git a/a.go b/a.go\n+a\n" + "diff --git a/b.go b/b.go\n+b\n")
	if err != nil || len(parts) != 1 || strings.Count(parts[0], "diff --git ") != 2 {
		t.Fatalf("chunks = %+v, %v", parts, err)
	}
}

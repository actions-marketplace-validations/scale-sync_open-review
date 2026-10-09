package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestReviewStopsAtAggregateBudgetWithUsage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["max_tokens"].(float64) > 1_200 {
			t.Errorf("output cap = %v", request["max_tokens"])
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}],"usage":{"prompt_tokens":6500,"completion_tokens":10}}`))
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	ctx, cancel, usage, err := WithSession(context.Background(), model.Limits{TotalTokens: 7_000, OutputTokens: 1_200})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	patch := "diff --git a/a.go b/a.go\n" + strings.Repeat("a", 23_000) + "\ndiff --git a/b.go b/b.go\n" + strings.Repeat("b", 23_000)
	response, err := Review(ctx, model.Request{Diff: patch, Files: []string{"a.go", "b.go"}})
	if err != nil || !response.Partial || calls != 1 || usage().TotalTokens != 6510 || usage().ModelCalls != 1 {
		t.Fatalf("partial = %t, calls = %d, usage = %+v, err = %v", response.Partial, calls, usage(), err)
	}
}

func TestRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	ctx, cancel, _, err := WithSession(context.Background(), model.Limits{RequestSeconds: 1, GroupSeconds: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	_, err = callModel(ctx, model.ModelIdentity{}, "test")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
}

func TestProviderRedirectDoesNotForwardCredential(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Store(true) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "private-test-key")
	t.Setenv("ANTHROPIC_BASE_URL", server.URL)
	_, err := call(context.Background(), "claude-test", "test")
	if err == nil || redirected.Load() || strings.Contains(err.Error(), "private-test-key") {
		t.Fatalf("redirect = %t, error = %v", redirected.Load(), err)
	}
}

func TestProviderReportedOverrunStopsResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}],"usage":{"prompt_tokens":10,"completion_tokens":10}}`))
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	ctx, cancel, usage, err := WithSession(context.Background(), model.Limits{OutputTokens: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	_, err = callModel(ctx, model.ModelIdentity{}, "test")
	if !errors.Is(err, ErrBudget) || usage().OutputTokens != 10 {
		t.Fatalf("error = %v, usage = %+v", err, usage())
	}
}

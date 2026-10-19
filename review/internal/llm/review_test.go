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

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
	sessionstore "github.com/scale-sync/open-review/review/internal/session"
)

func TestReviewResumesCompletedGroupsWithoutModelCalls(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	req := model.Request{ContractVersion: model.ContractVersion, Mode: "branch", RepoRoot: t.TempDir(), HeadSHA: strings.Repeat("a", 40), Files: []string{"a.go"}, Diff: "diff --git a/a.go b/a.go\n+change\n", Model: model.ModelIdentity{Provider: "openai", Name: "test-model"}}
	dir := t.TempDir()
	first, err := sessionstore.Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := Review(sessionstore.WithStore(context.Background(), first), req)
	if err != nil || calls.Load() != 1 {
		t.Fatalf("first review: %v, calls = %d", err, calls.Load())
	}
	second, err := sessionstore.Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := Review(sessionstore.WithStore(context.Background(), second), req)
	if err != nil || calls.Load() != 1 || len(resumed.Findings) != len(initial.Findings) {
		t.Fatalf("resumed review: %v, calls = %d", err, calls.Load())
	}
	req.Instructions = "Check for different behavior"
	third, err := sessionstore.Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Review(sessionstore.WithStore(context.Background(), third), req)
	if err != nil || calls.Load() != 2 {
		t.Fatalf("stale rules reused: %v, calls = %d", err, calls.Load())
	}
}

func TestReviewResumeReusesPlanAndGroups(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		content := `{"findings":[]}`
		if strings.Contains(request.Messages[0].Content, "one short review focus") {
			content = `{"focus":"check changed behavior"}`
		}
		response, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
		_, _ = w.Write(response)
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	files := []string{"a/one.go", "b/two.go", "c/three.go"}
	patch := ""
	for _, file := range files {
		patch += "diff --git a/" + file + " b/" + file + "\n+change\n"
	}
	req := model.Request{ContractVersion: model.ContractVersion, Mode: "branch", RepoRoot: t.TempDir(), HeadSHA: strings.Repeat("a", 40), Files: files, Diff: patch, Model: model.ModelIdentity{Provider: "openai", Name: "test-model"}}
	dir := t.TempDir()
	for attempt := 0; attempt < 2; attempt++ {
		store, err := sessionstore.Open(dir, req)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel, _, err := WithSession(sessionstore.WithStore(context.Background(), store), model.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		response, err := Review(ctx, req)
		cancel()
		if err != nil || response.Partial {
			t.Fatalf("attempt %d: %v, %+v", attempt, err, response)
		}
	}
	if calls.Load() != 4 {
		t.Fatalf("model calls after resume = %d", calls.Load())
	}
}

func TestModelProvidersAndMalformedOutput(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	tests := []struct{ provider, model, key, base, endpoint, response string }{
		{"openai", "gpt-4o", "OPENAI_API_KEY", "OPENAI_BASE_URL", "/chat/completions", `{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`},
		{"anthropic", "claude-sonnet", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "/v1/messages", `{"content":[{"text":"{\"findings\":[]}"}]}`},
		{"gemini", "gemini-flash", "GEMINI_API_KEY", "GEMINI_BASE_URL", "/v1beta/models/gemini-flash:generateContent", `{"candidates":[{"content":{"parts":[{"text":"{\"findings\":[]}"}]}}]}`},
		{"openrouter", "openrouter/free", "OPENROUTER_API_KEY", "OPENROUTER_BASE_URL", "/chat/completions", `{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`},
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
			answer, err := callModel(context.Background(), model.ModelIdentity{Provider: tc.provider, Name: tc.model}, "diff")
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

func TestReviewPlansLargeChangeAndCollectsParallelGroupsInOrder(t *testing.T) {
	var active, peak, plans atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		content := request.Messages[0].Content
		if strings.Contains(content, "one short review focus") {
			plans.Add(1)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"focus\":\"check callers\"}"}}]}`))
			return
		}
		current := active.Add(1)
		for {
			old := peak.Load()
			if current <= old || peak.CompareAndSwap(old, current) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(40 * time.Millisecond)
		file := "c/three.go"
		if strings.Contains(content, "b/one.go") {
			file = "b/one.go"
		}
		if strings.Contains(content, "a/two.go") {
			file = "a/two.go"
		}
		finding := `{"findings":[{"category":"bug","severity":"severe","file":"` + file + `","startLine":1,"endLine":1,"title":"check","explanation":"impact"}]}`
		response, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": finding}}}})
		_, _ = w.Write(response)
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	files := []string{"b/one.go", "a/two.go", "c/three.go"}
	patch := ""
	for _, file := range files {
		patch += "diff --git a/" + file + " b/" + file + "\n+change\n"
	}
	ctx, cancel, _, err := WithSession(context.Background(), model.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	result, err := Review(ctx, model.Request{Files: files, Diff: patch})
	if err != nil || result.Partial || plans.Load() != 1 || peak.Load() < 2 || peak.Load() > 3 || len(result.Findings) != 3 {
		t.Fatalf("result = %+v, plans = %d, peak = %d, err = %v", result, plans.Load(), peak.Load(), err)
	}
	for i, raw := range result.Findings {
		var finding model.Finding
		_ = json.Unmarshal(raw, &finding)
		if finding.File != files[i] {
			t.Fatalf("finding %d = %s", i, finding.File)
		}
	}
}

func TestReviewIsolatesGroupFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		content := `{"findings":[]}`
		if strings.Contains(request.Messages[0].Content, "b/two.go") {
			content = "bad json"
		}
		response, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
		_, _ = w.Write(response)
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	result, err := Review(context.Background(), model.Request{Files: []string{"a/one.go", "b/two.go"}, Diff: "diff --git a/a/one.go b/a/one.go\n+a\ndiff --git a/b/two.go b/b/two.go\n+b\n"})
	if err != nil || !result.Partial || len(result.ReviewedFiles) != 1 || result.ReviewedFiles[0] != "a/one.go" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestMalformedPlanFallsBackWithoutChangingGroups(t *testing.T) {
	var plans, reviews atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		content := `{"findings":[]}`
		if strings.Contains(request.Messages[0].Content, "one short review focus") {
			plans.Add(1)
			content = "bad plan"
		} else {
			reviews.Add(1)
		}
		response, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
		_, _ = w.Write(response)
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	files := []string{"a/one.go", "b/two.go", "c/three.go"}
	patch := ""
	for _, file := range files {
		patch += "diff --git a/" + file + " b/" + file + "\n+change\n"
	}
	result, err := Review(context.Background(), model.Request{Files: files, Diff: patch})
	var trace struct {
		Plan   string `json:"plan"`
		Groups int    `json:"groups"`
	}
	_ = json.Unmarshal(result.Trace, &trace)
	if err != nil || result.Partial || plans.Load() != 1 || reviews.Load() != 3 || trace.Plan != "fallback" || trace.Groups != 3 {
		t.Fatalf("trace = %+v, plans = %d, reviews = %d, err = %v", trace, plans.Load(), reviews.Load(), err)
	}
}

func TestConcurrentCallsReserveOneSharedBudget(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	ctx, cancel, _, err := WithSession(context.Background(), model.Limits{TotalTokens: 2_048})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := callModel(ctx, model.ModelIdentity{}, "first"); done <- err }()
	<-started
	_, err = callModel(ctx, model.ModelIdentity{}, "second")
	close(release)
	if !errors.Is(err, ErrBudget) || <-done != nil {
		t.Fatalf("second call = %v", err)
	}
}

func TestReviewHonorsCancellation(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Review(ctx, model.Request{Files: []string{"a.go"}, Diff: "diff --git a/a.go b/a.go\n+a\n"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

func TestIterativeReviewReadsDiffRecordsCommentAndCompletes(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		index := calls.Add(1)
		answer := `{"tool":"read_diff","path":"main.go"}`
		if index == 2 {
			if !strings.Contains(request.Messages[0].Content, "+bad") {
				t.Error("selected diff missing from second round")
			}
			answer = `{"tool":"comment","finding":{"category":"bug","severity":"severe","file":"main.go","startLine":1,"endLine":1,"title":"Bad call","explanation":"The call fails"}}`
		}
		if index == 3 {
			answer = `{"tool":"complete"}`
		}
		encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	patch := "diff --git a/main.go b/main.go\n@@ -0,0 +1 @@\n+bad\n"
	response, err := Review(context.Background(), model.Request{ReviewEffort: "medium", Files: []string{"main.go"}, Diff: patch, Changes: []diff.Change{diff.FromPatch(diff.Change{Path: "main.go", Patch: patch})}})
	var trace struct {
		ReviewRounds int `json:"reviewRounds"`
		ToolCalls    int `json:"toolCalls"`
	}
	_ = json.Unmarshal(response.Trace, &trace)
	if err != nil || response.Partial || len(response.Findings) != 1 || calls.Load() != 3 || trace.ReviewRounds != 3 || trace.ToolCalls != 2 {
		t.Fatalf("response = %+v, trace = %+v, calls = %d, err = %v", response, trace, calls.Load(), err)
	}
}

func TestReviewEffortStopsToolsAndLowFinishesEarly(t *testing.T) {
	var calls atomic.Int32
	answer := `{"tool":"read_diff","path":"main.go"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	patch := "diff --git a/main.go b/main.go\n+bad\n"
	req := model.Request{ReviewEffort: "high", Files: []string{"main.go"}, Diff: patch, Changes: []diff.Change{{Path: "main.go", Patch: patch}}}
	_, err := Review(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "tool limit") || calls.Load() != 5 {
		t.Fatalf("high calls = %d, err = %v", calls.Load(), err)
	}
	answer = `{"findings":[]}`
	calls.Store(0)
	req.ReviewEffort = "low"
	response, err := Review(context.Background(), req)
	if err != nil || response.Partial || calls.Load() != 1 {
		t.Fatalf("low calls = %d, err = %v", calls.Load(), err)
	}
}

func TestReviewAcceptsFencedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": "```json\n{\"findings\":[]}\n```"}}}})
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	response, err := Review(context.Background(), model.Request{ReviewEffort: "low", Files: []string{"main.go"}, Diff: "diff --git a/main.go b/main.go\n+safe\n"})
	if err != nil || len(response.Findings) != 0 {
		t.Fatalf("response = %+v, err = %v", response, err)
	}
}

func TestReviewJSONExtractsObjectAfterPreamble(t *testing.T) {
	if got := reviewJSON("Explanation before output.\n{\"findings\":[]}"); got != `{"findings":[]}` {
		t.Fatalf("review JSON = %q", got)
	}
}

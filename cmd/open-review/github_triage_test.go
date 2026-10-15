package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubTriageChecksThreadBeforeMutation(t *testing.T) {
	finding := triageFinding{ID: "F1", File: "src/a.go", StartLine: 9, EndLine: 9, Title: "Fix this bug"}
	mutations := 0
	path := "src/a.go"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing token")
		}
		var input struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(input.Query, "mutation") {
			mutations++
			fmt.Fprint(w, `{"data":{"resolveReviewThread":{"thread":{"id":"thread-1","isResolved":true}}}}`)
			return
		}
		fmt.Fprintf(w, `{"data":{"node":{"id":"thread-1","path":%q,"line":9,"isResolved":false,"viewerCanResolve":true,"pullRequest":{"url":"https://github.com/acme/code/pull/3"},"comments":{"nodes":[{"body":"Fix this bug"}]}}}}`, path)
	}))
	defer server.Close()
	if err := syncGitHubThread(server.Client(), server.URL, "test-token", "https://github.com/acme/code/pull/3", "thread-1", finding, "fixed"); err != nil {
		t.Fatal(err)
	}
	if mutations != 1 {
		t.Fatalf("mutations: %d", mutations)
	}
	path = "src/other.go"
	if err := syncGitHubThread(server.Client(), server.URL, "test-token", "https://github.com/acme/code/pull/3", "thread-1", finding, "ignored"); err == nil {
		t.Fatal("mismatched thread was accepted")
	}
	if mutations != 1 {
		t.Fatal("mismatched thread was changed")
	}
	if err := syncGitHubThread(server.Client(), server.URL, "test-token", "http://other.test/pull/3", "thread-1", finding, "fixed"); err == nil {
		t.Fatal("unsafe URL was accepted")
	}
}

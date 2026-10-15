package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestScanPreviewAndResumeWithoutGit(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var preview bytes.Buffer
	if err := run(context.Background(), []string{"review", "--repo", repo, "--scan", ".", "--preview", "--output", "json"}, &preview); err != nil || !strings.Contains(preview.String(), `"mode":"scan"`) || !strings.Contains(preview.String(), `"state":"selected"`) {
		t.Fatalf("preview = %s, %v", preview.String(), err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	dir := t.TempDir()
	for attempt := 0; attempt < 2; attempt++ {
		var output bytes.Buffer
		err := run(context.Background(), []string{"review", "--repo", repo, "--scan", ".", "--session-dir", dir, "--output", "json", "--quiet"}, &output)
		if err != nil || !strings.Contains(output.String(), `"status": "complete"`) {
			t.Fatalf("attempt %d: %v, %s", attempt, err, output.String())
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("model calls = %d", calls.Load())
	}
}

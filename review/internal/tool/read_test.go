package tool

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("safe.go", "package safe\nfunc Original() {}\n")
	write("large.go", strings.Repeat("x", maxFileBytes+1))
	write("binary.go", "a\x00b")
	write(".env", "SECRET=private")
	if err := os.Symlink("safe.go", filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-qm", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	sha, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	write("safe.go", "package changed\n")
	return root, strings.TrimSpace(string(sha))
}

func TestReadsPinnedRegularFilesAndSelectedDiffOnly(t *testing.T) {
	root, sha := fixture(t)
	req := model.Request{Mode: "workspace", WorktreeRoot: root, BaseSHA: sha, Files: []string{"safe.go"}, Changes: []diff.Change{{Path: "safe.go", Patch: "diff --git a/safe.go b/safe.go\n+new\n"}}}
	reader := New(req)
	if !reader.Available() {
		t.Fatal("expected pinned repository")
	}
	content, err := reader.Run(context.Background(), Request{Name: "read_file", Path: "safe.go"})
	if err != nil || !strings.Contains(content, "Original") || strings.Contains(content, "changed") {
		t.Fatalf("pinned content = %q, %v", content, err)
	}
	patch, err := reader.Run(context.Background(), Request{Name: "read_diff", Path: "safe.go"})
	if err != nil || !strings.Contains(patch, "+new") {
		t.Fatalf("diff = %q, %v", patch, err)
	}
	files, err := reader.Run(context.Background(), Request{Name: "list_files"})
	if err != nil || !strings.Contains(files, "safe.go") || strings.Contains(files, ".env") {
		t.Fatalf("files = %q, %v", files, err)
	}
	matches, err := reader.Run(context.Background(), Request{Name: "search", Query: "Original"})
	if err != nil || !strings.Contains(matches, "safe.go:2:") {
		t.Fatalf("matches = %q, %v", matches, err)
	}
}

func TestRejectsUnsafeBinarySymlinkAndLargeReads(t *testing.T) {
	root, sha := fixture(t)
	reader := New(model.Request{WorktreeRoot: root, HeadSHA: sha, Files: []string{"safe.go"}})
	for _, name := range []string{"../outside", "/etc/passwd", "a/../../outside", "link.go", "binary.go", "large.go", ".env", "missing.go"} {
		if _, err := reader.Run(context.Background(), Request{Name: "read_file", Path: name}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err := reader.Run(context.Background(), Request{Name: "read_diff", Path: "missing.go"}); err == nil {
		t.Fatal("accepted unselected diff")
	}
	if _, err := reader.Run(context.Background(), Request{Name: "search", Query: strings.Repeat("x", 101)}); err == nil {
		t.Fatal("accepted long query")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Run(ctx, Request{Name: "list_files"}); err != context.Canceled {
		t.Fatalf("cancellation = %v", err)
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestRemoteReadsUsePinnedGitHubBlobs(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "private-test-key")
	sha := strings.Repeat("a", 40)
	blob := strings.Repeat("b", 40)
	reader := New(model.Request{Mode: "branch", RepoRoot: "owner/repo", HeadSHA: sha})
	if !reader.Available() {
		t.Fatal("remote context unavailable")
	}
	reader.client.Transport = transportFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer private-test-key" || request.URL.Host != "api.github.com" {
			t.Errorf("unexpected remote request")
		}
		body := `{"tree":[{"path":"src/main.go","mode":"100644","type":"blob","sha":"` + blob + `","size":10},{"path":"secret.key","mode":"100644","type":"blob","sha":"` + blob + `","size":10},{"path":"link.go","mode":"120000","type":"blob","sha":"` + blob + `","size":10}]}`
		if strings.Contains(request.URL.Path, "/git/blobs/") {
			body = `{"content":"cGFja2FnZSBnbw==","encoding":"base64","size":10}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	content, err := reader.Run(context.Background(), Request{Name: "read_file", Path: "src/main.go"})
	if err != nil || content != "package go" {
		t.Fatalf("remote content = %q, %v", content, err)
	}
	if _, err := reader.Run(context.Background(), Request{Name: "read_file", Path: "secret.key"}); err == nil {
		t.Fatal("sensitive file was readable")
	}
	if _, err := reader.Run(context.Background(), Request{Name: "read_file", Path: "link.go"}); err == nil {
		t.Fatal("symlink was readable")
	}
	files, err := reader.Run(context.Background(), Request{Name: "list_files"})
	if err != nil || files != "src/main.go" {
		t.Fatalf("remote files = %q, %v", files, err)
	}
}

func TestRemoteRejectsTruncatedAndOversizedResponses(t *testing.T) {
	for _, body := range []string{`{"truncated":true,"tree":[]}`, strings.Repeat("x", maxListBytes+1)} {
		reader := &Reader{repository: "owner/repo", revision: strings.Repeat("a", 40), token: "private-test-key"}
		reader.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
		if _, err := reader.Run(context.Background(), Request{Name: "list_files"}); err == nil || strings.Contains(err.Error(), "private-test-key") {
			t.Fatalf("response error = %v", err)
		}
	}
}

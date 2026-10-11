package tool

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/scale-sync/open-review/review/internal/model"
)

const maxFileBytes = 8_000
const maxListBytes = 1 << 20

var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
var repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type Request struct {
	Name  string `json:"tool"`
	Path  string `json:"path,omitempty"`
	Query string `json:"query,omitempty"`
}

type Reader struct {
	root        string
	revision    string
	diffs       map[string]string
	repository  string
	token       string
	client      *http.Client
	mu          sync.Mutex
	remoteFiles []string
	remoteBlobs map[string]string
}

func New(req model.Request) *Reader {
	reader := &Reader{diffs: map[string]string{}}
	for _, change := range req.Changes {
		for _, file := range req.Files {
			if file == change.Path {
				reader.diffs[file] = change.Patch
				break
			}
		}
	}
	revision := req.HeadSHA
	if revision == "" {
		revision = req.BaseSHA
	}
	if !revisionPattern.MatchString(revision) {
		return reader
	}
	if req.WorktreeRoot == "" {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_PAT")
		}
		if req.Mode == "branch" && repositoryPattern.MatchString(req.RepoRoot) && token != "" {
			reader.repository, reader.revision, reader.token = req.RepoRoot, revision, token
			reader.client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		}
		return reader
	}
	root, err := filepath.EvalSymlinks(req.WorktreeRoot)
	if err != nil {
		return reader
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return reader
	}
	reader.root, reader.revision = root, revision
	return reader
}

func (r *Reader) Available() bool { return r.root != "" || r.repository != "" }

func (r *Reader) Run(ctx context.Context, call Request) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	switch call.Name {
	case "read_diff":
		if !safePath(call.Path) {
			return "", errors.New("invalid tool path")
		}
		patch, ok := r.diffs[call.Path]
		if !ok {
			return "", errors.New("diff is outside selected files")
		}
		return bounded(patch), nil
	case "read_file":
		if !r.Available() {
			return "", errors.New("repository reads unavailable")
		}
		return r.readFile(ctx, call.Path)
	case "list_files":
		if !r.Available() {
			return "", errors.New("repository reads unavailable")
		}
		files, err := r.list(ctx)
		if err != nil {
			return "", err
		}
		if call.Path != "" && !safePath(call.Path) {
			return "", errors.New("invalid tool path")
		}
		out := make([]string, 0, 100)
		for _, file := range files {
			if !safePath(file) {
				continue
			}
			if call.Path == "" || file == call.Path || strings.HasPrefix(file, strings.TrimSuffix(call.Path, "/")+"/") {
				out = append(out, file)
				if len(out) == 100 {
					break
				}
			}
		}
		return strings.Join(out, "\n"), nil
	case "search":
		if !r.Available() {
			return "", errors.New("repository reads unavailable")
		}
		if call.Query == "" || len(call.Query) > 100 || strings.ContainsAny(call.Query, "\x00\r\n") {
			return "", errors.New("invalid search query")
		}
		files, err := r.list(ctx)
		if err != nil {
			return "", err
		}
		matches := make([]string, 0, 10)
		checked := 0
		fileLimit := 40
		if r.repository != "" {
			fileLimit = 12
		}
		for _, file := range files {
			if checked == fileLimit || len(matches) == 10 {
				break
			}
			content, err := r.readFile(ctx, file)
			if err != nil {
				continue
			}
			checked++
			for lineNo, line := range strings.Split(content, "\n") {
				if strings.Contains(line, call.Query) {
					matches = append(matches, fmt.Sprintf("%s:%d:%s", file, lineNo+1, boundedLine(line)))
					if len(matches) == 10 {
						break
					}
				}
			}
		}
		return strings.Join(matches, "\n"), nil
	default:
		return "", errors.New("unsupported review tool")
	}
}

func (r *Reader) readFile(ctx context.Context, name string) (string, error) {
	if !safePath(name) {
		return "", errors.New("invalid tool path")
	}
	if r.repository != "" {
		if _, err := r.list(ctx); err != nil {
			return "", err
		}
		r.mu.Lock()
		blob := r.remoteBlobs[name]
		r.mu.Unlock()
		if blob == "" {
			return "", errors.New("file is unavailable or not a regular file")
		}
		var result struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
			Size     int    `json:"size"`
		}
		if err := r.api(ctx, "/git/blobs/"+blob, &result); err != nil {
			return "", err
		}
		if result.Encoding != "base64" || result.Size > maxFileBytes {
			return "", errors.New("file exceeds read limit")
		}
		content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(result.Content, "\n", ""))
		if err != nil || len(content) > maxFileBytes || bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
			return "", errors.New("file is binary or exceeds read limit")
		}
		return string(content), nil
	}
	entry, err := r.git(ctx, "ls-tree", "-z", r.revision, "--", name)
	if err != nil {
		return "", err
	}
	entry = bytes.TrimSuffix(entry, []byte{0})
	meta, listed, ok := bytes.Cut(entry, []byte{'\t'})
	fields := strings.Fields(string(meta))
	if !ok || string(listed) != name || len(fields) != 3 || fields[1] != "blob" || fields[0] != "100644" && fields[0] != "100755" {
		return "", errors.New("file is unavailable or not a regular file")
	}
	size, err := r.git(ctx, "cat-file", "-s", fields[2])
	if err != nil {
		return "", err
	}
	var count int
	if _, err := fmt.Sscanf(string(size), "%d", &count); err != nil || count > maxFileBytes || count < 0 {
		return "", errors.New("file exceeds read limit")
	}
	content, err := r.git(ctx, "cat-file", "blob", fields[2])
	if err != nil {
		return "", err
	}
	if len(content) > maxFileBytes || bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
		return "", errors.New("file is binary or exceeds read limit")
	}
	return string(content), nil
}

func (r *Reader) list(ctx context.Context) ([]string, error) {
	if r.repository != "" {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.remoteFiles != nil {
			return r.remoteFiles, nil
		}
		var tree struct {
			Truncated bool `json:"truncated"`
			Tree      []struct {
				Path string `json:"path"`
				Mode string `json:"mode"`
				Type string `json:"type"`
				SHA  string `json:"sha"`
				Size int    `json:"size"`
			} `json:"tree"`
		}
		if err := r.api(ctx, "/git/trees/"+r.revision+"?recursive=1", &tree); err != nil {
			return nil, err
		}
		if tree.Truncated || len(tree.Tree) > 2_000 {
			return nil, errors.New("repository tree exceeds limit")
		}
		r.remoteBlobs = map[string]string{}
		r.remoteFiles = []string{}
		for _, entry := range tree.Tree {
			if entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" || !safePath(entry.Path) {
				continue
			}
			r.remoteFiles = append(r.remoteFiles, entry.Path)
			if entry.Size >= 0 && entry.Size <= maxFileBytes && revisionPattern.MatchString(entry.SHA) {
				r.remoteBlobs[entry.Path] = entry.SHA
			}
		}
		return r.remoteFiles, nil
	}
	data, err := r.git(ctx, "ls-tree", "-r", "-z", "--name-only", r.revision)
	if err != nil {
		return nil, err
	}
	if len(data) > maxListBytes {
		return nil, errors.New("repository file list exceeds limit")
	}
	names := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	if len(names) > 2_000 {
		return nil, errors.New("repository file count exceeds limit")
	}
	return names, nil
}

func (r *Reader) api(ctx context.Context, suffix string, result any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+r.repository+suffix, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+r.token)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := r.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("repository API returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxListBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxListBytes {
		return errors.New("repository response exceeds limit")
	}
	return json.Unmarshal(data, result)
}

func (r *Reader) git(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.root, "-c", "core.quotePath=false"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1", "GIT_CONFIG_NOSYSTEM=1")
	output := &limitedOutput{}
	cmd.Stdout, cmd.Stderr = output, io.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

type limitedOutput struct{ bytes.Buffer }

func (out *limitedOutput) Write(data []byte) (int, error) {
	if out.Len()+len(data) > maxListBytes {
		return 0, errors.New("repository output exceeds limit")
	}
	return out.Buffer.Write(data)
}

func safePath(name string) bool {
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\x00\r\n") || strings.HasPrefix(name, "-") {
		return false
	}
	for _, part := range strings.Split(strings.ToLower(name), "/") {
		if part == ".git" || part == "_research_refs" || strings.HasPrefix(part, ".env") || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") || part == "id_rsa" || part == "credentials" {
			return false
		}
	}
	return true
}

func bounded(value string) string {
	if len(value) > maxFileBytes {
		return value[:maxFileBytes] + "\n[truncated]"
	}
	return value
}

func boundedLine(value string) string {
	if len(value) > 200 {
		return value[:200] + "..."
	}
	return value
}

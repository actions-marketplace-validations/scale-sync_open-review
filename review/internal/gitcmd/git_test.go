package gitcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func command(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	command(t, root, "init", "-q")
	command(t, root, "config", "user.name", "Test")
	command(t, root, "config", "user.email", "test@example.com")
	return root
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceIncludesStagedUnstagedAndUntracked(t *testing.T) {
	root := newRepo(t)
	write(t, root, "staged.txt", "old\n")
	write(t, root, "unstaged.txt", "old\n")
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "initial")
	write(t, root, "staged.txt", "new staged\n")
	command(t, root, "add", "staged.txt")
	write(t, root, "unstaged.txt", "new unstaged\n")
	write(t, root, "name with spaces.txt", "new untracked\n")
	input, err := Prepare(context.Background(), Options{Repo: root, Mode: "workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Files) != 3 {
		t.Fatalf("files = %q", input.Files)
	}
	for _, text := range []string{"new staged", "new unstaged", "new untracked", "name with spaces.txt"} {
		if !strings.Contains(input.Diff, text) {
			t.Fatalf("missing %q in diff: %s", text, input.Diff)
		}
	}
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "all")
	empty, err := Prepare(context.Background(), Options{Repo: root, Mode: "workspace"})
	if err != nil || empty.Diff != "" || len(empty.Files) != 0 {
		t.Fatalf("empty workspace = %+v, %v", empty, err)
	}
}

func TestBranchUsesMergeBase(t *testing.T) {
	root := newRepo(t)
	write(t, root, "base.txt", "base\n")
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "initial")
	initial := command(t, root, "rev-parse", "HEAD")
	command(t, root, "branch", "topic")
	write(t, root, "base.txt", "base changed\n")
	command(t, root, "commit", "-qam", "base changes")
	baseSHA := command(t, root, "rev-parse", "HEAD")
	command(t, root, "checkout", "-q", "topic")
	write(t, root, "topic.txt", "topic change\n")
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "topic changes")
	input, err := Prepare(context.Background(), Options{Repo: root, Mode: "branch", Base: baseSHA, Head: "topic"})
	if err != nil {
		t.Fatal(err)
	}
	if input.MergeBaseSHA != initial || input.BaseSHA != baseSHA || len(input.Files) != 1 || input.Files[0] != "topic.txt" {
		t.Fatalf("branch input = %+v", input)
	}
	if strings.Contains(input.Diff, "base changed") {
		t.Fatalf("base side leaked into diff: %s", input.Diff)
	}
}

func TestCommitRootRenameDeleteEmptyAndMerge(t *testing.T) {
	root := newRepo(t)
	write(t, root, "old.txt", "first\n")
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "root")
	rootSHA := command(t, root, "rev-parse", "HEAD")
	first, err := Prepare(context.Background(), Options{Repo: root, Mode: "commit", Commit: rootSHA})
	if err != nil || len(first.Parents) != 0 || len(first.Files) != 1 || !strings.Contains(first.Diff, "first") {
		t.Fatalf("root commit = %+v, %v", first, err)
	}
	command(t, root, "mv", "old.txt", "renamed.txt")
	command(t, root, "commit", "-qm", "rename")
	rename, err := Prepare(context.Background(), Options{Repo: root, Mode: "commit", Commit: "HEAD"})
	if err != nil || len(rename.Files) != 1 || rename.Files[0] != "renamed.txt" || !strings.Contains(rename.Diff, "rename from old.txt") {
		t.Fatalf("rename = %+v, %v", rename, err)
	}
	command(t, root, "rm", "renamed.txt")
	command(t, root, "commit", "-qm", "delete")
	deletion, err := Prepare(context.Background(), Options{Repo: root, Mode: "commit", Commit: "HEAD"})
	if err != nil || len(deletion.Files) != 1 || deletion.Files[0] != "renamed.txt" || !strings.Contains(deletion.Diff, "deleted file mode") {
		t.Fatalf("deletion = %+v, %v", deletion, err)
	}
	command(t, root, "commit", "--allow-empty", "-qm", "empty")
	empty, err := Prepare(context.Background(), Options{Repo: root, Mode: "commit", Commit: "HEAD"})
	if err != nil || len(empty.Files) != 0 || empty.Diff != "" {
		t.Fatalf("empty commit = %+v, %v", empty, err)
	}
	mainBranch := command(t, root, "branch", "--show-current")
	command(t, root, "branch", "side")
	write(t, root, "main.txt", "main\n")
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "main")
	command(t, root, "checkout", "-q", "side")
	write(t, root, "side.txt", "side\n")
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "side")
	command(t, root, "checkout", "-q", mainBranch)
	command(t, root, "merge", "--no-ff", "-qm", "merge", "side")
	merge, err := Prepare(context.Background(), Options{Repo: root, Mode: "commit", Commit: "HEAD"})
	if err != nil || len(merge.Parents) != 2 || len(merge.Files) != 1 || merge.Files[0] != "side.txt" {
		t.Fatalf("merge = %+v, %v", merge, err)
	}
}

func TestRejectsOutsideRepository(t *testing.T) {
	_, err := Prepare(context.Background(), Options{Repo: t.TempDir(), Mode: "workspace"})
	if err == nil {
		t.Fatal("expected non-repository error")
	}
}

func TestCommitPreservesCopyAndBinaryMetadata(t *testing.T) {
	root := newRepo(t)
	original := strings.Repeat("line of reusable content\n", 30)
	write(t, root, "source.go", original)
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "initial")
	write(t, root, "source.go", original+"one more line\n")
	write(t, root, "copy.go", original)
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte{0, 1, 2, 3}, 0600); err != nil {
		t.Fatal(err)
	}
	command(t, root, "add", ".")
	command(t, root, "commit", "-qm", "copy and binary")
	input, err := Prepare(context.Background(), Options{Repo: root, Mode: "commit", Commit: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	seenCopy, seenBinary := false, false
	for _, change := range input.Changes {
		if change.Path == "copy.go" {
			seenCopy = change.Status == "copied" && change.OldPath == "source.go"
		}
		if change.Path == "logo.png" {
			seenBinary = change.Binary && len(change.Hunks) == 0
		}
	}
	if !seenCopy || !seenBinary {
		t.Fatalf("copy or binary missing: %+v", input.Changes)
	}
}

func TestWorkspaceRecordsUnsafeAndOversizedUntrackedFiles(t *testing.T) {
	root := newRepo(t)
	if err := os.Symlink("/tmp/outside", filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.go"), make([]byte, maxUntrackedBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := Prepare(context.Background(), Options{Repo: root, Mode: "workspace"})
	if err != nil {
		t.Fatal(err)
	}
	issues := map[string]string{}
	for _, change := range input.Changes {
		issues[change.Path] = change.InputIssue
	}
	if issues["link.go"] != "unsafe_input" || issues["large.go"] != "size_limit" || input.Diff != "" {
		t.Fatalf("issues = %+v; diff = %q", issues, input.Diff)
	}
}

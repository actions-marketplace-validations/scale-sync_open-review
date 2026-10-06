package gitcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/scale-sync/open-review/review/internal/diff"
)

const maxUntrackedBytes = 5 << 20

type Input struct {
	Mode         string
	RepoRoot     string
	BaseRef      string
	HeadRef      string
	BaseSHA      string
	MergeBaseSHA string
	HeadSHA      string
	CommitSHA    string
	Parents      []string
	Diff         string
	Files        []string
	Changes      []diff.Change
}

type Options struct {
	Repo   string
	Mode   string
	Base   string
	Head   string
	Commit string
}

func Prepare(ctx context.Context, opts Options) (Input, error) {
	repo := opts.Repo
	if repo == "" {
		repo = "."
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return Input{}, err
	}
	rootBytes, err := git(ctx, abs, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return Input{}, fmt.Errorf("open repository: %w", err)
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return Input{}, err
	}
	in := Input{Mode: opts.Mode, RepoRoot: root, Files: []string{}}
	switch opts.Mode {
	case "workspace":
		err = workspace(ctx, &in)
	case "branch":
		err = branch(ctx, &in, opts.Base, opts.Head)
	case "commit":
		err = commit(ctx, &in, opts.Commit)
	default:
		return Input{}, fmt.Errorf("unknown review mode %q", opts.Mode)
	}
	return in, err
}

func workspace(ctx context.Context, in *Input) error {
	head, err := resolveCommit(ctx, in.RepoRoot, "HEAD")
	if err != nil {
		head, err = emptyTree(ctx, in.RepoRoot)
		if err != nil {
			return err
		}
	} else {
		in.BaseSHA = head
	}
	patch, changes, err := diffBetween(ctx, in.RepoRoot, head)
	if err != nil {
		return err
	}
	in.Diff = patch
	in.Changes = changes
	for _, change := range changes {
		in.Files = append(in.Files, change.Path)
	}

	names, err := git(ctx, in.RepoRoot, nil, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	for _, path := range splitNUL(names) {
		if !filepath.IsLocal(path) {
			return fmt.Errorf("unsafe untracked path %q", path)
		}
		full := filepath.Join(in.RepoRoot, path)
		info, err := os.Lstat(full)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			in.Files = append(in.Files, path)
			in.Changes = append(in.Changes, diff.Change{Path: path, Status: "added", Untracked: true, InputIssue: "unsafe_input"})
			continue
		}
		if info.Size() > maxUntrackedBytes {
			in.Files = append(in.Files, path)
			in.Changes = append(in.Changes, diff.Change{Path: path, Status: "added", Untracked: true, InputIssue: "size_limit"})
			continue
		}
		part, err := gitDiffNoIndex(ctx, in.RepoRoot, path)
		if err != nil {
			return err
		}
		in.Diff += part
		in.Files = append(in.Files, path)
		in.Changes = append(in.Changes, diff.Untracked(path, part))
	}
	return nil
}

func branch(ctx context.Context, in *Input, base, head string) error {
	if base == "" || head == "" {
		return errors.New("branch review needs both --base and --head")
	}
	var err error
	in.BaseSHA, err = resolveCommit(ctx, in.RepoRoot, base)
	if err != nil {
		return fmt.Errorf("base %q: %w", base, err)
	}
	in.HeadSHA, err = resolveCommit(ctx, in.RepoRoot, head)
	if err != nil {
		return fmt.Errorf("head %q: %w", head, err)
	}
	in.BaseRef, in.HeadRef = base, head
	mergeBase, err := git(ctx, in.RepoRoot, nil, "merge-base", in.BaseSHA, in.HeadSHA)
	if err != nil {
		return fmt.Errorf("merge base: %w", err)
	}
	in.MergeBaseSHA = strings.TrimSpace(string(mergeBase))
	in.Diff, in.Changes, err = diffBetween(ctx, in.RepoRoot, in.MergeBaseSHA, in.HeadSHA)
	for _, change := range in.Changes {
		in.Files = append(in.Files, change.Path)
	}
	return err
}

func commit(ctx context.Context, in *Input, ref string) error {
	if ref == "" {
		return errors.New("commit review needs --commit")
	}
	sha, err := resolveCommit(ctx, in.RepoRoot, ref)
	if err != nil {
		return fmt.Errorf("commit %q: %w", ref, err)
	}
	in.CommitSHA, in.HeadSHA = sha, sha
	line, err := git(ctx, in.RepoRoot, nil, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return err
	}
	parts := strings.Fields(string(line))
	if len(parts) == 0 || parts[0] != sha {
		return errors.New("could not read commit parents")
	}
	in.Parents = parts[1:]
	if len(in.Parents) == 0 {
		in.BaseSHA, err = emptyTree(ctx, in.RepoRoot)
	} else {
		in.BaseSHA = in.Parents[0] // Merge commits use the first parent.
	}
	if err != nil {
		return err
	}
	in.Diff, in.Changes, err = diffBetween(ctx, in.RepoRoot, in.BaseSHA, sha)
	for _, change := range in.Changes {
		in.Files = append(in.Files, change.Path)
	}
	return err
}

func diffBetween(ctx context.Context, root string, revs ...string) (string, []diff.Change, error) {
	args := append([]string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--find-copies", "--binary"}, revs...)
	patch, err := git(ctx, root, nil, append(args, "--")...)
	if err != nil {
		return "", nil, err
	}
	nameArgs := append([]string{"diff", "--name-status", "-z", "--find-renames", "--find-copies"}, revs...)
	names, err := git(ctx, root, nil, append(nameArgs, "--")...)
	if err != nil {
		return "", nil, err
	}
	changes, err := diff.Parse(string(patch), names)
	return string(patch), changes, err
}

func resolveCommit(ctx context.Context, root, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" || strings.ContainsAny(ref, "\x00\r\n") || strings.HasPrefix(ref, "-") {
		return "", errors.New("invalid Git ref")
	}
	out, err := git(ctx, root, nil, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func emptyTree(ctx context.Context, root string) (string, error) {
	out, err := git(ctx, root, []byte{}, "hash-object", "-t", "tree", "--stdin")
	return strings.TrimSpace(string(out)), err
}

func gitDiffNoIndex(ctx context.Context, root, path string) (string, error) {
	out, err := git(ctx, root, nil, "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-index", "--", os.DevNull, path)
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return "", err
		}
	}
	return string(out), nil
}

func git(ctx context.Context, root string, stdin []byte, args ...string) ([]byte, error) {
	argv := append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1", "GIT_CONFIG_NOSYSTEM=1")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out, fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(string(exit.Stderr)), err)
		}
		return out, err
	}
	return out, nil
}

func splitNUL(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	parts := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			out = append(out, string(part))
		}
	}
	return out
}

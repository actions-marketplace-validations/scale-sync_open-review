package scan

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/scale-sync/open-review/review/internal/diff"
)

const maxInputBytes = 5 << 20

func Prepare(repo, selected string) (string, []diff.Change, error) {
	if repo == "" {
		repo = "."
	}
	root, err := filepath.Abs(repo)
	if err != nil {
		return "", nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", nil, errors.New("scan repository must be a directory")
	}
	if selected == "" {
		selected = "."
	}
	if !filepath.IsLocal(selected) && selected != "." {
		return "", nil, errors.New("scan path must stay inside repository")
	}
	target := filepath.Join(root, selected)
	info, err = os.Lstat(target)
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", nil, errors.New("scan path cannot be a symlink")
	}
	changes := []diff.Change{}
	visit := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("scan path escaped repository")
		}
		if entry.IsDir() {
			if path != target && ignoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.ToSlash(rel)
		change := diff.Change{Path: name, Status: "added"}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			change.InputIssue = "unsafe_input"
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Size() > maxInputBytes {
				change.InputIssue = "size_limit"
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
					change.Binary = true
				} else if len(data) == 0 {
					change.InputIssue = "empty_file"
				} else {
					change.Patch = patch(name, string(data))
					change = diff.FromPatch(change)
				}
			}
		}
		changes = append(changes, change)
		return nil
	}
	if info.IsDir() {
		if err := filepath.WalkDir(target, visit); err != nil {
			return "", nil, err
		}
	} else {
		entry, err := os.ReadDir(filepath.Dir(target))
		if err != nil {
			return "", nil, err
		}
		found := false
		for _, item := range entry {
			if item.Name() == filepath.Base(target) {
				if err := visit(target, item, nil); err != nil {
					return "", nil, err
				}
				found = true
				break
			}
		}
		if !found {
			return "", nil, fmt.Errorf("scan file disappeared: %s", selected)
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return root, changes, nil
}

func ignoredDirectory(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", "coverage", "_research_refs":
		return true
	}
	return false
}

func patch(name, content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	a, b := "a/"+name, "b/"+name
	if strings.ContainsAny(name, " \t\"\\") {
		a, b = strconv.Quote(a), strconv.Quote(b)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "diff --git %s %s\nnew file mode 100644\n--- /dev/null\n+++ %s\n@@ -0,0 +1,%d @@\n", a, b, b, len(lines))
	for _, line := range lines {
		out.WriteByte('+')
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

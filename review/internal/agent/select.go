package agent

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

type Selection struct {
	Include       []string
	Exclude       []string
	MaxFiles      int
	MaxFileBytes  int
	MaxPatchBytes int
}

const truncationNotice = "\n[... patch truncated by review budget ...]\n"

func Select(req model.Request, opts Selection) (model.Request, error) {
	if opts.MaxFiles == 0 {
		opts.MaxFiles = 100
	}
	if opts.MaxFileBytes == 0 {
		opts.MaxFileBytes = 1 << 20
	}
	if opts.MaxPatchBytes == 0 {
		opts.MaxPatchBytes = 38_000
	}
	if opts.MaxFiles < 0 || opts.MaxFileBytes < 0 || opts.MaxPatchBytes < 1024 || opts.MaxPatchBytes > 38_000 {
		return req, errors.New("invalid review limits")
	}
	for _, pattern := range append(append([]string{}, opts.Include...), opts.Exclude...) {
		if pattern == "" {
			return req, errors.New("empty file pattern")
		}
		for _, segment := range strings.Split(strings.TrimSuffix(pattern, "/"), "/") {
			if segment != "**" {
				if _, err := path.Match(segment, ""); err != nil {
					return req, fmt.Errorf("invalid file pattern %q: %w", pattern, err)
				}
			}
		}
	}
	changes := append([]diff.Change{}, req.Changes...)
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	req.Files = []string{}
	req.Diff = ""
	req.Coverage = make([]model.CoverageEntry, 0, len(changes))
	for _, change := range changes {
		entry := model.CoverageEntry{
			Path:       change.Path,
			OldPath:    change.OldPath,
			Status:     change.Status,
			PatchBytes: len(change.Patch),
			State:      "selected",
		}
		included := matchesAny(opts.Include, change.Path)
		defaultSkip := defaultReason(change.Path)
		switch {
		case !safePath(change.Path) || change.OldPath != "" && !safePath(change.OldPath):
			entry.State, entry.Reason = "skipped", "unsafe_path"
		case change.InputIssue != "":
			entry.State, entry.Reason = "skipped", change.InputIssue
		case change.Binary || symlinkPatch(change.Patch):
			entry.State, entry.Reason = "skipped", "binary_or_symlink"
		case matchesAny(opts.Exclude, change.Path):
			entry.State, entry.Reason = "skipped", "user_excluded"
		case len(opts.Include) > 0 && !included:
			entry.State, entry.Reason = "skipped", "not_included"
		case !included && defaultSkip != "":
			entry.State, entry.Reason = "skipped", defaultSkip
		case len(change.Patch) > opts.MaxFileBytes:
			entry.State, entry.Reason = "skipped", "size_limit"
		case len(req.Files) >= opts.MaxFiles:
			entry.State, entry.Reason = "skipped", "file_limit"
		default:
			patch := change.Patch
			if len(patch) > opts.MaxPatchBytes {
				cut := strings.LastIndexByte(patch[:opts.MaxPatchBytes-len(truncationNotice)], '\n')
				if cut < 0 {
					entry.State, entry.Reason = "skipped", "patch_budget"
				} else {
					patch = patch[:cut+1] + truncationNotice
					entry.State, entry.Reason = "partial", "budget_truncated"
				}
			}
			if entry.State != "skipped" {
				entry.SentBytes = len(patch)
				req.Files = append(req.Files, change.Path)
				req.Diff += patch
			}
		}
		req.Coverage = append(req.Coverage, entry)
	}
	return req, nil
}

func symlinkPatch(patch string) bool {
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasSuffix(line, " mode 120000") && (strings.HasPrefix(line, "new file mode ") || strings.HasPrefix(line, "deleted file mode ") || strings.HasPrefix(line, "old mode ") || strings.HasPrefix(line, "new mode ")) {
			return true
		}
	}
	return false
}

func safePath(name string) bool {
	return name != "" && !path.IsAbs(name) && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../") && !strings.ContainsRune(name, 0)
}

func defaultReason(name string) string {
	parts := strings.Split(strings.ToLower(name), "/")
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "node_modules", "vendor", "dist", "build", ".git", "coverage":
			return "dependency_or_build"
		}
	}
	base := parts[len(parts)-1]
	if strings.Contains(base, ".generated.") || strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".min.css") || base == "pnpm-lock.yaml" || base == "package-lock.json" || base == "yarn.lock" {
		return "generated_or_lockfile"
	}
	if base == "dockerfile" || base == "makefile" || base == "readme" || base == "license" || base == "changelog" || base == "contributing" || base == ".gitignore" || base == ".env.example" {
		return ""
	}
	ext := path.Ext(base)
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py", ".rs", ".java", ".kt", ".kts", ".rb", ".sh", ".bash", ".zsh", ".sql", ".md", ".json", ".jsonc", ".yml", ".yaml", ".toml", ".xml", ".html", ".css", ".scss", ".txt", ".mod", ".proto", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".php", ".swift", ".scala", ".ex", ".exs", ".tf", ".hcl", ".graphql", ".vue", ".svelte", ".astro", ".gradle":
		return ""
	default:
		return "unsupported_file"
	}
}

func matchesAny(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/") && strings.HasPrefix(name, pattern) {
			return true
		}
		if !strings.Contains(pattern, "/") {
			matched, _ := path.Match(pattern, path.Base(name))
			if matched {
				return true
			}
			continue
		}
		if matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/")) {
			return true
		}
	}
	return false
}

func matchSegments(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}
	if pattern[0] == "**" {
		return matchSegments(pattern[1:], name) || len(name) > 0 && matchSegments(pattern, name[1:])
	}
	if len(name) == 0 {
		return false
	}
	matched, _ := path.Match(pattern[0], name[0])
	return matched && matchSegments(pattern[1:], name[1:])
}

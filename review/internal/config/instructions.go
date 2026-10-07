package config

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var instructionNames = []string{"REVIEW.md", "AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules"}

func Instructions(root string, files []string) string {
	dirs := map[string]bool{".": true}
	for _, name := range files {
		dir := filepath.Dir(filepath.FromSlash(name))
		for dir != "." && dir != "/" {
			dirs[dir] = true
			dir = filepath.Dir(dir)
		}
	}
	ordered := make([]string, 0, len(dirs))
	for dir := range dirs {
		ordered = append(ordered, dir)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := strings.Count(ordered[i], string(filepath.Separator)), strings.Count(ordered[j], string(filepath.Separator))
		if ordered[i] == "." {
			a = -1
		}
		if ordered[j] == "." {
			b = -1
		}
		if a != b {
			return a < b
		}
		return ordered[i] < ordered[j]
	})
	parts := make([]string, 0)
	remaining := int64(40_000)
	for _, dir := range ordered {
		for _, name := range instructionNames {
			if remaining <= 0 {
				return strings.Join(parts, "\n\n")
			}
			full := filepath.Join(root, dir, name)
			info, err := os.Lstat(full)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			file, err := os.Open(full)
			if err != nil {
				continue
			}
			raw, readErr := io.ReadAll(io.LimitReader(file, remaining))
			_ = file.Close()
			if readErr != nil || len(raw) == 0 {
				continue
			}
			parts = append(parts, "## "+filepath.ToSlash(filepath.Join(dir, name))+"\n"+string(raw))
			remaining -= int64(len(raw))
		}
	}
	return strings.Join(parts, "\n\n")
}

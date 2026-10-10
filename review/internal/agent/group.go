package agent

import (
	"errors"
	"path"
	"strconv"
	"strings"

	"github.com/scale-sync/open-review/review/internal/model"
)

const MaxGroupBytes = 40_000

type Group struct {
	Files []string
	Patch string
}

// Groups keeps every selected patch in exactly one bounded, deterministic group.
func Groups(req model.Request) ([]Group, error) {
	parts := strings.Split(req.Diff, "\ndiff --git ")
	if len(parts) != len(req.Files) || len(req.Files) == 0 {
		return nil, errors.New("selected files and patches do not match")
	}
	groups := make([]Group, 0, len(parts))
	byDir := map[string]int{}
	seen := map[string]bool{}
	for i, file := range req.Files {
		if file == "" || seen[file] || !strings.HasPrefix(parts[i], "diff --git ") && i == 0 {
			return nil, errors.New("duplicate or missing selected patch")
		}
		seen[file] = true
		patch := parts[i]
		if i > 0 {
			patch = "diff --git " + patch
		}
		header, _, _ := strings.Cut(patch, "\n")
		if !strings.HasPrefix(header, "diff --git ") || !strings.HasSuffix(header, " b/"+file) && !strings.HasSuffix(header, " "+strconv.Quote("b/"+file)) {
			return nil, errors.New("selected file and patch path do not match")
		}
		if len(patch) > MaxGroupBytes {
			return nil, errors.New("selected patch exceeds model budget")
		}
		dir := path.Dir(file)
		index, found := byDir[dir]
		separator := ""
		if found && !strings.HasSuffix(groups[index].Patch, "\n") {
			separator = "\n"
		}
		if !found || len(groups[index].Patch)+len(separator)+len(patch) > MaxGroupBytes {
			index = len(groups)
			groups = append(groups, Group{})
			byDir[dir] = index
			separator = ""
		}
		groups[index].Files = append(groups[index].Files, file)
		groups[index].Patch += separator + patch
	}
	return groups, nil
}

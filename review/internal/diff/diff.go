package diff

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Hunk struct {
	OldStart int      `json:"oldStart"`
	OldCount int      `json:"oldCount"`
	NewStart int      `json:"newStart"`
	NewCount int      `json:"newCount"`
	Lines    []string `json:"lines"`
}

type Change struct {
	Path       string `json:"path"`
	OldPath    string `json:"oldPath,omitempty"`
	Status     string `json:"status"`
	Untracked  bool   `json:"untracked,omitempty"`
	InputIssue string `json:"inputIssue,omitempty"`
	Binary     bool   `json:"binary"`
	Patch      string `json:"patch"`
	Hunks      []Hunk `json:"hunks"`
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func Parse(patch string, nameStatus []byte) ([]Change, error) {
	if patch != "" && !strings.HasPrefix(patch, "diff --git ") {
		return nil, fmt.Errorf("invalid Git patch header")
	}
	parts := splitPatch(patch)
	tokens := bytes.Split(bytes.TrimSuffix(nameStatus, []byte{0}), []byte{0})
	if len(nameStatus) == 0 {
		tokens = nil
	}
	changes := make([]Change, 0, len(parts))
	for i := 0; i < len(tokens); {
		status := string(tokens[i])
		i++
		if status == "" || i >= len(tokens) {
			return nil, fmt.Errorf("malformed Git name-status output")
		}
		change := Change{Status: statusName(status[0])}
		if change.Status == "" {
			return nil, fmt.Errorf("unsupported Git status %q", status)
		}
		if status[0] == 'R' || status[0] == 'C' {
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("malformed Git %s paths", status)
			}
			change.OldPath = string(tokens[i])
			change.Path = string(tokens[i+1])
			i += 2
		} else {
			change.Path = string(tokens[i])
			i++
		}
		changes = append(changes, change)
	}
	if len(changes) != len(parts) {
		return nil, fmt.Errorf("Git metadata has %d files but patch has %d", len(changes), len(parts))
	}
	for i := range changes {
		changes[i].Patch = parts[i]
		changes[i].Binary = isBinary(parts[i])
		changes[i].Hunks = parseHunks(parts[i])
	}
	return changes, nil
}

func Untracked(path, patch string) Change {
	return Change{Path: path, Status: "added", Untracked: true, Binary: isBinary(patch), Patch: patch, Hunks: parseHunks(patch)}
}

func statusName(code byte) string {
	switch code {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "type_changed"
	case 'U':
		return "unmerged"
	default:
		return ""
	}
}

func splitPatch(patch string) []string {
	if patch == "" {
		return nil
	}
	var parts []string
	start := 0
	for pos := 0; pos < len(patch); {
		if pos == 0 || patch[pos-1] == '\n' {
			if strings.HasPrefix(patch[pos:], "diff --git ") {
				if pos > start {
					parts = append(parts, patch[start:pos])
				}
				start = pos
			}
		}
		next := strings.IndexByte(patch[pos:], '\n')
		if next < 0 {
			break
		}
		pos += next + 1
	}
	if start < len(patch) {
		parts = append(parts, patch[start:])
	}
	return parts
}

func isBinary(patch string) bool {
	return strings.Contains(patch, "\nGIT binary patch\n") || strings.Contains(patch, "\nBinary files ")
}

func parseHunks(patch string) []Hunk {
	var hunks []Hunk
	for _, line := range strings.Split(strings.TrimSuffix(patch, "\n"), "\n") {
		match := hunkHeader.FindStringSubmatch(line)
		if match != nil {
			oldStart, _ := strconv.Atoi(match[1])
			newStart, _ := strconv.Atoi(match[3])
			oldCount, newCount := 1, 1
			if match[2] != "" {
				oldCount, _ = strconv.Atoi(match[2])
			}
			if match[4] != "" {
				newCount, _ = strconv.Atoi(match[4])
			}
			hunks = append(hunks, Hunk{OldStart: oldStart, OldCount: oldCount, NewStart: newStart, NewCount: newCount, Lines: []string{}})
			continue
		}
		if len(hunks) > 0 && (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\\")) {
			last := &hunks[len(hunks)-1]
			last.Lines = append(last.Lines, line)
		}
	}
	return hunks
}

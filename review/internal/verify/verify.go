package verify

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

type addedLine struct {
	number int
	text   string
}

func Findings(req model.Request, input []json.RawMessage) ([]json.RawMessage, []model.AuditEvent) {
	output := make([]json.RawMessage, 0, len(input))
	seen := map[string]bool{}
	filtered, uncertain, relocated := 0, 0, 0
	for _, raw := range input {
		var finding model.Finding
		if json.Unmarshal(raw, &finding) != nil || finding.File == "" || finding.StartLine < 1 || finding.EndLine < finding.StartLine || finding.EndLine-finding.StartLine > 50 || strings.TrimSpace(finding.Title) == "" || strings.TrimSpace(finding.Explanation) == "" {
			filtered++
			continue
		}
		lines := changedLines(req.Changes, finding.File)
		if len(lines) == 0 {
			filtered++
			continue
		}
		status, reason := anchor(&finding, lines)
		finding.AnchorStatus = status
		finding.Verification = "verified"
		if status == "unanchored" {
			finding.Verification = "uncertain"
			finding.VerificationReason = reason
			finding.SuggestedFix = ""
			uncertain++
		} else if status == "relocated" {
			relocated++
		}
		key := fmt.Sprintf("%s\x00%d\x00%s", finding.File, finding.StartLine, strings.ToLower(strings.TrimSpace(finding.Title)))
		if seen[key] {
			filtered++
			continue
		}
		seen[key] = true
		encoded, _ := json.Marshal(finding)
		output = append(output, encoded)
	}
	return output, []model.AuditEvent{{Stage: "finding_verification", Status: "complete", Message: fmt.Sprintf("filtered=%d uncertain=%d relocated=%d", filtered, uncertain, relocated)}}
}

func changedLines(changes []diff.Change, file string) []addedLine {
	var lines []addedLine
	for _, change := range changes {
		if change.Path != file {
			continue
		}
		for _, hunk := range change.Hunks {
			line := hunk.NewStart
			for _, content := range hunk.Lines {
				if strings.HasPrefix(content, "+") && !strings.HasPrefix(content, "+++") {
					lines = append(lines, addedLine{line, strings.TrimPrefix(content, "+")})
				}
				if strings.HasPrefix(content, "+") || strings.HasPrefix(content, " ") {
					line++
				}
			}
		}
	}
	return lines
}

func anchor(f *model.Finding, lines []addedLine) (string, string) {
	snippet := strings.Split(strings.TrimSuffix(f.CitedSnippet, "\n"), "\n")
	if f.CitedSnippet != "" {
		candidates := []int{}
		for i := range lines {
			if len(snippet) == 0 || len(snippet) > len(lines)-i {
				continue
			}
			match := true
			for offset, text := range snippet {
				if lines[i+offset].number != lines[i].number+offset || lines[i+offset].text != text {
					match = false
					break
				}
			}
			if match && abs(lines[i].number-f.StartLine) <= 20 {
				candidates = append(candidates, lines[i].number)
			}
		}
		if len(candidates) != 1 {
			return "unanchored", "cited code is absent or repeated near the reported line"
		}
		start, end := candidates[0], candidates[0]+len(snippet)-1
		status := "exact"
		if start != f.StartLine || end != f.EndLine {
			status = "relocated"
		}
		f.StartLine, f.EndLine = start, end
		f.Citations = []model.Citation{{File: f.File, StartLine: start, EndLine: end}}
		return status, ""
	}
	for line := f.StartLine; line <= f.EndLine; line++ {
		found := false
		for _, added := range lines {
			if added.number == line {
				found = true
				break
			}
		}
		if !found {
			return "unanchored", "reported range is outside added lines"
		}
	}
	return "exact", ""
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

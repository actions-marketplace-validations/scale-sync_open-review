package linter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/scale-sync/open-review/review/internal/agent"
	"github.com/scale-sync/open-review/review/internal/model"
)

type tool struct {
	name   string
	bin    string
	args   []string
	report string
}

type limitedOutput struct {
	bytes.Buffer
	truncated bool
}

func (out *limitedOutput) Write(input []byte) (int, error) {
	n := len(input)
	remaining := 1<<20 - out.Len()
	if n > remaining {
		out.truncated = true
	}
	if remaining > 0 {
		_, _ = out.Buffer.Write(input[:min(n, remaining)])
	}
	return n, nil
}

func Run(ctx context.Context, req model.Request, ai []json.RawMessage) ([]json.RawMessage, []model.AuditEvent) {
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("OPEN_REVIEW_TRUSTED_LINTERS") != "true" {
		return ai, []model.AuditEvent{{Stage: "linters", Status: "disabled", Message: "Action checkout is untrusted by default"}}
	}
	if req.WorktreeRoot == "" || len(req.Files) == 0 {
		return ai, nil
	}
	root, err := filepath.Abs(req.WorktreeRoot)
	if err != nil {
		return ai, []model.AuditEvent{{Stage: "linters", Status: "unavailable"}}
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return ai, []model.AuditEvent{{Stage: "linters", Status: "unavailable"}}
	}
	if req.HeadSHA != "" {
		head, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output()
		if err != nil || strings.TrimSpace(string(head)) != req.HeadSHA {
			return ai, []model.AuditEvent{{Stage: "linters", Status: "skipped", Message: "checkout does not match PR head"}}
		}
	}
	files := make([]string, 0, len(req.Files))
	for _, file := range req.Files {
		clean := filepath.Clean(filepath.FromSlash(file))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			continue
		}
		full := filepath.Join(root, clean)
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, filepath.ToSlash(clean))
	}
	if len(files) == 0 {
		return ai, []model.AuditEvent{{Stage: "linters", Status: "unavailable", Message: "selected files are not checked out"}}
	}
	home, err := os.MkdirTemp("", "open-review-linter-")
	if err != nil {
		return ai, []model.AuditEvent{{Stage: "linters", Status: "failed", Message: "cannot create isolated home"}}
	}
	defer os.RemoveAll(home)
	var js, py, shell []string
	for _, file := range files {
		switch filepath.Ext(file) {
		case ".ts", ".tsx", ".js", ".jsx":
			js = append(js, file)
		case ".py":
			py = append(py, file)
		case ".sh", ".bash", ".zsh", ".ksh":
			shell = append(shell, file)
		}
	}
	tools := []tool{}
	if len(js) > 0 {
		tools = append(tools, tool{name: "ESLint", bin: "eslint", args: append([]string{"--format", "json"}, js...)})
	}
	if len(py) > 0 {
		tools = append(tools, tool{name: "Ruff", bin: "ruff", args: append([]string{"check", "--output-format", "json"}, py...)})
	}
	for _, config := range []string{".semgrep.yml", ".semgrep.yaml"} {
		if info, err := os.Lstat(filepath.Join(root, config)); err == nil && info.Mode().IsRegular() {
			tools = append(tools, tool{name: "Semgrep", bin: "semgrep", args: append([]string{"scan", "--json", "--metrics=off", "--config", config}, files...)})
			break
		}
	}
	if len(shell) > 0 {
		tools = append(tools, tool{name: "ShellCheck", bin: "shellcheck", args: append([]string{"--format=json"}, shell...)})
	}
	report := filepath.Join(home, "gitleaks.json")
	tools = append(tools, tool{name: "Gitleaks", bin: "gitleaks", args: []string{"dir", "--report-format", "json", "--report-path", report, "--redact=100", "."}, report: report})

	merged := append([]json.RawMessage{}, ai...)
	audit := make([]model.AuditEvent, 0, len(tools))
	for _, t := range tools {
		if os.Getenv("LINTER_"+strings.ToUpper(t.bin)) == "false" {
			audit = append(audit, model.AuditEvent{Stage: t.name, Status: "disabled"})
			continue
		}
		bin, err := exec.LookPath(t.bin)
		if err != nil {
			audit = append(audit, model.AuditEvent{Stage: t.name, Status: "unavailable"})
			continue
		}
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		cmd := exec.CommandContext(checkCtx, bin, t.args...)
		cmd.Dir = root
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "NO_COLOR=1"}
		var output limitedOutput
		cmd.Stdout = &output
		cmd.Stderr = &bytes.Buffer{}
		err = cmd.Run()
		if t.report != "" {
			output.Reset()
			if file, openErr := os.Open(t.report); openErr == nil {
				_, readErr := io.Copy(&output, io.LimitReader(file, 1<<20+1))
				_ = file.Close()
				if readErr != nil {
					output.truncated = true
				}
			} else if err == nil {
				_, _ = output.Write([]byte("[]"))
			}
		}
		status := "complete"
		if checkCtx.Err() != nil || output.truncated {
			status = "failed"
		} else if err != nil && output.Len() == 0 || output.Len() > 0 && !json.Valid(output.Bytes()) {
			status = "failed"
		}
		cancel()
		audit = append(audit, model.AuditEvent{Stage: t.name, Status: status})
		if status != "complete" {
			continue
		}
		for _, finding := range parse(t.name, output.Bytes()) {
			finding.File = normalize(root, finding.File)
			if !contains(files, finding.File) || finding.StartLine < 1 || !agent.ChangedLine(req.Changes, finding.File, finding.StartLine) {
				continue
			}
			finding.EndLine = finding.StartLine
			finding.Citations = []model.Citation{{File: finding.File, StartLine: finding.StartLine, EndLine: finding.StartLine}}
			matched := false
			for i, raw := range merged {
				var existing model.Finding
				if json.Unmarshal(raw, &existing) == nil && (existing.Source == "ai" || existing.Source == "both") && existing.File == finding.File && existing.StartLine <= finding.StartLine && finding.StartLine <= existing.EndLine {
					existing.Source, existing.LinterName = "both", t.name
					merged[i], _ = json.Marshal(existing)
					matched = true
					break
				}
			}
			if !matched {
				raw, _ := json.Marshal(finding)
				merged = append(merged, raw)
			}
		}
	}
	return merged, audit
}

func parse(name string, raw []byte) []model.Finding {
	findings := []model.Finding{}
	add := func(file string, line int, title, message, severity string) {
		if file == "" || line < 1 {
			return
		}
		category := "flag"
		if severity == "severe" {
			category = "bug"
		}
		title = strings.TrimSpace(title)
		message = strings.TrimSpace(message)
		if len(title) > 120 {
			title = title[:120]
		}
		if len(message) > 2_000 {
			message = message[:2_000]
		}
		findings = append(findings, model.Finding{ID: strings.ToLower(name) + ":" + file + ":" + title, Category: category, Severity: severity, File: file, StartLine: line, EndLine: line, Title: title, Explanation: message, Source: "linter", LinterName: name})
	}
	switch name {
	case "ESLint":
		var results []struct {
			FilePath string `json:"filePath"`
			Messages []struct {
				Line     int    `json:"line"`
				RuleID   string `json:"ruleId"`
				Message  string `json:"message"`
				Severity int    `json:"severity"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &results)
		for _, file := range results {
			for _, m := range file.Messages {
				add(file.FilePath, m.Line, "ESLint: "+m.RuleID, m.Message, mapSeverity(m.Severity))
			}
		}
	case "Ruff":
		var results []struct {
			Filename string `json:"filename"`
			Code     string `json:"code"`
			Message  string `json:"message"`
			Location struct {
				Row int `json:"row"`
			} `json:"location"`
		}
		_ = json.Unmarshal(raw, &results)
		for _, m := range results {
			add(m.Filename, m.Location.Row, "Ruff: "+m.Code, m.Message, "non-severe")
		}
	case "Semgrep":
		var results struct {
			Results []struct {
				Path    string `json:"path"`
				CheckID string `json:"check_id"`
				Start   struct {
					Line int `json:"line"`
				} `json:"start"`
				Extra struct {
					Message  string `json:"message"`
					Severity string `json:"severity"`
				} `json:"extra"`
			} `json:"results"`
		}
		_ = json.Unmarshal(raw, &results)
		for _, m := range results.Results {
			severity := "non-severe"
			if m.Extra.Severity == "ERROR" {
				severity = "severe"
			}
			add(m.Path, m.Start.Line, "Semgrep: "+m.CheckID, m.Extra.Message, severity)
		}
	case "ShellCheck":
		var results []struct {
			File    string `json:"file"`
			Line    int    `json:"line"`
			Code    int    `json:"code"`
			Message string `json:"message"`
			Level   string `json:"level"`
		}
		_ = json.Unmarshal(raw, &results)
		for _, m := range results {
			severity := "informational"
			if m.Level == "error" {
				severity = "non-severe"
			}
			add(m.File, m.Line, "ShellCheck: SC"+strconv.Itoa(m.Code), m.Message, severity)
		}
	case "Gitleaks":
		var results []struct {
			File        string `json:"File"`
			StartLine   int    `json:"StartLine"`
			RuleID      string `json:"RuleID"`
			Description string `json:"Description"`
		}
		_ = json.Unmarshal(raw, &results)
		for _, m := range results {
			add(m.File, m.StartLine, "Gitleaks: "+m.RuleID, "Potential secret detected; inspect this line privately.", "severe")
		}
	}
	return findings
}

func normalize(root, file string) string {
	if filepath.IsAbs(file) {
		file, _ = filepath.Rel(root, file)
	}
	return filepath.ToSlash(filepath.Clean(file))
}

func contains(files []string, file string) bool {
	for _, candidate := range files {
		if candidate == file {
			return true
		}
	}
	return false
}

func mapSeverity(level int) string {
	if level == 2 {
		return "non-severe"
	}
	return "informational"
}

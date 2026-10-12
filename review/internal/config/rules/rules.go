package rules

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/scale-sync/open-review/review/internal/config"
	"github.com/scale-sync/open-review/review/internal/model"
)

var instructionNames = map[string]int{"REVIEW.md": 0, "AGENTS.md": 1, "CLAUDE.md": 2, ".cursorrules": 3, ".windsurfrules": 4}

type entry struct {
	pattern string
	text    string
	source  string
	mode    string
}

type pathRule struct {
	Path         string `yaml:"path"`
	Instructions string `yaml:"instructions"`
	Mode         string `yaml:"mode"`
}

type document struct {
	Profile              string     `yaml:"profile"`
	PathInstructions     []pathRule `yaml:"path_instructions"`
	Rules                []pathRule `yaml:"rules"`
	LabelingInstructions string     `yaml:"labeling_instructions"`
	FinishingTouches     string     `yaml:"finishing_touches"`
}

func LoadLocal(root string, files []string) ([]model.RuleSource, error) {
	var err error
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	dirs := map[string]bool{".": true}
	for _, file := range files {
		if !safe(file) {
			return nil, fmt.Errorf("unsafe selected file %q", file)
		}
		for dir := path.Dir(file); dir != "."; dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	sources := []model.RuleSource{}
	for dir := range dirs {
		for name := range instructionNames {
			relative := path.Join(dir, name)
			content, err := readRepoRegular(root, relative)
			if err != nil {
				return nil, err
			}
			if content != "" {
				sources = append(sources, model.RuleSource{Path: relative, Content: content})
			}
		}
	}
	content, err := readRepoRegular(root, ".reviewbuddy.yaml")
	if err != nil {
		return nil, err
	}
	if content != "" {
		sources = append(sources, model.RuleSource{Path: ".reviewbuddy.yaml", Content: content})
	}
	return sources, nil
}

func readRepoRegular(root, relative string) (string, error) {
	full := filepath.Join(root, filepath.FromSlash(relative))
	parent, err := filepath.EvalSymlinks(filepath.Dir(full))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	inside, err := filepath.Rel(root, parent)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("rule source escapes repository: %q", relative)
	}
	return readRegular(full)
}

func readRegular(name string) (string, error) {
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 40_000 {
		return "", fmt.Errorf("rule source %q must be a regular file under 40 KB", name)
	}
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 40_001))
	if err != nil {
		return "", err
	}
	if len(data) > 40_000 {
		return "", fmt.Errorf("rule source %q exceeds limit", name)
	}
	return string(data), nil
}

func Resolve(req model.Request) ([]model.RuleDiagnostic, error) {
	if len(req.Instructions) > 8_000 {
		return nil, errors.New("explicit instruction exceeds 8 KB")
	}
	repo, err := parseSources(req.RuleSources, req.Files)
	if err != nil {
		return nil, err
	}
	user := []entry{}
	if req.WorktreeRoot != "" {
		configPath, err := config.Path()
		if err != nil {
			return nil, err
		}
		content, err := readRegular(filepath.Join(filepath.Dir(configPath), "rules.yaml"))
		if err != nil {
			return nil, err
		}
		if content != "" {
			user, err = parseConfig("user:rules.yaml", content, true)
			if err != nil {
				return nil, err
			}
		}
	}
	diagnostics := make([]model.RuleDiagnostic, 0, len(req.Files))
	for _, file := range req.Files {
		if !safe(file) {
			return nil, fmt.Errorf("unsafe rule target %q", file)
		}
		parts, names := []string{}, []string{}
		apply := func(rule entry) error {
			matched, err := matches(rule.pattern, file)
			if err != nil {
				return err
			}
			if !matched {
				return nil
			}
			if rule.mode == "replace" {
				parts, names = nil, nil
			}
			parts, names = append(parts, rule.text), append(names, rule.source)
			return nil
		}
		if text := builtIn(file); text != "" {
			_ = apply(entry{pattern: "**", text: text, source: "built-in:" + path.Ext(file)})
		}
		for _, rule := range append(user, repo...) {
			if err := apply(rule); err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(req.Instructions) != "" {
			parts, names = append(parts, req.Instructions), append(names, "explicit-run")
		}
		diagnostics = append(diagnostics, model.RuleDiagnostic{Path: file, Sources: names, Text: strings.Join(parts, "\n\n")})
	}
	return diagnostics, nil
}

func parseSources(sources []model.RuleSource, files []string) ([]entry, error) {
	selected := make([]model.RuleSource, 0, len(sources))
	total := 0
	for _, source := range sources {
		if !safe(source.Path) {
			return nil, fmt.Errorf("unsafe rule source %q", source.Path)
		}
		base := path.Base(source.Path)
		if _, ok := instructionNames[base]; !ok && source.Path != ".reviewbuddy.yaml" {
			return nil, fmt.Errorf("unsupported rule source %q", source.Path)
		}
		if dir := path.Dir(source.Path); dir != "." {
			matchesSelected := false
			for _, file := range files {
				if strings.HasPrefix(file, dir+"/") {
					matchesSelected = true
					break
				}
			}
			if !matchesSelected {
				continue
			}
		}
		selected = append(selected, source)
		total += len(source.Content)
		if total > 40_000 {
			return nil, errors.New("rule sources exceed 40 KB")
		}
	}
	if len(selected) > 100 {
		return nil, errors.New("too many rule sources")
	}
	sort.SliceStable(selected, func(i, j int) bool {
		a, b := selected[i].Path, selected[j].Path
		if a == ".reviewbuddy.yaml" || b == ".reviewbuddy.yaml" {
			return b == ".reviewbuddy.yaml" && a != b
		}
		if strings.Count(a, "/") != strings.Count(b, "/") {
			return strings.Count(a, "/") < strings.Count(b, "/")
		}
		if path.Dir(a) == path.Dir(b) {
			return instructionNames[path.Base(a)] < instructionNames[path.Base(b)]
		}
		return a < b
	})
	rules := []entry{}
	for _, source := range selected {
		if source.Path == ".reviewbuddy.yaml" {
			parsed, err := parseConfig(source.Path, source.Content, false)
			if err != nil {
				return nil, err
			}
			rules = append(rules, parsed...)
			continue
		}
		pattern := "**"
		if dir := path.Dir(source.Path); dir != "." {
			pattern = dir + "/**"
		}
		rules = append(rules, entry{pattern: pattern, text: source.Content, source: source.Path})
	}
	return rules, nil
}

func parseConfig(source, content string, user bool) ([]entry, error) {
	var doc document
	decoder := yaml.NewDecoder(strings.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid extra YAML document in %s", source)
	}
	if doc.Profile != "" && doc.Profile != "assertive" && doc.Profile != "chill" {
		return nil, fmt.Errorf("invalid profile in %s", source)
	}
	if user && (doc.Profile != "" || len(doc.PathInstructions) > 0 || doc.LabelingInstructions != "" || doc.FinishingTouches != "") {
		return nil, fmt.Errorf("unsupported user rule fields in %s", source)
	}
	out := []entry{}
	if doc.Profile == "chill" {
		out = append(out, entry{pattern: "**", text: "Report only severe and non-severe bugs with concrete impact.", source: source + ":profile"})
	}
	if doc.LabelingInstructions != "" {
		out = append(out, entry{pattern: "**", text: doc.LabelingInstructions, source: source + ":labeling"})
	}
	for _, rule := range append(doc.PathInstructions, doc.Rules...) {
		if rule.Path == "" || rule.Instructions == "" || rule.Mode != "" && rule.Mode != "merge" && rule.Mode != "replace" {
			return nil, fmt.Errorf("invalid rule in %s", source)
		}
		if _, err := matches(rule.Path, "check.go"); err != nil {
			return nil, fmt.Errorf("invalid rule path in %s: %w", source, err)
		}
		out = append(out, entry{pattern: rule.Path, text: rule.Instructions, source: source + ":" + rule.Path, mode: rule.Mode})
	}
	return out, nil
}

func matches(pattern, name string) (bool, error) {
	if pattern == "**" {
		return true, nil
	}
	if !safe(strings.TrimSuffix(pattern, "/")) {
		return false, errors.New("unsafe rule glob")
	}
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(name, pattern), nil
	}
	parts := strings.Split(pattern, "/")
	for _, part := range parts {
		if part != "**" {
			if _, err := path.Match(part, ""); err != nil {
				return false, err
			}
		}
	}
	if len(parts) == 1 {
		return path.Match(pattern, path.Base(name))
	}
	var walk func([]string, []string) bool
	walk = func(glob, target []string) bool {
		if len(glob) == 0 {
			return len(target) == 0
		}
		if glob[0] == "**" {
			return walk(glob[1:], target) || len(target) > 0 && walk(glob, target[1:])
		}
		if len(target) == 0 {
			return false
		}
		ok, _ := path.Match(glob[0], target[0])
		return ok && walk(glob[1:], target[1:])
	}
	return walk(parts, strings.Split(name, "/")), nil
}

func safe(name string) bool {
	return name != "" && !path.IsAbs(name) && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\x00\r\n")
}

func builtIn(name string) string {
	switch strings.ToLower(path.Base(name)) {
	case "dockerfile", "makefile", ".gitignore", ".env.example":
		return "Check configuration safety, defaults, and compatibility."
	case "readme", "license", "changelog", "contributing":
		return "Check instructions and examples for accuracy."
	}
	switch path.Ext(name) {
	case ".go":
		return "Check Go error handling, context cancellation, and goroutine ownership."
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "Check type boundaries, async errors, and runtime validation."
	case ".py":
		return "Check exceptions, input validation, and resource cleanup."
	case ".rs":
		return "Check ownership, error propagation, and unsafe code boundaries."
	case ".kt", ".kts":
		return "Check nullability, coroutine cancellation, and error handling."
	case ".java", ".scala", ".cs", ".swift", ".rb", ".php", ".ex", ".exs":
		return "Check error handling, data boundaries, and resource ownership."
	case ".sh", ".bash", ".zsh":
		return "Check quoting, exit status handling, and unsafe input."
	case ".c", ".cc", ".cpp", ".h", ".hpp":
		return "Check memory ownership, bounds, and error paths."
	case ".sql", ".graphql":
		return "Check query correctness, authorization, and expensive operations."
	case ".html", ".css", ".scss", ".vue", ".svelte", ".astro":
		return "Check accessibility, user-visible behavior, and input handling."
	case ".yaml", ".yml", ".json", ".jsonc", ".toml", ".tf", ".hcl", ".xml", ".mod", ".gradle", ".proto":
		return "Check configuration safety, defaults, and compatibility."
	case ".md", ".txt":
		return "Check instructions and examples for accuracy."
	default:
		return ""
	}
}

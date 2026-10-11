package llm

import (
	"strings"
	"testing"
)

func TestCompressionKeepsTaskAndNewestEvidence(t *testing.T) {
	task := "Initial review task: check added lines in main.go."
	history := []toolRound{
		{name: "search", target: "old", result: strings.Repeat("older", 1_000)},
		{name: "read_file", target: "recent.go", result: "recent evidence: caller checks nil"},
		{name: "read_diff", target: "main.go", result: "newest evidence: +bad call"},
	}
	content, compressed := compose(task, history, 1_000)
	if !compressed || len(content) > 1_000 || !strings.Contains(content, task) || !strings.Contains(content, "Earlier tool rounds") || !strings.Contains(content, "search \"old\"") || !strings.Contains(content, "recent evidence: caller checks nil") || !strings.Contains(content, "newest evidence: +bad call") {
		t.Fatalf("compressed = %t, bytes = %d, prompt = %q", compressed, len(content), content)
	}
}

func TestShortTranscriptStaysWhole(t *testing.T) {
	content, compressed := compose("task", []toolRound{{name: "read_diff", target: "a.go", result: "complete evidence"}}, 1_000)
	if compressed || !strings.Contains(content, "complete evidence") {
		t.Fatalf("compressed = %t, prompt = %q", compressed, content)
	}
}

func BenchmarkComposeLongTranscript(b *testing.B) {
	history := []toolRound{
		{name: "search", target: "caller", result: strings.Repeat("caller evidence\n", 500)},
		{name: "read_file", target: "a.go", result: strings.Repeat("first file\n", 500)},
		{name: "read_file", target: "b.go", result: strings.Repeat("second file\n", 500)},
		{name: "read_diff", target: "main.go", result: strings.Repeat("changed line\n", 500)},
	}
	for range b.N {
		compose("initial task", history, 8_000)
	}
}

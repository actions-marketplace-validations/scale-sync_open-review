package verify

import (
	"encoding/json"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

func TestRelocationAndUncertainFindings(t *testing.T) {
	patch := "diff --git a/a.go b/a.go\n@@ -1,1 +10,5 @@\n+unique()\n+repeat()\n+repeat()\n context\n+last()\n"
	req := model.Request{Changes: []diff.Change{diff.FromPatch(diff.Change{Path: "a.go", Patch: patch})}}
	input := []json.RawMessage{}
	for _, candidate := range []model.Finding{
		{File: "a.go", StartLine: 8, EndLine: 8, CitedSnippet: "unique()", Title: "Moved", Explanation: "bad", SuggestedFix: "fixed()"},
		{File: "a.go", StartLine: 11, EndLine: 11, CitedSnippet: "repeat()", Title: "Repeated", Explanation: "bad", SuggestedFix: "fixed()"},
		{File: "a.go", StartLine: 13, EndLine: 13, Title: "Context", Explanation: "bad"},
		{File: "a.go", StartLine: 10, EndLine: 10, CitedSnippet: "unique()", Title: "Moved", Explanation: "bad"},
		{File: "a.go", StartLine: -1, EndLine: -1, Title: "Invalid", Explanation: "bad"},
	} {
		raw, _ := json.Marshal(candidate)
		input = append(input, raw)
	}
	output, audit := Findings(req, input)
	if len(output) != 3 || audit[0].Message != "filtered=2 uncertain=2 relocated=1" {
		t.Fatalf("output=%s audit=%+v", output, audit)
	}
	var moved, repeated, context model.Finding
	_ = json.Unmarshal(output[0], &moved)
	_ = json.Unmarshal(output[1], &repeated)
	_ = json.Unmarshal(output[2], &context)
	if moved.StartLine != 10 || moved.AnchorStatus != "relocated" || moved.Citations[0].StartLine != 10 || moved.SuggestedFix != "fixed()" {
		t.Fatalf("relocation: %+v", moved)
	}
	if repeated.Verification != "uncertain" || repeated.SuggestedFix != "" || context.AnchorStatus != "unanchored" {
		t.Fatalf("uncertain: %+v %+v", repeated, context)
	}
}

func TestMultiLineCitationAndBoundedFallback(t *testing.T) {
	patch := "diff --git a/a.go b/a.go\n@@ -0,0 +40,2 @@\n+first()\n+second()\n"
	req := model.Request{Changes: []diff.Change{diff.FromPatch(diff.Change{Path: "a.go", Patch: patch})}}
	input := []json.RawMessage{}
	for _, candidate := range []model.Finding{
		{File: "a.go", StartLine: 39, EndLine: 39, CitedSnippet: "first()\nsecond()", Title: "Range", Explanation: "bad"},
		{File: "a.go", StartLine: 1, EndLine: 1, CitedSnippet: "first()", Title: "Far", Explanation: "bad"},
	} {
		raw, _ := json.Marshal(candidate)
		input = append(input, raw)
	}
	output, _ := Findings(req, input)
	var multi, far model.Finding
	_ = json.Unmarshal(output[0], &multi)
	_ = json.Unmarshal(output[1], &far)
	if multi.StartLine != 40 || multi.EndLine != 41 || multi.Citations[0].EndLine != 41 || far.Verification != "uncertain" {
		t.Fatalf("multi=%+v far=%+v", multi, far)
	}
}

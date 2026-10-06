package agent

import (
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/diff"
	"github.com/scale-sync/open-review/review/internal/model"
)

func TestSelectionPrecedenceAndFullPatchPreservation(t *testing.T) {
	changes := []diff.Change{
		{Path: "src/keep.go", Status: "modified", Patch: "diff --git a/src/keep.go b/src/keep.go\n+keep\n"},
		{Path: "vendor/allowed.go", Status: "modified", Patch: "diff --git a/vendor/allowed.go b/vendor/allowed.go\n+allowed\n"},
		{Path: "src/blocked.go", Status: "modified", Patch: "diff --git a/src/blocked.go b/src/blocked.go\n+blocked\n"},
		{Path: "src/image.png", Status: "added", Binary: true, Patch: "diff --git a/src/image.png b/src/image.png\nGIT binary patch\n"},
		{Path: "src/link.go", Status: "added", Patch: "diff --git a/src/link.go b/src/link.go\nnew file mode 120000\n+../secret\n"},
		{Path: "../outside.go", Status: "added", Patch: "diff --git a/../outside.go b/../outside.go\n+unsafe\n"},
	}
	req, err := Select(model.Request{Changes: changes}, Selection{
		Include: []string{"**/*.go", "*.png"}, Exclude: []string{"src/blocked.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Files) != 2 || req.Files[0] != "src/keep.go" || req.Files[1] != "vendor/allowed.go" {
		t.Fatalf("selected: %q", req.Files)
	}
	if strings.Contains(req.Diff, "blocked") || strings.Contains(req.Diff, "GIT binary patch") || strings.Contains(req.Diff, "unsafe") {
		t.Fatalf("excluded content reached model: %s", req.Diff)
	}
	want := map[string]string{"src/blocked.go": "user_excluded", "src/image.png": "binary_or_symlink", "src/link.go": "binary_or_symlink", "../outside.go": "unsafe_path"}
	for _, entry := range req.Coverage {
		if reason, ok := want[entry.Path]; ok && (entry.State != "skipped" || entry.Reason != reason) {
			t.Fatalf("coverage %s: %+v", entry.Path, entry)
		}
	}
	if req.Changes[0].Patch != changes[0].Patch {
		t.Fatal("full patch was modified")
	}
}

func TestSelectionLimitsAndPartialCoverage(t *testing.T) {
	large := "diff --git a/large.go b/large.go\n" + strings.Repeat("+line\n", 300)
	req, err := Select(model.Request{Changes: []diff.Change{
		{Path: "a.go", Patch: "diff --git a/a.go b/a.go\n+ok\n"},
		{Path: "large.go", Patch: large},
		{Path: "z.go", Patch: "diff --git a/z.go b/z.go\n+late\n"},
	}}, Selection{MaxFiles: 2, MaxFileBytes: 2000, MaxPatchBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Files) != 2 || req.Coverage[1].State != "partial" || req.Coverage[1].Reason != "budget_truncated" || req.Coverage[2].Reason != "file_limit" {
		t.Fatalf("limits: files=%q coverage=%+v", req.Files, req.Coverage)
	}
	if req.Coverage[1].SentBytes > 1024 || req.Coverage[1].SentBytes >= req.Coverage[1].PatchBytes || !strings.Contains(req.Diff, "patch truncated") || !strings.Contains(req.Changes[1].Patch, strings.Repeat("+line\n", 300)) {
		t.Fatal("partial review lost the full original patch")
	}
	sized, err := Select(model.Request{Changes: []diff.Change{{Path: "large.go", Patch: large}}}, Selection{MaxFileBytes: 1024})
	if err != nil || sized.Coverage[0].Reason != "size_limit" {
		t.Fatalf("size limit: %+v, %v", sized.Coverage, err)
	}
}

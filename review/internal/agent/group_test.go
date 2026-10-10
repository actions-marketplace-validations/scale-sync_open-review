package agent

import (
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestGroupsKeepRelatedFilesAndAllPatches(t *testing.T) {
	req := model.Request{Files: []string{"a/one.go", "b/two.go", "a/three.go"}, Diff: "diff --git a/a/one.go b/a/one.go\n+one\n" + "diff --git a/b/two.go b/b/two.go\n+two\n" + "diff --git a/a/three.go b/a/three.go\n+three\n"}
	groups, err := Groups(req)
	if err != nil || len(groups) != 2 || strings.Join(groups[0].Files, ",") != "a/one.go,a/three.go" || strings.Join(groups[1].Files, ",") != "b/two.go" || strings.Count(groups[0].Patch+groups[1].Patch, "diff --git ") != 3 {
		t.Fatalf("groups = %+v, err = %v", groups, err)
	}
}

func TestGroupsRejectMissingDuplicateAndOverlargePatches(t *testing.T) {
	for _, req := range []model.Request{
		{Files: []string{"a.go", "b.go"}, Diff: "diff --git a/a.go b/a.go\n+a\n"},
		{Files: []string{"a.go", "a.go"}, Diff: "diff --git a/a.go b/a.go\n+a\ndiff --git a/a.go b/a.go\n+a\n"},
		{Files: []string{"a.go", "b.go"}, Diff: "diff --git a/b.go b/b.go\n+b\ndiff --git a/a.go b/a.go\n+a\n"},
		{Files: []string{"a.go"}, Diff: "diff --git a/a.go b/a.go\n" + strings.Repeat("x", MaxGroupBytes)},
	} {
		if _, err := Groups(req); err == nil {
			t.Fatalf("accepted %+v", req.Files)
		}
	}
}

func TestGroupsSplitOversizedFolder(t *testing.T) {
	patch := "diff --git a/a/one.go b/a/one.go\n" + strings.Repeat("a", 23_000) + "\ndiff --git a/a/two.go b/a/two.go\n" + strings.Repeat("b", 23_000)
	groups, err := Groups(model.Request{Files: []string{"a/one.go", "a/two.go"}, Diff: patch})
	if err != nil || len(groups) != 2 || groups[0].Files[0] != "a/one.go" || groups[1].Files[0] != "a/two.go" {
		t.Fatalf("groups = %+v, err = %v", groups, err)
	}
}

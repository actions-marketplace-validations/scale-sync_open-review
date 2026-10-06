package diff

import (
	"strings"
	"testing"
)

func TestParseRenameDeleteAndBinary(t *testing.T) {
	patch := "diff --git a/old.go b/new.go\nrename from old.go\nrename to new.go\n@@ -2,1 +2,2 @@\n-old\n+new\n+next\n" +
		"diff --git a/deleted.go b/deleted.go\ndeleted file mode 100644\n@@ -1 +0,0 @@\n-gone\n" +
		"diff --git a/logo.png b/logo.png\nnew file mode 100644\nGIT binary patch\nliteral 1\nA\n"
	metadata := []byte("R100\x00old.go\x00new.go\x00D\x00deleted.go\x00A\x00logo.png\x00")
	changes, err := Parse(patch, metadata)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 || changes[0].OldPath != "old.go" || changes[0].Path != "new.go" || changes[0].Status != "renamed" {
		t.Fatalf("rename: %+v", changes)
	}
	if len(changes[0].Hunks) != 1 || changes[0].Hunks[0].NewCount != 2 || changes[0].Hunks[0].Lines[2] != "+next" {
		t.Fatalf("hunks: %+v", changes[0].Hunks)
	}
	if changes[1].Status != "deleted" || changes[2].Status != "added" || !changes[2].Binary || len(changes[2].Hunks) != 0 {
		t.Fatalf("statuses: %+v", changes)
	}
}

func TestParseCopyAndUntracked(t *testing.T) {
	patch := "diff --git a/source.go b/copy.go\ncopy from source.go\ncopy to copy.go\n"
	changes, err := Parse(patch, []byte("C100\x00source.go\x00copy.go\x00"))
	if err != nil || len(changes) != 1 || changes[0].Status != "copied" || changes[0].OldPath != "source.go" {
		t.Fatalf("copy: %+v, %v", changes, err)
	}
	untracked := Untracked("new file.go", "diff --git a/new file.go b/new file.go\n@@ -0,0 +1 @@\n+hello\n")
	if !untracked.Untracked || untracked.Path != "new file.go" || len(untracked.Hunks) != 1 {
		t.Fatalf("untracked: %+v", untracked)
	}
	if _, err := Parse(patch, nil); err == nil {
		t.Fatal("expected mismatch error")
	}
	if _, err := Parse(strings.TrimPrefix(patch, "diff --git "), []byte("C100\x00source.go\x00copy.go\x00")); err == nil {
		t.Fatal("expected invalid header error")
	}
}

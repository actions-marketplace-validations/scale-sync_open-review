package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareWholeDirectoryWithoutGit(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "node_modules"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"src/main.go": "package main\nfunc main() {}\n", "src/empty.go": "", "node_modules/ignored.go": "package ignored\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "src", "binary.go"), []byte{0, 1, 2}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, changes, err := Prepare(root, ".")
	if err != nil || len(changes) != 3 {
		t.Fatalf("changes = %+v, %v", changes, err)
	}
	if changes[0].Path != "src/binary.go" || !changes[0].Binary || changes[1].InputIssue != "empty_file" || changes[2].Path != "src/main.go" {
		t.Fatalf("changes = %+v", changes)
	}
	if len(changes[2].Hunks) != 1 || changes[2].Hunks[0].NewCount != 2 || !strings.Contains(changes[2].Patch, "+func main() {}") {
		t.Fatalf("patch = %s", changes[2].Patch)
	}
}

func TestPrepareRejectsEscapeAndSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Prepare(root, outside); err == nil {
		t.Fatal("absolute outside path accepted")
	}
	if _, _, err := Prepare(root, "../outside.go"); err == nil {
		t.Fatal("parent escape accepted")
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Prepare(root, "link.go"); err == nil {
		t.Fatal("symlink target accepted")
	}
	_, changes, err := Prepare(root, ".")
	if err != nil || len(changes) != 1 || changes[0].InputIssue != "unsafe_input" {
		t.Fatalf("changes = %+v, %v", changes, err)
	}
}

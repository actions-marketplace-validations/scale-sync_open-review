package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearningsCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OPEN_REVIEW_HOME", home)
	repo := t.TempDir()

	// 1. List on empty
	var out bytes.Buffer
	if err := runLearnings([]string{"list", "--repo", repo}, &out); err != nil {
		t.Fatalf("list on empty failed: %v", err)
	}
	if !strings.Contains(out.String(), "No learnings") {
		t.Errorf("expected empty message, got: %s", out.String())
	}

	// 2. Add learning
	out.Reset()
	if err := runLearnings([]string{"add", "--repo", repo, "--trigger", "null pointer", "--finding", "Check for nil before calling methods"}, &out); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !strings.Contains(out.String(), "Added learning") {
		t.Errorf("expected added message, got: %s", out.String())
	}

	// 3. List
	out.Reset()
	if err := runLearnings([]string{"list", "--repo", repo, "--json"}, &out); err != nil {
		t.Fatalf("list json failed: %v", err)
	}
	if !strings.Contains(out.String(), "null pointer") {
		t.Errorf("expected finding in list, got: %s", out.String())
	}

	// 4. Export
	exportFile := filepath.Join(home, "export.json")
	out.Reset()
	if err := runLearnings([]string{"export", "--repo", repo, "--output", exportFile}, &out); err != nil {
		t.Fatalf("export failed: %v", err)
	}
	exportedData, err := os.ReadFile(exportFile)
	if err != nil || !strings.Contains(string(exportedData), "null pointer") {
		t.Fatalf("exported data missing or invalid: %v", err)
	}

	// Extract learning ID from export JSON
	type LearningItem struct {
		ID string `json:"id"`
	}
	var items []LearningItem
	if err := json.Unmarshal(exportedData, &items); err != nil || len(items) == 0 {
		t.Fatalf("failed to parse exported items: %v", err)
	}
	id := items[0].ID

	// 5. Delete
	out.Reset()
	if err := runLearnings([]string{"delete", "--repo", repo, id}, &out); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !strings.Contains(out.String(), "Deleted learning") {
		t.Errorf("expected deleted message, got: %s", out.String())
	}

	// 6. Verify list empty after delete
	out.Reset()
	if err := runLearnings([]string{"list", "--repo", repo}, &out); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out.String(), "No learnings") {
		t.Errorf("expected empty message, got: %s", out.String())
	}

	// 7. Import
	out.Reset()
	if err := runLearnings([]string{"import", "--repo", repo, exportFile}, &out); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if !strings.Contains(out.String(), "Imported 1 learnings") {
		t.Errorf("expected imported message, got: %s", out.String())
	}
}

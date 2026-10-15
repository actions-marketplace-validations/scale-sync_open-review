package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestTriagePreservesReviewAndRejectsUnknownFindings(t *testing.T) {
	req := testRequest(t)
	dir := t.TempDir()
	store, err := Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(model.Result{Status: "complete", Findings: []json.RawMessage{json.RawMessage(`{"id":"F1"}`)}}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(dir, store.ID(), "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"fixed", "ignored", "false_positive", "open"} {
		states, err := SetTriage(dir, store.ID(), req.RepoRoot, "F1", state)
		if err != nil {
			t.Fatal(err)
		}
		if state == "open" && len(states) != 0 || state != "open" && states["F1"] != state {
			t.Fatalf("%s: %+v", state, states)
		}
	}
	for _, input := range [][2]string{{"missing", "fixed"}, {"F1", "closed"}} {
		if _, err := SetTriage(dir, store.ID(), req.RepoRoot, input[0], input[1]); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	if _, err := SetTriage(dir, store.ID(), t.TempDir(), "F1", "fixed"); err == nil {
		t.Fatal("foreign repository changed triage")
	}
	current, err := os.ReadFile(filepath.Join(dir, store.ID(), "result.json"))
	if err != nil || string(current) != string(original) {
		t.Fatal("review evidence changed")
	}
}

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func testRequest(t *testing.T) model.Request {
	t.Helper()
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "private-test-token")
	return model.Request{ContractVersion: model.ContractVersion, Mode: "branch", RepoRoot: t.TempDir(), HeadSHA: strings.Repeat("a", 40), Diff: "diff --git a/a.go b/a.go\n+change\n", Files: []string{"a.go"}, Model: model.ModelIdentity{Provider: "openai", Name: "test-model"}}
}

func TestResumeOnlyMatchingInputAndPrivateTrace(t *testing.T) {
	req := testRequest(t)
	dir := t.TempDir()
	first, err := Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	value := struct {
		Findings []string `json:"findings"`
	}{[]string{"one"}}
	if err := first.SaveGroup(0, "group-key", value); err != nil {
		t.Fatal(err)
	}
	if err := first.Finish(model.Result{Status: "partial"}); err != nil {
		t.Fatal(err)
	}
	second, err := Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	var loaded struct {
		Findings []string `json:"findings"`
	}
	if ok, err := second.LoadGroup(0, "group-key", &loaded); err != nil || !ok || len(loaded.Findings) != 1 {
		t.Fatalf("loaded = %+v, ok = %t, err = %v", loaded, ok, err)
	}
	if ok, err := second.LoadGroup(0, "other-group", &loaded); err != nil || ok {
		t.Fatalf("wrong group reused: %t, %v", ok, err)
	}
	changed := req
	changed.Diff += "another change"
	third, err := Open(dir, changed)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := third.LoadGroup(0, "group-key", &loaded); err != nil || ok {
		t.Fatalf("changed diff reused: %t, %v", ok, err)
	}
	workspace := req
	workspace.Mode = "workspace"
	fourth, err := Open(dir, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := fourth.LoadGroup(0, "group-key", &loaded); err != nil || ok {
		t.Fatalf("workspace reused: %t, %v", ok, err)
	}
	if info, err := os.Stat(filepath.Join(dir, first.ID(), "events.jsonl")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("event permissions: %v, %v", info, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, first.ID(), "meta.json"))
	if err != nil || strings.Contains(string(data), "private-test-token") || strings.Contains(string(data), req.Diff) {
		t.Fatalf("sensitive input persisted: %v", err)
	}
}

func TestSessionReadRejectsMalformedAndForeignRecords(t *testing.T) {
	req := testRequest(t)
	dir := t.TempDir()
	store, err := Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(model.Result{Status: "complete", Findings: []json.RawMessage{json.RawMessage(`{"id":"one"}`)}}); err != nil {
		t.Fatal(err)
	}
	items, err := List(dir, req.RepoRoot)
	if err != nil || len(items) != 1 {
		t.Fatalf("list = %+v, %v", items, err)
	}
	if _, err := Read(dir, store.ID(), t.TempDir()); err == nil {
		t.Fatal("foreign repository could read session")
	}
	if err := Delete(dir, "../"+store.ID(), req.RepoRoot); err == nil {
		t.Fatal("path traversal accepted")
	}
	record, err := Read(dir, store.ID(), req.RepoRoot)
	if err != nil || record.Summary.Status != "complete" || len(record.Events) != 2 {
		t.Fatalf("record = %+v, %v", record, err)
	}
	if err := os.WriteFile(filepath.Join(dir, store.ID(), "events.jsonl"), []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, store.ID(), req.RepoRoot); err == nil {
		t.Fatal("malformed event accepted")
	}
	if err := Delete(dir, store.ID(), req.RepoRoot); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedGroupCacheCannotBeReused(t *testing.T) {
	req := testRequest(t)
	dir := t.TempDir()
	first, err := Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, first.ID(), "group-0.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if _, err := second.LoadGroup(0, "group-key", &result); err == nil {
		t.Fatal("malformed cache accepted")
	}
}

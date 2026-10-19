package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestACPWorkspaceReviewAndMalformedMessage(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[]}"}}]}`))
	}))
	defer model.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", model.URL)
	reader, writer := io.Pipe()
	outReader, outWriter := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- serveACP(context.Background(), root, reader, outWriter); outWriter.Close() }()
	request := json.NewEncoder(writer)
	response := json.NewDecoder(outReader)
	if _, err := io.WriteString(writer, "not json\n"); err != nil {
		t.Fatal(err)
	}
	var msg map[string]json.RawMessage
	if err := response.Decode(&msg); err != nil || string(msg["id"]) != "null" {
		t.Fatalf("malformed message: %v %v", msg, err)
	}
	if err := request.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": 1}}); err != nil {
		t.Fatal(err)
	}
	msg = nil
	if err := response.Decode(&msg); err != nil {
		t.Fatal(err)
	}
	if string(msg["id"]) != "1" {
		t.Fatalf("initialize: %v", msg)
	}
	if err := request.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": map[string]any{"cwd": root, "mcpServers": []any{}}}); err != nil {
		t.Fatal(err)
	}
	msg = nil
	if err := response.Decode(&msg); err != nil {
		t.Fatal(err)
	}
	var session struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(msg["result"], &session); err != nil || session.SessionID == "" {
		t.Fatalf("session: %v %v", msg, err)
	}
	if err := request.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "session/prompt", "params": map[string]any{"sessionId": session.SessionID, "prompt": []any{map[string]any{"type": "text", "text": "review"}}}}); err != nil {
		t.Fatal(err)
	}
	finished := make(chan map[string]json.RawMessage, 1)
	go func() {
		for {
			var item map[string]json.RawMessage
			if response.Decode(&item) != nil {
				return
			}
			if string(item["id"]) == "3" {
				finished <- item
				return
			}
		}
	}()
	select {
	case result := <-finished:
		var payload struct {
			StopReason string `json:"stopReason"`
		}
		if json.Unmarshal(result["result"], &payload) != nil || payload.StopReason != "end_turn" {
			t.Fatalf("prompt: %v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ACP prompt timed out")
	}
	writer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ACP server did not stop")
	}
}

func TestACPCancelStopsPendingReview(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer model.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", model.URL)
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- serveACP(context.Background(), root, inReader, outWriter); outWriter.Close() }()
	send := json.NewEncoder(inWriter)
	receive := json.NewDecoder(outReader)
	read := func() map[string]json.RawMessage {
		t.Helper()
		var message map[string]json.RawMessage
		if err := receive.Decode(&message); err != nil {
			t.Fatal(err)
		}
		return message
	}
	if err := send.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": 1}}); err != nil {
		t.Fatal(err)
	}
	read()
	if err := send.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": map[string]any{"cwd": root, "mcpServers": []any{}}}); err != nil {
		t.Fatal(err)
	}
	var session struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(read()["result"], &session); err != nil {
		t.Fatal(err)
	}
	if err := send.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "session/prompt", "params": map[string]any{"sessionId": session.SessionID, "prompt": []any{map[string]any{"type": "text", "text": "review"}}}}); err != nil {
		t.Fatal(err)
	}
	if update := read(); string(update["method"]) != `"session/update"` {
		t.Fatalf("missing progress: %v", update)
	}
	if err := send.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/cancel", "params": map[string]any{"sessionId": session.SessionID}}); err != nil {
		t.Fatal(err)
	}
	finished := make(chan map[string]json.RawMessage, 1)
	go func() {
		for {
			var item map[string]json.RawMessage
			if receive.Decode(&item) != nil {
				return
			}
			if string(item["id"]) == "3" {
				finished <- item
				return
			}
		}
	}()
	select {
	case answer := <-finished:
		if !strings.Contains(string(answer["result"]), `"cancelled"`) {
			t.Fatalf("cancellation: %v", answer)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation timed out")
	}
	inWriter.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

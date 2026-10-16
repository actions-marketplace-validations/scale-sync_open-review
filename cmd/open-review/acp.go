package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/scale-sync/open-review/review"
)

type acpMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type acpWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func (w *acpWriter) send(value any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encoder.Encode(value)
}
func (w *acpWriter) result(id json.RawMessage, value any) {
	_ = w.send(map[string]any{"jsonrpc": "2.0", "id": id, "result": value})
}
func (w *acpWriter) failure(id json.RawMessage, code int, message string) {
	_ = w.send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}
func (w *acpWriter) update(session string, value any) {
	_ = w.send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": session, "update": value}})
}

func runACP(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 || args[0] != "serve" {
		return errors.New("usage: open-review acp serve --repo PATH")
	}
	flags := flag.NewFlagSet("acp serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "repository available to the editor")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *repo == "" {
		return errors.New("acp serve requires --repo and no extra arguments")
	}
	root, err := filepath.EvalSymlinks(*repo)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	return serveACP(ctx, root, input, output)
}

func serveACP(ctx context.Context, root string, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	writer := &acpWriter{encoder: json.NewEncoder(output)}
	initialized, session := "", ""
	var active context.CancelFunc
	var mu sync.Mutex
	var work sync.WaitGroup
	defer func() {
		mu.Lock()
		if active != nil {
			active()
		}
		mu.Unlock()
		work.Wait()
	}()
	for scanner.Scan() {
		var msg acpMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil || msg.JSONRPC != "2.0" || msg.Method == "" {
			writer.failure(nil, -32700, "invalid JSON-RPC request")
			continue
		}
		if len(msg.ID) == 0 && msg.Method != "session/cancel" {
			continue
		}
		switch msg.Method {
		case "initialize":
			var params struct {
				ProtocolVersion int `json:"protocolVersion"`
			}
			if json.Unmarshal(msg.Params, &params) != nil || params.ProtocolVersion < 1 {
				writer.failure(msg.ID, -32602, "unsupported protocol version")
				continue
			}
			initialized = "yes"
			writer.result(msg.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{}, "agentInfo": map[string]any{"name": "open-review", "title": "open-review", "version": "v1.7.0"}, "authMethods": []any{}})
		case "session/new":
			if initialized == "" {
				writer.failure(msg.ID, -32600, "initialize first")
				continue
			}
			var params struct {
				CWD        string            `json:"cwd"`
				MCPServers []json.RawMessage `json:"mcpServers"`
			}
			if json.Unmarshal(msg.Params, &params) != nil || len(params.MCPServers) != 0 {
				writer.failure(msg.ID, -32602, "invalid session options")
				continue
			}
			resolved, err := filepath.EvalSymlinks(params.CWD)
			if err != nil || resolved != root {
				writer.failure(msg.ID, -32602, "cwd must match configured repository")
				continue
			}
			var token [12]byte
			if _, err := rand.Read(token[:]); err != nil {
				writer.failure(msg.ID, -32603, "session creation failed")
				continue
			}
			session = hex.EncodeToString(token[:])
			writer.result(msg.ID, map[string]any{"sessionId": session})
		case "session/prompt":
			var params struct {
				SessionID string `json:"sessionId"`
				Prompt    []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"prompt"`
			}
			if json.Unmarshal(msg.Params, &params) != nil || params.SessionID != session || session == "" || len(params.Prompt) != 1 || params.Prompt[0].Type != "text" || len(params.Prompt[0].Text) > 2000 {
				writer.failure(msg.ID, -32602, "invalid review prompt")
				continue
			}
			mu.Lock()
			if active != nil {
				mu.Unlock()
				writer.failure(msg.ID, -32600, "review already running")
				continue
			}
			runCtx, cancel := context.WithCancel(ctx)
			active = cancel
			work.Add(1)
			mu.Unlock()
			go func(id json.RawMessage, sessionID, instruction string) {
				defer work.Done()
				defer func() { mu.Lock(); active = nil; mu.Unlock(); cancel() }()
				writer.update(sessionID, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "review", "title": "Reviewing repository", "kind": "read", "status": "in_progress"})
				mode, scanPath := "workspace", ""
				if strings.HasPrefix(instruction, "scan: ") {
					mode = "scan"
					scanPath = strings.TrimSpace(strings.TrimPrefix(instruction, "scan: "))
					instruction = ""
				}
				prepared, err := review.Prepare(runCtx, review.Options{Repo: root, Mode: mode, ScanPath: scanPath, Instructions: instruction})
				var result review.Result
				if err == nil {
					result, err = review.Run(runCtx, prepared)
				}
				if runCtx.Err() != nil {
					writer.update(sessionID, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "review", "status": "failed"})
					writer.result(id, map[string]any{"stopReason": "cancelled"})
					return
				}
				if err != nil {
					writer.update(sessionID, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "review", "status": "failed"})
					writer.failure(id, -32603, err.Error())
					return
				}
				writer.update(sessionID, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "review", "status": "completed"})
				for _, raw := range result.Findings {
					var finding struct {
						ID, File, Title string
						StartLine       int
					}
					if json.Unmarshal(raw, &finding) != nil || finding.ID == "" || finding.File == "" {
						continue
					}
					path := filepath.Join(root, filepath.FromSlash(finding.File))
					writer.update(sessionID, map[string]any{"sessionUpdate": "tool_call", "toolCallId": finding.ID, "title": finding.Title, "kind": "read", "status": "completed", "locations": []any{map[string]any{"path": path, "line": finding.StartLine}}})
				}
				data, _ := json.Marshal(result)
				writer.update(sessionID, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": string(data)}})
				writer.result(id, map[string]any{"stopReason": "end_turn"})
			}(append(json.RawMessage(nil), msg.ID...), session, params.Prompt[0].Text)
		case "session/cancel":
			var params struct {
				SessionID string `json:"sessionId"`
			}
			if json.Unmarshal(msg.Params, &params) == nil && params.SessionID == session {
				mu.Lock()
				if active != nil {
					active()
				}
				mu.Unlock()
			}
		default:
			writer.failure(msg.ID, -32601, fmt.Sprintf("unsupported method %s", msg.Method))
		}
	}
	return scanner.Err()
}

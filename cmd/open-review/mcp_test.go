package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPPreviewMatchesCLI(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	server := newMCPServer(root)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	protocol, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "review", Arguments: map[string]any{"mode": "workspace", "preview": true}})
	if err != nil || protocol.IsError || len(protocol.Content) != 1 {
		t.Fatalf("MCP result: %v, %+v", err, protocol)
	}
	var cli bytes.Buffer
	if err := run(ctx, []string{"review", "--repo", root, "--preview", "--output", "json"}, &cli); err != nil {
		t.Fatal(err)
	}
	var fromCLI, fromProtocol struct {
		Coverage []struct{ Path, State string } `json:"coverage"`
	}
	if err := json.Unmarshal(cli.Bytes(), &fromCLI); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(protocol.Content[0].(*mcp.TextContent).Text), &fromProtocol); err != nil {
		t.Fatal(err)
	}
	if len(fromCLI.Coverage) != 1 || len(fromProtocol.Coverage) != 1 || fromCLI.Coverage[0] != fromProtocol.Coverage[0] {
		t.Fatalf("CLI=%+v MCP=%+v", fromCLI, fromProtocol)
	}
	denied, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "review", Arguments: map[string]any{"mode": "outside", "preview": true}})
	if err == nil && !denied.IsError {
		t.Fatal("invalid mode was accepted")
	}
}

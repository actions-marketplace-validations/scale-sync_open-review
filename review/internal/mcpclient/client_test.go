package mcpclient

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConfigRejectsUnlistedToolArgumentsAndRemoteSecrets(t *testing.T) {
	for _, config := range []string{
		`{"servers":[{"name":"docs","url":"https://example.com/mcp","allow":["lookup"],"arguments":{"upload":{"source":"private"}}}]}`,
		`{"servers":[{"name":"docs","url":"http://example.com/mcp","allow":["lookup"]}]}`,
		`{"servers":[{"name":"docs","url":"https://example.com/mcp","allow":["lookup"],"env":["SECRET"]}]}`,
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted unsafe config: %s", config)
		}
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"servers":[{"name":"docs","url":"https://example.com/mcp","allow":["lookup"],"arguments":{"lookup":{"topic":"public"}}}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(path)
	if err != nil || config.Servers[0].Arguments["lookup"]["topic"] != "public" {
		t.Fatalf("static arguments: %+v, %v", config, err)
	}
	if strings.Contains(config.Servers[0].URL, "@") {
		t.Fatal("URL credentials accepted")
	}
}

func TestCallUsesOnlyConfiguredArguments(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: "lookup", InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
			return nil, err
		}
		data, _ := json.Marshal(args)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil
	})
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "fixture-client", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{sessions: []*mcp.ClientSession{clientSession}, allowed: map[string]allowed{"mcp/docs/lookup": {session: clientSession, tool: "lookup", timeout: time.Second, arguments: map[string]any{"topic": "public"}}}}
	defer client.Close()
	output, err := client.Call(ctx, "mcp/docs/lookup")
	if err != nil || !strings.Contains(output, `"topic":"public"`) {
		t.Fatalf("result %q: %v", output, err)
	}
	if _, err := client.Call(ctx, "mcp/docs/upload"); err == nil {
		t.Fatal("unlisted tool executed")
	}
}

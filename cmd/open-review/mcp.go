package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scale-sync/open-review/review"
)

type mcpReviewInput struct {
	Mode         string `json:"mode"`
	Base         string `json:"base,omitempty"`
	Head         string `json:"head,omitempty"`
	Commit       string `json:"commit,omitempty"`
	ScanPath     string `json:"scanPath,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	Preview      bool   `json:"preview,omitempty"`
}

func runMCP(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "serve" {
		return errors.New("usage: open-review mcp serve --repo PATH")
	}
	flags := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "repository available to the protocol client")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *repo == "" {
		return errors.New("mcp serve requires --repo and no extra arguments")
	}
	root, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	return newMCPServer(root).Run(ctx, &mcp.StdioTransport{})
}

func newMCPServer(root string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "open-review", Version: "v1.7.0"}, nil)
	server.AddTool(&mcp.Tool{Name: "review", Description: "Review changes in the configured repository", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"mode": map[string]any{"type": "string", "enum": []string{"workspace", "branch", "commit", "scan"}}, "base": map[string]any{"type": "string"}, "head": map[string]any{"type": "string"}, "commit": map[string]any{"type": "string"}, "scanPath": map[string]any{"type": "string"}, "instructions": map[string]any{"type": "string"}, "preview": map[string]any{"type": "boolean"}}, "additionalProperties": false}}, func(callCtx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if len(req.Params.Arguments) > 4096 {
			return nil, errors.New("review arguments exceed 4 KB")
		}
		decoder := json.NewDecoder(strings.NewReader(string(req.Params.Arguments)))
		decoder.DisallowUnknownFields()
		var input mcpReviewInput
		if err := decoder.Decode(&input); err != nil {
			return nil, err
		}
		if decoder.Decode(new(any)) != io.EOF {
			return nil, errors.New("extra review arguments")
		}
		if len(input.Instructions) > 2000 {
			return nil, errors.New("instructions exceed 2 KB")
		}
		if input.Mode == "" {
			input.Mode = "workspace"
		}
		if input.Mode != "workspace" && input.Mode != "branch" && input.Mode != "commit" && input.Mode != "scan" {
			return nil, errors.New("unsupported review mode")
		}
		prepared, err := review.Prepare(callCtx, review.Options{Repo: root, Mode: input.Mode, Base: input.Base, Head: input.Head, Commit: input.Commit, ScanPath: input.ScanPath, Instructions: input.Instructions})
		if err != nil {
			return nil, err
		}
		var payload any
		if input.Preview {
			payload = map[string]any{"mode": prepared.Mode, "coverage": prepared.Coverage, "rules": prepared.RuleDiagnostics}
		} else {
			result, runErr := review.Run(callCtx, prepared)
			if runErr != nil {
				return nil, runErr
			}
			payload = result
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil
	})
	return server
}

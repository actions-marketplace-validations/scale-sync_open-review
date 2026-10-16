package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServerConfig struct {
	Name           string                    `json:"name"`
	Command        []string                  `json:"command,omitempty"`
	URL            string                    `json:"url,omitempty"`
	Allow          []string                  `json:"allow"`
	Arguments      map[string]map[string]any `json:"arguments,omitempty"`
	Env            []string                  `json:"env,omitempty"`
	TokenEnv       string                    `json:"tokenEnv,omitempty"`
	TimeoutSeconds int                       `json:"timeoutSeconds,omitempty"`
}
type Config struct {
	Servers []ServerConfig `json:"servers"`
}
type Tool struct{ Name string }
type allowed struct {
	session   *mcp.ClientSession
	tool      string
	timeout   time.Duration
	arguments map[string]any
}
type Client struct {
	sessions []*mcp.ClientSession
	tools    []Tool
	allowed  map[string]allowed
}
type contextKey struct{}

var safeName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,39}$`)
var safeEnv = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil {
		return Config{}, err
	}
	if len(raw) > 64<<10 {
		return Config{}, errors.New("MCP config exceeds 64 KB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Config{}, errors.New("MCP config contains extra JSON")
	}
	if len(config.Servers) == 0 || len(config.Servers) > 4 {
		return Config{}, errors.New("MCP config needs 1 to 4 servers")
	}
	seen := map[string]bool{}
	for _, server := range config.Servers {
		if !safeName.MatchString(server.Name) || seen[server.Name] || (len(server.Command) == 0) == (server.URL == "") || len(server.Allow) == 0 || len(server.Allow) > 20 || server.TimeoutSeconds < 0 || server.TimeoutSeconds > 60 || (len(server.Command) > 0 && server.Command[0] == "") {
			return Config{}, errors.New("invalid MCP server configuration")
		}
		seen[server.Name] = true
		for _, name := range server.Allow {
			if !safeName.MatchString(name) {
				return Config{}, errors.New("invalid MCP tool allowlist")
			}
		}
		for name, args := range server.Arguments {
			permitted := false
			for _, allowed := range server.Allow {
				if allowed == name {
					permitted = true
					break
				}
			}
			data, err := json.Marshal(args)
			if !permitted || err != nil || len(data) > 4096 {
				return Config{}, errors.New("invalid static MCP arguments")
			}
		}
		for _, name := range server.Env {
			if !safeEnv.MatchString(name) {
				return Config{}, errors.New("invalid MCP environment name")
			}
		}
		if server.TokenEnv != "" && !safeEnv.MatchString(server.TokenEnv) {
			return Config{}, errors.New("invalid MCP token environment name")
		}
		if server.URL != "" {
			parsed, err := url.Parse(server.URL)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || len(server.Env) != 0 {
				return Config{}, errors.New("remote MCP server needs a plain HTTPS URL")
			}
		} else if server.TokenEnv != "" {
			return Config{}, errors.New("local MCP server cannot use tokenEnv")
		}
	}
	return config, nil
}

func Connect(ctx context.Context, config Config) (*Client, error) {
	connected := &Client{allowed: map[string]allowed{}}
	for _, server := range config.Servers {
		var transport mcp.Transport
		if server.URL != "" {
			token := ""
			if server.TokenEnv != "" {
				token = os.Getenv(server.TokenEnv)
				if token == "" {
					connected.Close()
					return nil, errors.New("MCP token environment variable is empty")
				}
			}
			transport = &mcp.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: bearerTransport{base: http.DefaultTransport, token: token}}}
		} else {
			command := exec.Command(server.Command[0], server.Command[1:]...)
			command.Env = []string{"PATH=" + os.Getenv("PATH")}
			for _, name := range server.Env {
				command.Env = append(command.Env, name+"="+os.Getenv(name))
			}
			transport = &mcp.CommandTransport{Command: command}
		}
		deadline := time.Duration(server.TimeoutSeconds) * time.Second
		if deadline == 0 {
			deadline = 10 * time.Second
		}
		connectCtx, cancel := context.WithTimeout(ctx, deadline)
		session, err := mcp.NewClient(&mcp.Implementation{Name: "open-review", Version: "v1.7.0"}, nil).Connect(connectCtx, transport, nil)
		cancel()
		if err != nil {
			connected.Close()
			return nil, fmt.Errorf("MCP server %s: %w", server.Name, err)
		}
		connected.sessions = append(connected.sessions, session)
		available := map[string]*mcp.Tool{}
		cursor := ""
		for page := 0; page < 5; page++ {
			listCtx, stop := context.WithTimeout(ctx, deadline)
			listed, err := session.ListTools(listCtx, &mcp.ListToolsParams{Cursor: cursor})
			stop()
			if err != nil {
				connected.Close()
				return nil, err
			}
			for _, tool := range listed.Tools {
				available[tool.Name] = tool
			}
			cursor = listed.NextCursor
			if cursor == "" {
				break
			}
		}
		if cursor != "" {
			connected.Close()
			return nil, errors.New("MCP tool list exceeds five pages")
		}
		for _, name := range server.Allow {
			tool := available[name]
			if tool == nil {
				connected.Close()
				return nil, fmt.Errorf("allowed MCP tool %s/%s is unavailable", server.Name, name)
			}
			alias := "mcp/" + server.Name + "/" + name
			connected.tools = append(connected.tools, Tool{Name: alias})
			connected.allowed[alias] = allowed{session: session, tool: name, timeout: deadline, arguments: server.Arguments[name]}
		}
	}
	return connected, nil
}

func (c *Client) Close() {
	for _, session := range c.sessions {
		_ = session.Close()
	}
}
func (c *Client) Tools() []Tool { return append([]Tool(nil), c.tools...) }
func WithClient(ctx context.Context, client *Client) context.Context {
	return context.WithValue(ctx, contextKey{}, client)
}
func FromContext(ctx context.Context) *Client {
	client, _ := ctx.Value(contextKey{}).(*Client)
	return client
}

func (c *Client) Call(ctx context.Context, name string) (string, error) {
	selected, ok := c.allowed[name]
	if !ok {
		return "", errors.New("MCP tool is not allowed")
	}
	callCtx, cancel := context.WithTimeout(ctx, selected.timeout)
	defer cancel()
	result, err := selected.session.CallTool(callCtx, &mcp.CallToolParams{Name: selected.tool, Arguments: selected.arguments})
	if err != nil {
		return "", err
	}
	if result.IsError {
		return "", errors.New("MCP tool reported an error")
	}
	var output strings.Builder
	for _, content := range result.Content {
		if item, ok := content.(*mcp.TextContent); ok {
			remaining := 8000 - output.Len()
			if remaining <= 0 {
				break
			}
			if len(item.Text) > remaining {
				output.WriteString(item.Text[:remaining])
				break
			}
			output.WriteString(item.Text)
			if output.Len() < 8000 {
				output.WriteByte('\n')
			}
		}
	}
	return output.String(), nil
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	if t.token != "" {
		copy.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(copy)
}

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/yinhe/devclaw/internal/provider"
)

// ServerConfig defines an MCP server connection
type ServerConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env,omitempty"`
}

// LoadServers reads MCP server configs from .drone/mcp.json
func LoadServers(root string) []ServerConfig {
	paths := []string{
		root + "/.drone/mcp.json",
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		paths = append(paths, home+"/.drone/mcp.json")
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg struct {
			Servers []ServerConfig `json:"servers"`
		}
		if json.Unmarshal(data, &cfg) == nil && len(cfg.Servers) > 0 {
			return cfg.Servers
		}
	}
	return nil
}

// Client manages a stdio MCP server connection
type Client struct {
	name   string
	cmd    *exec.Cmd
	stdin  *json.Encoder
	stdout *bufio.Scanner
	mu     sync.Mutex
	nextID atomic.Int64
	tools  []ToolInfo
}

// ToolInfo describes a tool from MCP server
type ToolInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// jsonrpc messages
type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewClient starts an MCP server process and connects via stdio
func NewClient(cfg ServerConfig) (*Client, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = append(os.Environ(), cfg.Env...)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdin: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp start %s: %w", cfg.Name, err)
	}

	c := &Client{
		name:   cfg.Name,
		cmd:    cmd,
		stdin:  json.NewEncoder(stdinPipe),
		stdout: bufio.NewScanner(stdoutPipe),
	}

	// Initialize
	if err := c.initialize(); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("mcp init %s: %w", cfg.Name, err)
	}

	// List tools
	if err := c.listTools(); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("mcp list_tools %s: %w", cfg.Name, err)
	}

	return c, nil
}

func (c *Client) call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := c.stdin.Encode(req); err != nil {
		return nil, err
	}

	for c.stdout.Scan() {
		var resp rpcResponse
		if json.Unmarshal(c.stdout.Bytes(), &resp) == nil && resp.ID == id {
			if resp.Error != nil {
				return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return resp.Result, nil
		}
	}
	return nil, fmt.Errorf("mcp %s: connection closed", c.name)
}

func (c *Client) initialize() error {
	_, err := c.call("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "drone", "version": "1.0"},
	})
	return err
}

func (c *Client) listTools() error {
	result, err := c.call("tools/list", nil)
	if err != nil {
		return err
	}
	var resp struct {
		Tools []ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return err
	}
	c.tools = resp.Tools
	return nil
}

// Name returns the server name
func (c *Client) Name() string { return c.name }

// Tools returns discovered tools
func (c *Client) Tools() []ToolInfo { return c.tools }

// CallTool invokes a tool on the MCP server
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	result, err := c.call("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return string(result), nil
	}
	var texts []string
	for _, c := range resp.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	return fmt.Sprintf("%s", texts), nil
}

// ToDefinitions converts MCP tools to provider.ToolDefinition for LLM
func (c *Client) ToDefinitions() []provider.ToolDefinition {
	var defs []provider.ToolDefinition
	for _, t := range c.tools {
		defs = append(defs, provider.ToolDefinition{
			Type: "function",
			Function: provider.FunctionSchema{
				Name:        c.name + "__" + t.Name,
				Description: fmt.Sprintf("[MCP:%s] %s", c.name, t.Description),
				Parameters:  t.InputSchema,
			},
		})
	}
	return defs
}

// Close kills the MCP server process
func (c *Client) Close() {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
}

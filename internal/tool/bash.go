package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

const maxOutput = 64 * 1024 // 64KB

// BashTool executes shell commands
type BashTool struct {
	workspace string
}

func NewBashTool(ws string) *BashTool { return &BashTool{workspace: ws} }

func (t *BashTool) Name() string        { return "Bash" }
func (t *BashTool) Description() string { return "Execute a shell command and return stdout+stderr." }
func (t *BashTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]string{"type": "string", "description": "Shell command to execute"},
			"cwd":     map[string]string{"type": "string", "description": "Working directory"},
			"timeout": map[string]string{"type": "integer", "description": "Timeout in seconds (default 30)"},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	timeout := time.Duration(p.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", p.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", p.Command)
	}

	cwd := p.Cwd
	if cwd == "" {
		cwd = t.workspace
	}
	cmd.Dir = cwd
	cmd.Env = append(cmd.Environ(), "PAGER=cat")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var result string
	if stdout.Len() > 0 {
		result = stdout.String()
	}
	if stderr.Len() > 0 {
		if result != "" {
			result += "\n"
		}
		result += stderr.String()
	}
	if result == "" && err != nil {
		result = err.Error()
	}

	if len(result) > maxOutput {
		result = result[:maxOutput] + "\n...(truncated)"
	}
	return result, nil
}

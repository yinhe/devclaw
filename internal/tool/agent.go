package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SubTaskFunc is the function signature for executing a subtask.
// Injected by the runtime to avoid circular imports.
type SubTaskFunc func(ctx context.Context, task string, role string) (string, error)

// AgentTool allows the LLM to spawn sub-Drone instances for subtasks
type AgentTool struct {
	runSubTask SubTaskFunc
}

func NewAgentTool(fn SubTaskFunc) *AgentTool {
	return &AgentTool{runSubTask: fn}
}

func (t *AgentTool) Name() string { return "Agent" }
func (t *AgentTool) Description() string {
	return "Spawn a sub-agent to execute a subtask autonomously. Use for independent work that can run in isolation."
}
func (t *AgentTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]string{"type": "string", "description": "Subtask description for the sub-agent"},
			"role": map[string]string{"type": "string", "description": "Role for the sub-agent (dev/test/ops). Default: dev"},
		},
		"required": []string{"task"},
	}
}

func (t *AgentTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Task string `json:"task"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if p.Role == "" {
		p.Role = "dev"
	}
	if t.runSubTask == nil {
		return "Error: Agent tool not configured (no subtask runner)", nil
	}

	result, err := t.runSubTask(ctx, p.Task, p.Role)
	if err != nil {
		return fmt.Sprintf("Sub-agent error: %v", err), nil
	}
	if len(result) > 4000 {
		result = result[:4000] + "\n...(truncated)"
	}
	return result, nil
}

// ParallelTool runs multiple subtasks concurrently
type ParallelTool struct {
	runSubTask SubTaskFunc
}

func NewParallelTool(fn SubTaskFunc) *ParallelTool {
	return &ParallelTool{runSubTask: fn}
}

func (t *ParallelTool) Name() string { return "Parallel" }
func (t *ParallelTool) Description() string {
	return "Run multiple subtasks in parallel. Each gets its own sub-agent. Returns all results when complete."
}
func (t *ParallelTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tasks": map[string]interface{}{
				"type":        "array",
				"description": "Array of subtask descriptions",
				"items":       map[string]string{"type": "string"},
			},
			"role": map[string]string{"type": "string", "description": "Role for all sub-agents. Default: dev"},
		},
		"required": []string{"tasks"},
	}
}

func (t *ParallelTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Tasks []string `json:"tasks"`
		Role  string   `json:"role"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if p.Role == "" {
		p.Role = "dev"
	}
	if t.runSubTask == nil {
		return "Error: Parallel tool not configured", nil
	}
	if len(p.Tasks) == 0 {
		return "Error: no tasks provided", nil
	}
	if len(p.Tasks) > 5 {
		return "Error: max 5 parallel tasks", nil
	}

	type taskResult struct {
		Index   int    `json:"index"`
		Task    string `json:"task"`
		Result  string `json:"result"`
		Success bool   `json:"success"`
	}

	results := make([]taskResult, len(p.Tasks))
	var wg sync.WaitGroup
	start := time.Now()

	runner := t.runSubTask
	for i, task := range p.Tasks {
		wg.Add(1)
		go func(idx int, subtask string) {
			defer wg.Done()
			res, err := runner(ctx, subtask, p.Role)
			if err != nil {
				results[idx] = taskResult{Index: idx, Task: subtask, Result: err.Error(), Success: false}
			} else {
				if len(res) > 2000 {
					res = res[:2000] + "..."
				}
				results[idx] = taskResult{Index: idx, Task: subtask, Result: res, Success: true}
			}
		}(i, task)
	}

	wg.Wait()

	succeeded := 0
	for _, r := range results {
		if r.Success {
			succeeded++
		}
	}

	summary := map[string]interface{}{
		"total":     len(p.Tasks),
		"succeeded": succeeded,
		"duration":  time.Since(start).Round(time.Second).String(),
		"results":   results,
	}

	out, _ := json.MarshalIndent(summary, "", "  ")
	return string(out), nil
}

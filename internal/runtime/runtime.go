package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yinhe/devclaw/internal/config"
	"github.com/yinhe/devclaw/internal/provider"
	"github.com/yinhe/devclaw/internal/role"
	"github.com/yinhe/devclaw/internal/tool"
)

// DroneRuntime is the task-driven agent execution loop.
// Unlike Zerg's interactive REPL, Drone runs autonomously until task completion.
type DroneRuntime struct {
	provider provider.Provider
	tools    *tool.Registry
	config   config.Config
	role     role.Profile
	messages []provider.ChatMessage

	// Callbacks for progress reporting
	OnText      func(text string)
	OnToolStart func(name, args string)
	OnToolEnd   func(name, result string)
	OnCompress  func(stats string)

	// Trajectory logging (optional)
	Trajectory *TrajectoryLogger
}

// TaskResult is the outcome of a Drone task execution
type TaskResult struct {
	Success   bool
	Summary   string
	ToolCalls int
	Turns     int
	Usage     provider.TokenUsage
	Duration  time.Duration
}

// New creates a new DroneRuntime
func New(p provider.Provider, tools *tool.Registry, cfg config.Config) *DroneRuntime {
	return &DroneRuntime{
		provider: p,
		tools:    tools,
		config:   cfg,
		role:     role.Get(cfg.Role),
		messages: []provider.ChatMessage{},
	}
}

// RunSubTask creates a child DroneRuntime and executes a subtask.
// Used by AgentTool and ParallelTool to spawn sub-Drones.
func (d *DroneRuntime) RunSubTask(ctx context.Context, task string, roleName string) (string, error) {
	childCfg := d.config
	childCfg.Task = task
	childCfg.Role = roleName
	childCfg.MaxTurns = 20 // Sub-tasks get fewer turns
	childCfg.Permission = role.Get(roleName).Permission

	child := New(d.provider, d.tools, childCfg)
	result, err := child.Run(ctx)
	if err != nil {
		return "", err
	}
	return result.Summary, nil
}

// Run executes the task autonomously until completion or max turns
func (d *DroneRuntime) Run(ctx context.Context) (*TaskResult, error) {
	start := time.Now()
	totalUsage := provider.TokenUsage{}
	totalToolCalls := 0

	// Build system prompt
	sysPrompt := d.buildSystemPrompt()

	// Inject task as first user message
	d.messages = append(d.messages, provider.ChatMessage{
		Role:    "user",
		Content: d.config.Task,
	})

	maxTurns := d.config.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}

	for turn := 1; turn <= maxTurns; turn++ {
		// Check context pressure and compress if needed
		if ShouldCompress(d.messages, 128000) {
			compressed, stats, err := CompressMessages(ctx, d.provider, d.config.Model, d.messages, 8)
			if err == nil && len(compressed) < len(d.messages) {
				d.messages = compressed
				if d.OnCompress != nil {
					d.OnCompress(stats)
				}
				if d.Trajectory != nil {
					d.Trajectory.LogCompress(stats)
				}
				log.Printf("[drone] %s", stats)
			}
		}

		// Build request
		toolDefs := d.tools.Definitions()
		req := &provider.ChatRequest{
			Model:       d.config.Model,
			Messages:    d.withSystem(sysPrompt),
			Tools:       toolDefs,
			Temperature: 0.1, // Low temperature for deterministic coding
		}
		if len(toolDefs) > 0 {
			req.ToolChoice = "auto"
		}

		// Stream from LLM
		ch, err := d.provider.Chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("provider error on turn %d: %w", turn, err)
		}

		var responseText string
		var toolCalls []provider.ToolCall

		for chunk := range ch {
			if chunk.Error != "" {
				return &TaskResult{
					Summary:  "Error: " + chunk.Error,
					Turns:    turn,
					Usage:    totalUsage,
					Duration: time.Since(start),
				}, nil
			}
			if chunk.Content != "" {
				responseText += chunk.Content
				if d.OnText != nil {
					d.OnText(chunk.Content)
				}
			}
			if chunk.Tool != nil {
				toolCalls = append(toolCalls, *chunk.Tool)
			}
			if chunk.Usage != nil {
				totalUsage.PromptTokens += chunk.Usage.PromptTokens
				totalUsage.CompletionTokens += chunk.Usage.CompletionTokens
				totalUsage.TotalTokens += chunk.Usage.TotalTokens
			}
		}

		// No tool calls → try fallback parsing from text
		if len(toolCalls) == 0 {
			if parsed := d.tryParseToolCallFromText(responseText); parsed != nil {
				toolCalls = parsed
			}
		}

		// No tool calls means task is complete (LLM gave final response)
		if len(toolCalls) == 0 {
			d.messages = append(d.messages, provider.ChatMessage{
				Role:    "assistant",
				Content: responseText,
			})
			if d.Trajectory != nil {
				d.Trajectory.LogMessage("assistant", responseText)
			}
			return &TaskResult{
				Success:   true,
				Summary:   responseText,
				ToolCalls: totalToolCalls,
				Turns:     turn,
				Usage:     totalUsage,
				Duration:  time.Since(start),
			}, nil
		}

		// Add assistant message with tool calls
		d.messages = append(d.messages, provider.ChatMessage{
			Role:      "assistant",
			Content:   responseText,
			ToolCalls: toolCalls,
		})

		// Execute each tool
		for _, tc := range toolCalls {
			totalToolCalls++
			if d.OnToolStart != nil {
				d.OnToolStart(tc.Function.Name, tc.Function.Arguments)
			}
			if d.Trajectory != nil {
				d.Trajectory.LogToolCall(tc.Function.Name, tc.Function.Arguments)
			}

			result := d.tools.Execute(ctx, tc.Function.Name, tc.Function.Arguments)

			if d.OnToolEnd != nil {
				d.OnToolEnd(tc.Function.Name, result)
			}
			if d.Trajectory != nil {
				d.Trajectory.LogToolResult(tc.Function.Name, result)
			}

			d.messages = append(d.messages, provider.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return &TaskResult{
		Summary:   "Reached maximum turns without completion",
		ToolCalls: totalToolCalls,
		Turns:     maxTurns,
		Usage:     totalUsage,
		Duration:  time.Since(start),
	}, nil
}

func (d *DroneRuntime) withSystem(sysPrompt string) []provider.ChatMessage {
	out := make([]provider.ChatMessage, 0, len(d.messages)+1)
	out = append(out, provider.ChatMessage{Role: "system", Content: sysPrompt})
	out = append(out, d.messages...)
	return out
}

func (d *DroneRuntime) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are Drone, an autonomous AI programming tool created by StarClaw.\n")
	sb.WriteString("You execute tasks independently, producing code changes, commits, and reports.\n")
	sb.WriteString("You do NOT have memory across tasks — each task starts fresh.\n\n")

	// Role-specific instructions
	sb.WriteString(fmt.Sprintf("## Role: %s\n", d.role.Name))
	sb.WriteString(d.role.SystemHint + "\n\n")

	// Permission level
	sb.WriteString(fmt.Sprintf("## Permission: %s\n", d.config.Permission))
	switch d.config.Permission {
	case "readonly":
		sb.WriteString("You may only READ files and run non-destructive commands. Do NOT write or edit files.\n")
	case "workspace_write":
		sb.WriteString("You may read and write files within the workspace. Do NOT run destructive system commands.\n")
	case "full_access":
		sb.WriteString("You have full access. Be careful with destructive operations.\n")
	}

	// Available tools
	sb.WriteString("\n## Available Tools\n")
	for _, name := range d.tools.Names() {
		sb.WriteString(fmt.Sprintf("- %s\n", name))
	}

	// Guidelines
	sb.WriteString("\n## Guidelines\n")
	sb.WriteString("- Be concise and direct\n")
	sb.WriteString("- Read files before editing\n")
	sb.WriteString("- Prefer minimal, targeted edits\n")
	sb.WriteString("- Run tests after making changes\n")
	sb.WriteString("- When the task is complete, provide a clear summary of what was done\n")
	sb.WriteString("- If you encounter an error you cannot resolve, explain what happened\n")

	sb.WriteString(fmt.Sprintf("\n## Workspace\nCurrent working directory: %s\n", d.config.Workspace))

	// Git context
	gc := CollectGitContext(d.config.Workspace)
	if gc != nil {
		sb.WriteString(gc.Format())
	}

	// DRONE.md project knowledge
	droneMD := config.LoadDroneMD(d.config.Workspace)
	if droneMD != "" {
		sb.WriteString("\n## Project Knowledge (DRONE.md)\n")
		sb.WriteString(droneMD + "\n")
	}

	// Skills
	skills := config.LoadDroneSkills(d.config.Workspace)
	if len(skills) > 0 {
		sb.WriteString("\n## Available Skills\n")
		for _, s := range skills {
			sb.WriteString(s + "\n---\n")
		}
	}

	return sb.String()
}

// tryParseToolCallFromText detects tool-call-like JSON in the model's text output.
func (d *DroneRuntime) tryParseToolCallFromText(text string) []provider.ToolCall {
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		return nil
	}

	var flat map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &flat); err == nil {
		name := ""
		for _, key := range []string{"name", "tool", "function", "tool_name"} {
			if v, ok := flat[key].(string); ok && v != "" {
				name = v
				delete(flat, key)
				break
			}
		}
		if name != "" {
			toolName := d.matchToolName(name)
			if toolName != "" {
				args := make(map[string]interface{})
				if params, ok := flat["parameters"].(map[string]interface{}); ok && len(params) > 0 {
					args = params
				} else if arguments, ok := flat["arguments"].(map[string]interface{}); ok && len(arguments) > 0 {
					args = arguments
				}
				for k, v := range flat {
					if k == "parameters" || k == "arguments" {
						continue
					}
					if _, exists := args[k]; !exists {
						args[k] = v
					}
				}
				argsJSON, _ := json.Marshal(args)
				return []provider.ToolCall{{
					ID:   fmt.Sprintf("fallback_%d", time.Now().UnixNano()),
					Type: "function",
					Function: provider.FunctionCall{
						Name:      toolName,
						Arguments: string(argsJSON),
					},
				}}
			}
		}
	}

	return nil
}

func (d *DroneRuntime) matchToolName(name string) string {
	lower := strings.ToLower(name)
	for _, def := range d.tools.Definitions() {
		if strings.ToLower(def.Function.Name) == lower {
			return def.Function.Name
		}
	}
	return ""
}

func extractJSON(text string) string {
	// Try markdown code block first
	if idx := strings.Index(text, "```"); idx >= 0 {
		rest := text[idx+3:]
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			candidate := strings.TrimSpace(rest[:end])
			if (strings.HasPrefix(candidate, "{") || strings.HasPrefix(candidate, "[")) && json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	// Try to find raw JSON object
	for _, opener := range []string{"{", "["} {
		idx := strings.Index(text, opener)
		if idx < 0 {
			continue
		}
		closer := "}"
		if opener == "[" {
			closer = "]"
		}
		depth := 0
		for i := idx; i < len(text); i++ {
			switch text[i] {
			case opener[0]:
				depth++
			case closer[0]:
				depth--
				if depth == 0 {
					candidate := text[idx : i+1]
					if json.Valid([]byte(candidate)) {
						return candidate
					}
				}
			}
		}
	}

	return ""
}

package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yinhe/devclaw/internal/provider"
)

// TrajectoryEntry is a single step in the execution trajectory
type TrajectoryEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "user", "assistant", "tool_call", "tool_result", "compress", "system"
	Role      string    `json:"role,omitempty"`
	Content   string    `json:"content,omitempty"`
	ToolName  string    `json:"tool_name,omitempty"`
	ToolArgs  string    `json:"tool_args,omitempty"`
	TokensIn  int       `json:"tokens_in,omitempty"`
	TokensOut int       `json:"tokens_out,omitempty"`
}

// Trajectory records the full execution trace of a Drone task
type Trajectory struct {
	TaskID    string            `json:"task_id"`
	Task      string            `json:"task"`
	Role      string            `json:"role"`
	Model     string            `json:"model"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time,omitempty"`
	Success   bool              `json:"success"`
	Turns     int               `json:"turns"`
	ToolCalls int               `json:"tool_calls"`
	Usage     provider.TokenUsage `json:"usage"`
	Entries   []TrajectoryEntry `json:"entries"`
}

// TrajectoryLogger collects trajectory data during execution
type TrajectoryLogger struct {
	trajectory Trajectory
	enabled    bool
	outputDir  string
}

// NewTrajectoryLogger creates a new logger
func NewTrajectoryLogger(taskID, task, roleName, model, outputDir string) *TrajectoryLogger {
	return &TrajectoryLogger{
		trajectory: Trajectory{
			TaskID:    taskID,
			Task:      task,
			Role:      roleName,
			Model:     model,
			StartTime: time.Now(),
			Entries:   []TrajectoryEntry{},
		},
		enabled:   outputDir != "",
		outputDir: outputDir,
	}
}

// LogMessage records a conversation message
func (l *TrajectoryLogger) LogMessage(role, content string) {
	if !l.enabled {
		return
	}
	entryType := role
	if role == "tool" {
		entryType = "tool_result"
	}
	l.trajectory.Entries = append(l.trajectory.Entries, TrajectoryEntry{
		Timestamp: time.Now(),
		Type:      entryType,
		Role:      role,
		Content:   truncContent(content, 5000),
	})
}

// LogToolCall records a tool invocation
func (l *TrajectoryLogger) LogToolCall(name, args string) {
	if !l.enabled {
		return
	}
	l.trajectory.Entries = append(l.trajectory.Entries, TrajectoryEntry{
		Timestamp: time.Now(),
		Type:      "tool_call",
		ToolName:  name,
		ToolArgs:  truncContent(args, 2000),
	})
}

// LogToolResult records a tool result
func (l *TrajectoryLogger) LogToolResult(name, result string) {
	if !l.enabled {
		return
	}
	l.trajectory.Entries = append(l.trajectory.Entries, TrajectoryEntry{
		Timestamp: time.Now(),
		Type:      "tool_result",
		ToolName:  name,
		Content:   truncContent(result, 2000),
	})
}

// LogCompress records a compression event
func (l *TrajectoryLogger) LogCompress(stats string) {
	if !l.enabled {
		return
	}
	l.trajectory.Entries = append(l.trajectory.Entries, TrajectoryEntry{
		Timestamp: time.Now(),
		Type:      "compress",
		Content:   stats,
	})
}

// LogUsage records token usage for a turn
func (l *TrajectoryLogger) LogUsage(usage *provider.TokenUsage) {
	if !l.enabled || usage == nil {
		return
	}
	if len(l.trajectory.Entries) > 0 {
		last := &l.trajectory.Entries[len(l.trajectory.Entries)-1]
		last.TokensIn = usage.PromptTokens
		last.TokensOut = usage.CompletionTokens
	}
}

// Finalize saves the trajectory to disk
func (l *TrajectoryLogger) Finalize(result *TaskResult) error {
	if !l.enabled {
		return nil
	}
	l.trajectory.EndTime = time.Now()
	l.trajectory.Success = result.Success
	l.trajectory.Turns = result.Turns
	l.trajectory.ToolCalls = result.ToolCalls
	l.trajectory.Usage = result.Usage

	// Ensure output directory exists
	if err := os.MkdirAll(l.outputDir, 0o755); err != nil {
		return fmt.Errorf("create trajectory dir: %w", err)
	}

	filename := fmt.Sprintf("trajectory_%s_%s.json", l.trajectory.TaskID, l.trajectory.StartTime.Format("20060102_150405"))
	path := filepath.Join(l.outputDir, filename)

	data, err := json.MarshalIndent(l.trajectory, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trajectory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write trajectory: %w", err)
	}

	return nil
}

// GetTrajectory returns the recorded trajectory
func (l *TrajectoryLogger) GetTrajectory() *Trajectory {
	return &l.trajectory
}

func truncContent(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

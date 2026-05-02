// Package main implements the Drone CLI — DevClaw's autonomous coding kernel.
//
// This file is the OSS (open-source) variant: kernel-only, no Forge/Pheromone
// integration. The full enterprise build lives in StarClaw's private monorepo
// and adds Forge issue tracking + Pheromone event reporting on top of this
// same runtime via plugin hooks.
//
// Build:   go build -o drone ./cmd/drone
// Run:     drone run --task "your task"
// Roles:   drone roles
// Version: drone version
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/yinhe/devclaw/internal/config"
	"github.com/yinhe/devclaw/internal/mcp"
	"github.com/yinhe/devclaw/internal/provider"
	"github.com/yinhe/devclaw/internal/role"
	"github.com/yinhe/devclaw/internal/runtime"
	"github.com/yinhe/devclaw/internal/tool"
	"github.com/yinhe/devclaw/internal/worktree"
)

// Build info — goreleaser injects real values via -ldflags at release time.
// Local `go build` leaves these as "dev" / "none" / "unknown".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-v", "--version":
			fmt.Printf("drone %s (DevClaw kernel · OSS build)\n", version)
			if commit != "none" || date != "unknown" {
				fmt.Printf("  commit: %s\n  built:  %s\n", commit, date)
			}
			return
		case "roles":
			fmt.Println("Available roles:")
			for _, r := range role.ValidRoles() {
				p := role.Get(r)
				fmt.Printf("  %-8s %s (permission: %s)\n", r, p.Description, p.Permission)
			}
			return
		case "run":
			os.Args = os.Args[1:] // shift so flag.Parse works on "run" args
		case "help", "-h", "--help":
			printUsage()
			return
		}
	}

	// Flags (kernel-only — no Forge/Pheromone)
	taskFlag := flag.String("task", "", "Task description (required, or use --task-file or positional args)")
	taskFileFlag := flag.String("task-file", "", "Read task from file")
	roleFlag := flag.String("role", "dev", "Role: dev, test, ops, sense, scout")
	modelFlag := flag.String("model", "", "Model name (default: from DRONE_MODEL env)")
	maxTurnsFlag := flag.Int("max-turns", 50, "Maximum agent loop turns")
	workspaceFlag := flag.String("workspace", "", "Workspace directory (default: cwd)")
	worktreeFlag := flag.Bool("worktree", false, "Use git worktree isolation")
	permFlag := flag.String("permission", "", "Permission: readonly, workspace_write, full_access (default: from role)")
	trajectoryFlag := flag.String("trajectory", "", "Directory for trajectory logs (enables future Abathur distillation)")
	quietFlag := flag.Bool("quiet", false, "Suppress streaming output")
	flag.Parse()

	// Load task
	task := *taskFlag
	if task == "" && *taskFileFlag != "" {
		data, err := os.ReadFile(*taskFileFlag)
		if err != nil {
			log.Fatalf("Error reading task file: %v", err)
		}
		task = strings.TrimSpace(string(data))
	}
	if task == "" && flag.NArg() > 0 {
		task = strings.Join(flag.Args(), " ")
	}
	if task == "" {
		printUsage()
		os.Exit(1)
	}

	// Build config
	cfg := config.DefaultConfig()
	cfg.Task = task
	cfg.Role = *roleFlag
	cfg.MaxTurns = *maxTurnsFlag
	cfg.Worktree = *worktreeFlag

	if *modelFlag != "" {
		cfg.Model = *modelFlag
	}
	if *workspaceFlag != "" {
		cfg.Workspace = *workspaceFlag
	}
	if *permFlag != "" {
		cfg.Permission = *permFlag
	} else {
		cfg.Permission = role.Get(cfg.Role).Permission
	}
	if *trajectoryFlag != "" {
		cfg.TrajectoryDir = *trajectoryFlag
	}

	if cfg.APIKey == "" {
		log.Fatal("Error: DRONE_API_KEY or OPENAI_API_KEY environment variable required")
	}

	// Setup context with SIGINT handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\n[drone] Interrupted, cleaning up...\n")
		cancel()
	}()

	// Setup workspace (optionally with worktree isolation)
	workspace := cfg.Workspace
	var wt *worktree.Worktree
	if cfg.Worktree {
		taskID := fmt.Sprintf("task-%d", time.Now().Unix())
		var err error
		wt, err = worktree.Create(workspace, taskID)
		if err != nil {
			log.Fatalf("Failed to create worktree: %v", err)
		}
		defer func() {
			if wt != nil {
				wt.Cleanup(false)
			}
		}()
		workspace = wt.WorktreeDir
		cfg.Workspace = workspace
		fmt.Fprintf(os.Stderr, "[drone] Worktree: %s (branch: %s)\n", wt.WorktreeDir, wt.BranchName)
	}

	// Setup provider
	base := provider.NewOpenAIProvider(provider.OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
	})
	p := provider.NewRetryProvider(base, 2)

	// Setup tools
	registry := tool.NewRegistry()
	registerTools(registry, workspace, cfg.Permission)

	// Connect MCP servers (.drone/mcp.json)
	var mcpClients []*mcp.Client
	mcpServers := mcp.LoadServers(workspace)
	for _, srv := range mcpServers {
		client, err := mcp.NewClient(srv)
		if err != nil {
			log.Printf("[drone] MCP %s: %v (skipping)", srv.Name, err)
			continue
		}
		mcpClients = append(mcpClients, client)
		for _, td := range client.ToDefinitions() {
			registry.Register(&mcpProxyTool{client: client, info: td})
		}
		fmt.Fprintf(os.Stderr, "[drone] MCP %s: %d tools\n", srv.Name, len(client.Tools()))
	}
	defer func() {
		for _, c := range mcpClients {
			c.Close()
		}
	}()

	// Print header
	fmt.Fprintf(os.Stderr, "[drone] Role: %s | Model: %s | Permission: %s | Tools: %d\n",
		cfg.Role, cfg.Model, cfg.Permission, registry.Size())
	fmt.Fprintf(os.Stderr, "[drone] Task: %s\n", truncate(task, 100))
	fmt.Fprintf(os.Stderr, "[drone] Working directory: %s\n", workspace)
	fmt.Fprintf(os.Stderr, "[drone] ---\n")

	// Build and run the runtime
	dr := runtime.New(p, registry, cfg)

	// Register Agent/Parallel tools (sub-Drone spawning)
	if cfg.Permission != "readonly" {
		registry.Register(tool.NewAgentTool(dr.RunSubTask))
		registry.Register(tool.NewParallelTool(dr.RunSubTask))
	}

	// Trajectory logging
	taskID := fmt.Sprintf("drone-%d", time.Now().Unix())
	if cfg.TrajectoryDir != "" {
		dr.Trajectory = runtime.NewTrajectoryLogger(taskID, task, cfg.Role, cfg.Model, cfg.TrajectoryDir)
		fmt.Fprintf(os.Stderr, "[drone] Trajectory: %s\n", cfg.TrajectoryDir)
	}

	if !*quietFlag {
		dr.OnText = func(text string) { fmt.Print(text) }
		dr.OnToolStart = func(name, args string) {
			fmt.Fprintf(os.Stderr, "\n[tool] %s: %s\n", name, extractToolSummary(name, args))
		}
		dr.OnToolEnd = func(name, result string) {
			if len(result) > 200 {
				result = result[:200] + "..."
			}
			fmt.Fprintf(os.Stderr, "[tool] %s -> %s\n", name, strings.ReplaceAll(result, "\n", " "))
		}
		dr.OnCompress = func(stats string) { fmt.Fprintf(os.Stderr, "[drone] %s\n", stats) }
	}

	result, err := dr.Run(ctx)
	if err != nil {
		log.Fatalf("[drone] Fatal: %v", err)
	}

	if dr.Trajectory != nil {
		if err := dr.Trajectory.Finalize(result); err != nil {
			log.Printf("[drone] Warning: trajectory save failed: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "[drone] Trajectory saved to %s\n", cfg.TrajectoryDir)
		}
	}

	if wt != nil {
		if err := wt.CommitAll(fmt.Sprintf("drone(%s): %s", cfg.Role, truncate(task, 72))); err != nil {
			log.Printf("[drone] Warning: commit failed: %v", err)
		} else if diff := wt.DiffSummary(); diff != "" {
			fmt.Fprintf(os.Stderr, "\n[drone] Changes committed to branch %s:\n%s\n", wt.BranchName, diff)
		}
	}

	fmt.Fprintf(os.Stderr, "\n[drone] ---\n")
	fmt.Fprintf(os.Stderr, "[drone] Completed: %d turns, %d tool calls, %s\n",
		result.Turns, result.ToolCalls, result.Duration.Round(time.Second))
	if result.Usage.TotalTokens > 0 {
		fmt.Fprintf(os.Stderr, "[drone] Tokens: %d prompt + %d completion = %d total\n",
			result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens)
	}
	if !result.Success {
		os.Exit(1)
	}
}

func registerTools(registry *tool.Registry, workspace, permission string) {
	registry.Register(tool.NewReadTool(workspace))
	registry.Register(tool.NewListDirTool())
	registry.Register(tool.NewGlobTool(workspace))
	registry.Register(tool.NewGrepTool(workspace))

	if permission != "readonly" {
		registry.Register(tool.NewWriteTool())
		registry.Register(tool.NewEditTool())
		registry.Register(tool.NewMultiEditTool())
		registry.Register(tool.NewPatchTool())
		registry.Register(tool.NewUndoTool())
	}

	registry.Register(tool.NewBashTool(workspace))
}

type mcpProxyTool struct {
	client *mcp.Client
	info   provider.ToolDefinition
}

func (t *mcpProxyTool) Name() string            { return t.info.Function.Name }
func (t *mcpProxyTool) Description() string     { return t.info.Function.Description }
func (t *mcpProxyTool) Parameters() interface{} { return t.info.Function.Parameters }

func (t *mcpProxyTool) Execute(ctx context.Context, args string) (string, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return "", fmt.Errorf("parse mcp args: %w", err)
	}
	parts := strings.SplitN(t.info.Function.Name, "__", 2)
	toolName := t.info.Function.Name
	if len(parts) == 2 {
		toolName = parts[1]
	}
	return t.client.CallTool(ctx, toolName, m)
}

func extractToolSummary(name, args string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return truncate(args, 80)
	}
	switch name {
	case "Bash":
		if cmd, ok := m["command"].(string); ok {
			return truncate(cmd, 80)
		}
	case "Read", "Write", "Edit":
		if fp, ok := m["file_path"].(string); ok {
			return fp
		}
	case "Glob", "Grep":
		if p, ok := m["pattern"].(string); ok {
			return p
		}
	case "ListDir":
		if p, ok := m["path"].(string); ok {
			return p
		}
	}
	return truncate(args, 80)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Drone — DevClaw's autonomous coding kernel

USAGE:
  drone run --task "your task description"
  drone run --task-file task.md
  drone run "your task description"
  drone roles
  drone version

FLAGS:
  --task          Task description
  --task-file     Read task from file
  --role          dev | test | ops | sense | scout (default: dev)
  --model         Model name (default: $DRONE_MODEL)
  --max-turns     Max agent loop turns (default: 50)
  --workspace     Workspace dir (default: cwd)
  --worktree      Use git worktree isolation
  --permission    readonly | workspace_write | full_access
  --trajectory    Directory for trajectory logs
  --quiet         Suppress streaming output

ENVIRONMENT:
  DRONE_API_KEY   LLM API key (or OPENAI_API_KEY / STARAI_API_KEY)
  DRONE_BASE_URL  LLM API base URL (default: auto-detect)
  DRONE_MODEL     Model name (default: auto-detect)

LEARN MORE:
  https://github.com/yinhe/devclaw              Source code
  https://github.com/yinhe/devclaw/discussions  Ask questions
  https://github.com/yinhe/devclaw/releases     Download binaries
`)
}

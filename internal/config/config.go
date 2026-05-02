// Package config holds Drone runtime configuration.
//
// OSS variant: this version excludes the Forge / Overlord / Pheromone
// integration fields used by the StarClaw enterprise build. Add those back
// in your own fork or via build tags if needed.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds Drone runtime configuration.
type Config struct {
	// LLM settings
	APIKey  string
	BaseURL string
	Model   string

	// Task settings
	Task     string // task description
	Role     string // dev, test, ops, sense, scout
	MaxTurns int

	// Workspace
	Workspace string // project root directory
	Worktree  bool   // use git worktree isolation

	// Permission
	Permission string // readonly, workspace_write, full_access

	// Trajectory logging
	TrajectoryDir string // Directory for trajectory logs (empty = disabled)
}

// DefaultConfig returns sensible defaults.
//
// Resolution priority:
//   - APIKey:  DRONE_API_KEY > STARAI_API_KEY > OPENAI_API_KEY > "ollama"
//   - BaseURL: DRONE_BASE_URL > OPENAI_BASE_URL > auto-detect
//                (StarAI cloud if STARAI_API_KEY set, else local Ollama)
//   - Model:   DRONE_MODEL > auto-detect
//                (qwen3-coder-plus on StarAI, qwen3-coder on Ollama)
func DefaultConfig() Config {
	cwd, _ := os.Getwd()

	apiKey := firstEnv("DRONE_API_KEY", "STARAI_API_KEY", "OPENAI_API_KEY")
	isStarAI := apiKey != "" && os.Getenv("STARAI_API_KEY") != "" && os.Getenv("DRONE_API_KEY") == ""
	if apiKey == "" {
		apiKey = "ollama"
	}

	baseURL := firstEnv("DRONE_BASE_URL", "OPENAI_BASE_URL")
	if baseURL == "" {
		if isStarAI || apiKey == os.Getenv("STARAI_API_KEY") {
			baseURL = "https://api.star-ai.net/v1"
		} else {
			baseURL = "http://localhost:11434/v1"
		}
	}

	model := os.Getenv("DRONE_MODEL")
	if model == "" {
		if strings.Contains(baseURL, "star-ai.net") {
			model = "qwen3-coder-plus"
		} else {
			model = "qwen3-coder"
		}
	}

	return Config{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      model,
		Role:       "dev",
		MaxTurns:   50,
		Workspace:  cwd,
		Worktree:   false,
		Permission: "workspace_write",
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// LoadDroneMD discovers and reads the DRONE.md project knowledge file.
// Search order: workspace/DRONE.md, workspace/.drone/DRONE.md
func LoadDroneMD(workspace string) string {
	paths := []string{
		filepath.Join(workspace, "DRONE.md"),
		filepath.Join(workspace, ".drone", "DRONE.md"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// LoadDroneSkills discovers skill files from .drone/skills/ directory.
func LoadDroneSkills(workspace string) []string {
	skillDir := filepath.Join(workspace, ".drone", "skills")
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return nil
	}
	var skills []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(skillDir, e.Name()))
		if err == nil {
			skills = append(skills, string(data))
		}
	}
	return skills
}

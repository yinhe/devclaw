package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- Read Tool ---

type ReadTool struct{ workspace string }

func NewReadTool(ws string) *ReadTool { return &ReadTool{workspace: ws} }

func (t *ReadTool) Name() string        { return "Read" }
func (t *ReadTool) Description() string { return "Read file contents. Returns numbered lines." }
func (t *ReadTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]string{"type": "string", "description": "Absolute file path"},
			"offset":    map[string]string{"type": "integer", "description": "Start line (1-indexed)"},
			"limit":     map[string]string{"type": "integer", "description": "Number of lines"},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		FilePath string `json:"file_path"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	lines := strings.Split(string(data), "\n")
	offset := p.Offset
	if offset < 1 {
		offset = 1
	}
	offset-- // 0-indexed
	limit := p.Limit
	if limit <= 0 {
		limit = len(lines)
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	for i := offset; i < end; i++ {
		fmt.Fprintf(&sb, "%d\t%s\n", i+1, lines[i])
	}
	return sb.String(), nil
}

// --- Write Tool ---

type WriteTool struct{}

func NewWriteTool() *WriteTool { return &WriteTool{} }

func (t *WriteTool) Name() string { return "Write" }
func (t *WriteTool) Description() string {
	return "Write or create a file. Creates parent directories."
}
func (t *WriteTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]string{"type": "string", "description": "Absolute file path"},
			"content":   map[string]string{"type": "string", "description": "File content"},
		},
		"required": []string{"file_path", "content"},
	}
}

func (t *WriteTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p.FilePath), 0o755); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	if err := os.WriteFile(p.FilePath, []byte(p.Content), 0o644); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return fmt.Sprintf("Written %d bytes to %s", len(p.Content), p.FilePath), nil
}

// --- Edit Tool ---

type EditTool struct{}

func NewEditTool() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string { return "Edit" }
func (t *EditTool) Description() string {
	return "Replace exact text in a file. old_string must be unique."
}
func (t *EditTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path":  map[string]string{"type": "string", "description": "File path"},
			"old_string": map[string]string{"type": "string", "description": "Text to find (exact)"},
			"new_string": map[string]string{"type": "string", "description": "Replacement text"},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

func (t *EditTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		FilePath  string `json:"file_path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	content := string(data)
	count := strings.Count(content, p.OldString)
	if count == 0 {
		return fmt.Sprintf("Error: old_string not found in %s", p.FilePath), nil
	}
	if count > 1 {
		return fmt.Sprintf("Error: old_string found %d times, must be unique", count), nil
	}
	saveUndoBackup(p.FilePath, content)
	content = strings.Replace(content, p.OldString, p.NewString, 1)
	if err := os.WriteFile(p.FilePath, []byte(content), 0o644); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return fmt.Sprintf("Edited %s (replaced 1 occurrence)", p.FilePath), nil
}

// --- ListDir Tool ---

type ListDirTool struct{}

func NewListDirTool() *ListDirTool { return &ListDirTool{} }

func (t *ListDirTool) Name() string        { return "ListDir" }
func (t *ListDirTool) Description() string { return "List files and directories in a path." }
func (t *ListDirTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]string{"type": "string", "description": "Directory path"},
		},
		"required": []string{"path"},
	}
}

func (t *ListDirTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	var sb strings.Builder
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		prefix := "      "
		if e.IsDir() {
			prefix = "[dir] "
		}
		fmt.Fprintf(&sb, "%s%s\n", prefix, e.Name())
	}
	if sb.Len() == 0 {
		return "(empty directory)", nil
	}
	return sb.String(), nil
}

// --- Glob Tool ---

type GlobTool struct{ workspace string }

func NewGlobTool(ws string) *GlobTool { return &GlobTool{workspace: ws} }

func (t *GlobTool) Name() string        { return "Glob" }
func (t *GlobTool) Description() string { return "Search for files by name pattern." }
func (t *GlobTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]string{"type": "string", "description": "Glob pattern (e.g. *.go)"},
			"path":    map[string]string{"type": "string", "description": "Search directory"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	searchDir := p.Path
	if searchDir == "" {
		searchDir = t.workspace
	}
	var results []string
	filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || len(results) >= 50 {
			return filepath.SkipDir
		}
		name := info.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		matched, _ := filepath.Match(p.Pattern, name)
		if matched {
			rel, _ := filepath.Rel(searchDir, path)
			results = append(results, rel)
		}
		return nil
	})
	if len(results) == 0 {
		return "No matches", nil
	}
	return strings.Join(results, "\n"), nil
}

// --- Grep Tool ---

type GrepTool struct{ workspace string }

func NewGrepTool(ws string) *GrepTool { return &GrepTool{workspace: ws} }

func (t *GrepTool) Name() string        { return "Grep" }
func (t *GrepTool) Description() string { return "Search file contents by string." }
func (t *GrepTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]string{"type": "string", "description": "Search string"},
			"path":    map[string]string{"type": "string", "description": "Search directory"},
			"include": map[string]string{"type": "string", "description": "File filter (e.g. *.go)"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Include string `json:"include"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	searchDir := p.Path
	if searchDir == "" {
		searchDir = t.workspace
	}
	var results []string
	filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || len(results) >= 100 {
			if info != nil && info.IsDir() {
				name := info.Name()
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if p.Include != "" {
			matched, _ := filepath.Match(p.Include, info.Name())
			if !matched {
				return nil
			}
		}
		if info.Size() > 1024*1024 {
			return nil // skip files > 1MB
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		rel, _ := filepath.Rel(searchDir, path)
		matchCount := 0
		for i, line := range lines {
			if strings.Contains(line, p.Pattern) {
				results = append(results, fmt.Sprintf("%s:%d:%s", rel, i+1, truncStr(line, 200)))
				matchCount++
				if matchCount >= 5 {
					break
				}
			}
		}
		return nil
	})
	if len(results) == 0 {
		return "No matches", nil
	}
	return strings.Join(results, "\n"), nil
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

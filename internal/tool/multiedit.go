package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MultiEditTool applies multiple replacements to a single file
type MultiEditTool struct{}

func NewMultiEditTool() *MultiEditTool { return &MultiEditTool{} }

func (t *MultiEditTool) Name() string        { return "MultiEdit" }
func (t *MultiEditTool) Description() string { return "Multiple find-and-replace edits in one file. Each old_string must be unique." }
func (t *MultiEditTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]string{"type": "string", "description": "File path"},
			"edits": map[string]interface{}{
				"type": "array",
				"description": "Array of {old_string, new_string} pairs",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"old_string": map[string]string{"type": "string"},
						"new_string": map[string]string{"type": "string"},
					},
				},
			},
		},
		"required": []string{"file_path", "edits"},
	}
}

func (t *MultiEditTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		FilePath string `json:"file_path"`
		Edits    []struct {
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		} `json:"edits"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	content := string(data)

	// Validate all edits first
	for i, e := range p.Edits {
		count := strings.Count(content, e.OldString)
		if count == 0 {
			return fmt.Sprintf("Error: edit[%d] old_string not found", i), nil
		}
		if count > 1 {
			return fmt.Sprintf("Error: edit[%d] old_string found %d times, must be unique", i, count), nil
		}
	}

	// Save backup for undo
	saveUndoBackup(p.FilePath, content)

	// Apply edits sequentially
	for _, e := range p.Edits {
		content = strings.Replace(content, e.OldString, e.NewString, 1)
	}

	if err := os.WriteFile(p.FilePath, []byte(content), 0o644); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return fmt.Sprintf("Applied %d edits to %s", len(p.Edits), p.FilePath), nil
}

// PatchTool applies a unified diff patch to a file
type PatchTool struct{}

func NewPatchTool() *PatchTool { return &PatchTool{} }

func (t *PatchTool) Name() string        { return "Patch" }
func (t *PatchTool) Description() string { return "Apply text changes to a file using before/after blocks." }
func (t *PatchTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]string{"type": "string", "description": "File path"},
			"before":    map[string]string{"type": "string", "description": "Exact text block to find"},
			"after":     map[string]string{"type": "string", "description": "Replacement text block"},
		},
		"required": []string{"file_path", "before", "after"},
	}
}

func (t *PatchTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		FilePath string `json:"file_path"`
		Before   string `json:"before"`
		After    string `json:"after"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	content := string(data)
	count := strings.Count(content, p.Before)
	if count == 0 {
		return fmt.Sprintf("Error: before block not found in %s", p.FilePath), nil
	}
	if count > 1 {
		return fmt.Sprintf("Error: before block found %d times, must be unique", count), nil
	}

	saveUndoBackup(p.FilePath, content)
	content = strings.Replace(content, p.Before, p.After, 1)
	if err := os.WriteFile(p.FilePath, []byte(content), 0o644); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return fmt.Sprintf("Patched %s (%d→%d chars)", p.FilePath, len(p.Before), len(p.After)), nil
}

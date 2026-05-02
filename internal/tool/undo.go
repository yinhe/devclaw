package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Undo stack — stores one backup per file path
var (
	undoMu    sync.Mutex
	undoStack = make(map[string]string) // file_path → previous content
)

func saveUndoBackup(filePath, content string) {
	undoMu.Lock()
	undoStack[filePath] = content
	undoMu.Unlock()
}

// UndoTool reverts the last edit to a file
type UndoTool struct{}

func NewUndoTool() *UndoTool { return &UndoTool{} }

func (t *UndoTool) Name() string        { return "Undo" }
func (t *UndoTool) Description() string { return "Undo the last edit/write to a file." }
func (t *UndoTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]string{"type": "string", "description": "File to undo"},
		},
		"required": []string{"file_path"},
	}
}

func (t *UndoTool) Execute(_ context.Context, args string) (string, error) {
	var p struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	undoMu.Lock()
	prev, ok := undoStack[p.FilePath]
	if ok {
		delete(undoStack, p.FilePath)
	}
	undoMu.Unlock()

	if !ok {
		return fmt.Sprintf("Error: no undo history for %s", p.FilePath), nil
	}

	if err := os.WriteFile(p.FilePath, []byte(prev), 0o644); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return fmt.Sprintf("Reverted %s to previous state (%d bytes)", p.FilePath, len(prev)), nil
}

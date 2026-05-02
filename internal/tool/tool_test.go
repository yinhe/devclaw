package tool

import (
	"context"
	"testing"
)

func TestRegistryBasics(t *testing.T) {
	r := NewRegistry()
	if r.Size() != 0 {
		t.Fatalf("expected 0, got %d", r.Size())
	}

	r.Register(NewReadTool("."))
	r.Register(NewBashTool("."))
	if r.Size() != 2 {
		t.Fatalf("expected 2, got %d", r.Size())
	}

	tool, ok := r.Get("Read")
	if !ok || tool.Name() != "Read" {
		t.Fatal("Read tool not found")
	}

	_, ok = r.Get("NonExistent")
	if ok {
		t.Fatal("should not find NonExistent")
	}
}

func TestRegistryExecuteNotFound(t *testing.T) {
	r := NewRegistry()
	result := r.Execute(context.Background(), "Foo", "{}")
	if result != `Error: tool "Foo" not found` {
		t.Fatalf("unexpected: %s", result)
	}
}

func TestRegistryDefinitions(t *testing.T) {
	r := NewRegistry()
	r.Register(NewReadTool("."))
	r.Register(NewGlobTool("."))
	defs := r.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	for _, d := range defs {
		if d.Type != "function" {
			t.Fatalf("expected function type, got %s", d.Type)
		}
	}
}

func TestRegistryNames(t *testing.T) {
	r := NewRegistry()
	r.Register(NewReadTool("."))
	r.Register(NewEditTool())
	names := r.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2, got %d", len(names))
	}
}

func TestBashApproval(t *testing.T) {
	tests := []struct {
		cmd      string
		danger   bool
		risk     string
	}{
		{"ls -la", false, "safe"},
		{"rm -rf /tmp/foo", true, "dangerous"},
		{"git push origin main", true, "dangerous"},
		{"echo hello", false, "write"},
		{"cat foo.txt", false, "safe"},
		{"sudo apt install vim", true, "dangerous"},
		{"mv a.txt b.txt", false, "write"},
	}
	for _, tt := range tests {
		if got := IsDangerousCommand(tt.cmd); got != tt.danger {
			t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.cmd, got, tt.danger)
		}
		if got := ClassifyRisk(tt.cmd); got != tt.risk {
			t.Errorf("ClassifyRisk(%q) = %q, want %q", tt.cmd, got, tt.risk)
		}
	}
}

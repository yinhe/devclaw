package tool

import (
	"context"
	"fmt"

	"github.com/yinhe/devclaw/internal/provider"
)

// Tool is the interface all tools must implement
type Tool interface {
	Name() string
	Description() string
	Parameters() interface{}
	Execute(ctx context.Context, args string) (string, error)
}

// ToDefinition converts a Tool to a provider.ToolDefinition
func ToDefinition(t Tool) provider.ToolDefinition {
	return provider.ToolDefinition{
		Type: "function",
		Function: provider.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		},
	}
}

// Registry manages available tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns all tool definitions for LLM
func (r *Registry) Definitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, ToDefinition(t))
	}
	return defs
}

// Execute runs a tool by name with JSON args
func (r *Registry) Execute(ctx context.Context, name, args string) string {
	t, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("Error: tool %q not found", name)
	}
	result, err := t.Execute(ctx, args)
	if err != nil {
		return fmt.Sprintf("Error executing %s: %v", name, err)
	}
	return result
}

// Size returns the number of registered tools
func (r *Registry) Size() int {
	return len(r.tools)
}

// Names returns all registered tool names
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

package provider

import "context"

// Provider is the interface for LLM providers
type Provider interface {
	Name() string
	Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error)
}
